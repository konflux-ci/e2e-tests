package pipelines

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	releaseapi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	releasecommon "github.com/redhat-appstudio/e2e-tests/tests/release"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rhioServiceAccountName   = "release-service-account"
	rhioCatalogPathInRepo    = "pipelines/rh-push-to-registry-redhat-io/rh-push-to-registry-redhat-io.yaml"
)

var testComponent *appservice.Component

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for rh-push-to-redhat-io pipeline", Label("release-pipelines", "rh-push-to-redhat-io"), func() {
	defer GinkgoRecover()
	var pyxisKeyDecoded, pyxisCertDecoded []byte

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var rhioApplicationName = "rhio-app-" + util.GenerateRandomString(4)
	var rhioComponentName = "rhio-comp-" + util.GenerateRandomString(4)
	var rhioReleasePlanName = "rhio-rp-" + util.GenerateRandomString(4)
	var rhioReleasePlanAdmissionName = "rhio-rpa-" + util.GenerateRandomString(4)
	var rhioEnterpriseContractPolicyName = "rhio-policy-" + util.GenerateRandomString(4)

	var snapshot *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var releasePR, buildPR *tektonv1.PipelineRun

	AfterEach(framework.ReportFailure(&devFw))

	Describe("Rh-push-to-redhat-io happy path", Label("PushToRedhatIO"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace

			keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
			Expect(keyPyxisStage).ToNot(BeEmpty())

			certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
			Expect(certPyxisStage).ToNot(BeEmpty())

			// Creating k8s secret to access Pyxis stage based on base64 decoded of key and cert
			pyxisKeyDecoded, err = base64.StdEncoding.DecodeString(string(keyPyxisStage))
			Expect(err).ToNot(HaveOccurred())

			pyxisCertDecoded, err = base64.StdEncoding.DecodeString(string(certPyxisStage))
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pyxis",
					Namespace: managedNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"cert": pyxisCertDecoded,
					"key":  pyxisKeyDecoded,
				},
			}

			// Delete the secret if it exists in case it is not correct
			_ = managedFw.AsKubeAdmin.CommonController.DeleteSecret(managedNamespace, "pyxis")
			_, err = managedFw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
			Expect(err).ToNot(HaveOccurred())

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(rhioApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			 _, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(rhioReleasePlanName, devNamespace, rhioApplicationName, managedNamespace, "true", nil)
			Expect(err).NotTo(HaveOccurred())

			testComponent = releasecommon.CreateComponentByCDQ(*devFw, devNamespace, managedNamespace, rhioApplicationName, rhioComponentName, releasecommon.AdditionalGitSourceComponentUrl)
			createRHIOReleasePlanAdmission(rhioReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, rhioApplicationName, rhioEnterpriseContractPolicyName, rhioCatalogPathInRepo)

			createRHIOEnterpriseContractPolicy(rhioEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(rhioApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(rhioEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(rhioReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {
			It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				// Create a ticker that ticks every 3 minutes
				ticker := time.NewTicker(3 * time.Minute)
				// Schedule the stop of the ticker after 15 minutes
				time.AfterFunc(10*time.Minute, func() {
					ticker.Stop()
					fmt.Println("Stopped executing every 3 minutes.")
				})
				// Run a goroutine to handle the ticker ticks
				go func() {
					for range ticker.C {
						devFw = releasecommon.NewFramework(devWorkspace)
					}
				}()
				Eventually(func() error {
					buildPR, err = devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(testComponent.Name, rhioApplicationName, devNamespace, "")
					if err != nil {
						return err
					}
					if !buildPR.IsDone() {
						return fmt.Errorf("build pipelinerun %s in namespace %s did not finish yet", buildPR.Name, buildPR.Namespace)
					}
					Expect(tekton.HasPipelineRunSucceeded(buildPR)).To(BeTrue(), fmt.Sprintf("build pipelinerun %s/%s did not succeed", buildPR.GetNamespace(), buildPR.GetName()))
					snapshot, err = devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPR.Name, "", devNamespace)
					if err != nil {
						return err
					}
					return nil
				}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for build pipelinerun to be created")
				Expect(devFw.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(testComponent, "", devFw.AsKubeDeveloper.TektonController, &has.RetryOptions{Retries: 3, Always: true}, nil)).To(Succeed())
			})
			It("verifies the rhio release pipelinerun is running and succeeds", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				managedFw = releasecommon.NewFramework(managedWorkspace)

				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
					if err != nil {
						return err
					}
					releasePR, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
					if err != nil {
						return err
					}
					GinkgoWriter.Println("Release PR: ", releasePR.Name)
					if  !releasePR.IsDone(){
						return fmt.Errorf("release pipelinerun %s in namespace %s did not finished yet", releasePR.Name, releasePR.Namespace)
					}
					return nil
				}, releasecommon.ReleasePipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for release pipelinerun to succeed")
				Expect(tekton.HasPipelineRunSucceeded(releasePR)).To(BeTrue(), fmt.Sprintf("release pipelinerun %s/%s did not succeed", releasePR.GetNamespace(), releasePR.GetName()))
			})

			It("verifies release CR completed and set succeeded.", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				Eventually(func() error {
					releaseCr, err := devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
					if err != nil {
						return err
					}
					GinkgoWriter.Println("Release CR: ", releaseCr.Name)
					if !releaseCr.IsReleased() {
						return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
					}
					return nil
				}, 10 * time.Minute, releasecommon.DefaultInterval).Should(Succeed())
			})

			It("verifies if the MR URL is valid", func() {
				managedFw = releasecommon.NewFramework(managedWorkspace)
				releasePR, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).NotTo(HaveOccurred())

				trReleaseLogs, err := managedFw.AsKubeAdmin.TektonController.GetTaskRunLogs(releasePR.GetName(), "run-file-updates", releasePR.GetNamespace())
				Expect(err).NotTo(HaveOccurred())

				var log, mrURL string
				for _, tasklog := range trReleaseLogs {
					log = tasklog
				}

				re := regexp.MustCompile(`(?:MR Created: )(.+)`)
				mrURL = re.FindStringSubmatch(log)[1]

				pattern := `https?://[^/\s]+/[^/\s]+/[^/\s]+/+\-\/merge_requests\/(\d+)`
				re, err = regexp.Compile(pattern)
				Expect(err).NotTo(HaveOccurred())
				Expect(re.MatchString(mrURL)).To(BeTrue(), fmt.Sprintf("MR URL %s is not valid", mrURL))
			})
		})
	})
})

