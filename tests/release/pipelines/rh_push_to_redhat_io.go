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
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	rhioServiceAccountName = "release-service-account"
	rhioCatalogPathInRepo  = "pipelines/rh-push-to-registry-redhat-io/rh-push-to-registry-redhat-io.yaml"
	rhioGitSourceURL       = "https://github.com/redhat-appstudio-qe/devfile-sample-python-basic-release"
	rhioGitSrcSHA          = "33ff89edf85fb01a37d3d652d317080223069fc7"
)

var rhioComponentName = "rhio-comp-" + util.GenerateRandomString(4)

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for rh-push-to-redhat-io pipeline", Pending, Label("release-pipelines", "rh-push-to-redhat-io"), func() {
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
	var rhioReleasePlanName = "rhio-rp-" + util.GenerateRandomString(4)
	var rhioReleasePlanAdmissionName = "rhio-rpa-" + util.GenerateRandomString(4)
	var rhioEnterpriseContractPolicyName = "rhio-policy-" + util.GenerateRandomString(4)
	var sampleImage = "quay.io/hacbs-release-tests/e2e-rhio-comp@sha256:bf2fb2c7d63c924ff9170c27f0f15558f6a59bdfb5ad9613eb61d3e4bc1cff0a"

	var snapshotPush *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var releasePR *tektonv1.PipelineRun

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

			pyxisSecret, err := managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, "pyxis")
			if pyxisSecret == nil || errors.IsNotFound(err){
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

				_, err = managedFw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
				Expect(err).ToNot(HaveOccurred())
			}

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(rhioApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(rhioReleasePlanName, devNamespace, rhioApplicationName, managedNamespace, "true", nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createRHIOReleasePlanAdmission(rhioReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, rhioApplicationName, rhioEnterpriseContractPolicyName, rhioCatalogPathInRepo)

			createRHIOEnterpriseContractPolicy(rhioEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(*devFw, rhioComponentName, rhioApplicationName, devNamespace, sampleImage, rhioGitSourceURL, rhioGitSrcSHA, "", "", "", "")
			GinkgoWriter.Println("snapshotPush.Name: %s", snapshotPush.GetName())
                        Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(rhioApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(rhioEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(rhioReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {
			It("verifies the rhio release pipelinerun is running and succeeds", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return err
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())

				Expect(managedFw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
			})

			It("verifies release CR completed and set succeeded.", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return err
					}
					GinkgoWriter.Println("Release CR: ", releaseCR.Name)
					if !releaseCR.IsReleased() {
						return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())
			})

			It("verifies if the MR URL is valid", func() {
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
			Exclude: []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			Include: []string{"@slsa3"},
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
					"name":       rhioComponentName,
					"repository": "quay.io/redhat-pending/rhtap----konflux-release-e2e",
					"tags": []string{"latest", "latest-{{ timestamp }}", "testtag",
						"testtag-{{ timestamp }}", "testtag2", "testtag2-{{ timestamp }}"},
				},
			},
		},
		"pyxis": map[string]interface{}{
			"server": "stage",
			"secret": "pyxis",
		},
		"fileUpdates": []map[string]interface{}{
			{
				"repo":          releasecommon.GitLabRunFileUpdatesTestRepo,
				"upstream_repo": releasecommon.GitLabRunFileUpdatesTestRepo,
				"ref":           "master",
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
			"cosignSecretName": "test-cosign-secret",
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