func createRHIOEnterpriseContractPolicy(rhioECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
	defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
		Description: "Red Hat's enterprise requirements",
		PublicKey:   "k8s://openshift-pipelines/public-key",
		Sources: []ecp.Source{{
			Name:   "Default",
			Policy: []string{releasecommon.EcPolicyLibPath, releasecommon.EcPolicyReleasePath},
			Data:   []string{releasecommon.EcPolicyDataBundle, releasecommon.EcPolicyDataPath},
		}},
		Configuration: &ecp.EnterpriseContractPolicyConfiguration{
			Exclude: []string{"cve", "step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			Include: []string{"minimal"},
		},
	}

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(rhioECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createRHIOReleasePlanAdmission(rhioRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, rhioAppName, rhioECPName, pathInRepoValue string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"name"       : testComponent.GetName(),
					"repository" : "quay.io/redhat-pending/rhtap----konflux-release-e2e",
				},
			},
		},
		"pyxis": map[string]interface{}{
			"server": "stage",
			"secret": "pyxis",
		},
		"images": map[string]interface{}{
			"defaultTag":      "latest",
			"addGitShaTag":    false,
			"addTimestampTag": false,
			"addSourceShaTag": false,
			"floatingTags":    []string{"testtag", "testtag2"},
		},
		"fileUpdates":  []map[string]interface{}{
			{
				"repo": releasecommon.GitLabRunFileUpdatesTestRepo,
				"upstream_repo": releasecommon.GitLabRunFileUpdatesTestRepo,
				"ref": "master",
				"paths": []map[string]interface{}{
					{
						"path": "data/app-interface/app-interface-settings.yml",
						"replacements": []map[string]interface{}{
							{
								"key": ".description",
								// description: App Interface settings 
								"replacement": "|description:.*|description: {{ .components[0].containerImage }}|",
							},
						},
					},
				},
			},
		},
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(rhioRPAName, managedNamespace, "", devNamespace, rhioECPName, rhioServiceAccountName, []string{rhioAppName}, true, &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasecommon.RelSvcCatalogURL},
			{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
			{Name: "pathInRepo", Value: pathInRepoValue},
		},
	}, &runtime.RawExtension{
		Raw: data,
	})
	Expect(err).NotTo(HaveOccurred())
}
