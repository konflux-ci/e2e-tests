package pipelines

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
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
	"knative.dev/pkg/apis"
)

const (
	devportalServiceAccountName = "release-service-account"
	devportalSourceGitURL       = "https://github.com/hacbs-release-tests/devfile-sample-go-basic-e2e"
	devportalCatalogPathInRepo  = "pipelines/push-binaries-to-dev-portal/push-binaries-to-dev-portal.yaml"
)

var devportalComponent *appservice.Component
var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for push-binaries-to-dev-portal pipeline", Label("release-pipelines", "push-binaries-to-dev-portal"), func() {
	defer GinkgoRecover()

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var devportalApplicationName = "devportal-app-" + util.GenerateRandomString(4)
	var devportalComponentName = "devportal-comp-" + util.GenerateRandomString(4)
	var devportalReleasePlanName = "devportal-rp-" + util.GenerateRandomString(4)
	var devportalReleasePlanAdmissionName = "devportal-rpa-" + util.GenerateRandomString(4)
	var devportalEnterpriseContractPolicyName = "devportal-policy-" + util.GenerateRandomString(4)

	var snapshot *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var buildPR, releasePR *tektonv1.PipelineRun

	AfterEach(framework.ReportFailure(&devFw))

	Describe("Push-to-dev-portal happy path", Label("pushToDevPortal"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			// Create a ticker that ticks every 3 minutes
			ticker := time.NewTicker(3 * time.Minute)
			// Schedule the stop of the ticker after 15 minutes
			time.AfterFunc(15*time.Minute, func() {
				ticker.Stop()
				fmt.Println("Stopped executing every 3 minutes.")
			})
			// Run a goroutine to handle the ticker ticks
			go func() {
				for range ticker.C {
					devFw = releasecommon.NewFramework(devWorkspace)
					managedFw = releasecommon.NewFramework(managedWorkspace)
				}
			}()

			managedNamespace = managedFw.UserNamespace

			// Linking the build secret to the pipeline service account in dev namespace.
			err = devFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, releasecommon.HacbsReleaseTestsTokenSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			keyExodusProd := os.Getenv(constants.EXODUS_PROD_KEY_ENV)
			Expect(keyExodusProd).ToNot(BeEmpty())

			certExodusProd := os.Getenv(constants.EXODUS_PROD_CERT_ENV)
			Expect(certExodusProd).ToNot(BeEmpty())

			// Creating k8s secret to access Exodus product based on base64 decoded of key and cert
			exodusKeyDecoded, err := base64.StdEncoding.DecodeString(string(keyExodusProd))
			Expect(err).ToNot(HaveOccurred())

			exodusCertDecoded, err := base64.StdEncoding.DecodeString(string(certExodusProd))
			Expect(err).ToNot(HaveOccurred())

			exodusSecret, err := managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, releasecommon.ExodusProdSecretName)

			if exodusSecret == nil || errors.IsNotFound(err) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      releasecommon.ExodusProdSecretName,
						Namespace: managedNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"cert": exodusCertDecoded,
						"key":  exodusKeyDecoded,
					},
				}

				_, err = managedFw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
				Expect(err).ToNot(HaveOccurred())
			}

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.ExodusProdSecretName, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			githubToken := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
			Expect(githubToken).ToNot(BeEmpty())
			qeSecret, err := managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, releasecommon.RedhatAppstudioQESecret)
			if qeSecret == nil || errors.IsNotFound(err) {
				githubSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      releasecommon.RedhatAppstudioQESecret,
						Namespace: managedNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"token": []byte(githubToken),
					},
				}
				_, err = managedFw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, githubSecret)
				Expect(err).ToNot(HaveOccurred())
			}

			cgwUser := utils.GetEnv(constants.CGW_USERNAME_ENV, "konflux-release-service-cgw")
			cgwToken := utils.GetEnv(constants.CGW_TOKEN_ENV, "")
			cgwSecret, err := managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, releasecommon.CgwSecretName)
			if cgwSecret == nil || errors.IsNotFound(err) {
				githubSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      releasecommon.CgwSecretName,
						Namespace: managedNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"username": []byte(cgwUser),
						"token": []byte(cgwToken),
					},
				}
				_, err = managedFw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, githubSecret)
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(err).ToNot(HaveOccurred())

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioQESecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.CgwSecretName, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(devportalApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(devportalReleasePlanName, devNamespace, devportalApplicationName, managedNamespace, "true", nil, nil)
			Expect(err).NotTo(HaveOccurred())

			devportalComponent = releasecommon.CreateComponent(*devFw, devNamespace, devportalApplicationName, devportalComponentName, devportalSourceGitURL, "", ".", "Dockerfile", constants.DefaultDockerBuildPipelineBundle)

			createDevPortalReleasePlanAdmission(devportalReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, devportalApplicationName, devportalEnterpriseContractPolicyName, devportalCatalogPathInRepo, "false", "", "", "", "")
			Expect(err).NotTo(HaveOccurred())

			createDevPortalEnterpriseContractPolicy(devportalEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(devportalApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(devportalEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(devportalReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {
			It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
				Eventually(func() error {
					buildPR, err = devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(devportalComponent.Name, devportalApplicationName, devNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the component %s/%s\n", devNamespace, devportalComponent.Name)
						return err
					}
					GinkgoWriter.Printf("PipelineRun %s reason: %s\n", buildPR.Name, buildPR.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason())
					if !buildPR.IsDone() {
						return fmt.Errorf("build pipelinerun %s in namespace %s did not finish yet", buildPR.Name, buildPR.Namespace)
					}
					if buildPR.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
						snapshot, err = devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPR.Name, "", devNamespace)
						if err != nil {
							return err
						}
						return nil
					} else {
						return fmt.Errorf(tekton.GetFailedPipelineRunLogs(devFw.AsKubeDeveloper.HasController.KubeRest(), devFw.AsKubeDeveloper.HasController.KubeInterface(), buildPR))
					}
				}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to be finished for the component %s/%s", devNamespace, devportalComponent.Name))
			})
			It("verifies release pipelinerun is running and succeeds", func() {
				releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(managedFw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))

				_, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).NotTo(HaveOccurred())
			})

			It("verifies release CR completed and set succeeded.", func() {
				Eventually(func() error {
					releaseCR, err := devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
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

			It("verifies if publish-to-cgw task completed successfully", func() {
				releasePR, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).NotTo(HaveOccurred())

				trReleaseLogs, err := managedFw.AsKubeAdmin.TektonController.GetTaskRunLogs(releasePR.GetName(), "publish-to-cgw", releasePR.GetNamespace())
				Expect(err).NotTo(HaveOccurred())

				var log string
				for _, tasklog := range trReleaseLogs {
					log = tasklog
				}

				pattern := `New file created with file_id:- \d+`

				re, err := regexp.Compile(pattern)
				Expect(err).NotTo(HaveOccurred())
				Expect(re.MatchString(log)).To(BeTrue(), "New file created in publish-to-cgw task")

				substr := "All CGW operations are successfully completed...!"
				Expect(strings.Contains(log, substr)).To(BeTrue(), "All CGW operations are successfully completed")
			})
		})
	})
})

func createDevPortalEnterpriseContractPolicy(devportalECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
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

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(devportalECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createDevPortalReleasePlanAdmission(devportalRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, devportalAppName, devportalECPName, pathInRepoValue, hotfix, issueId, preGA, productName, productVersion string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"exodus": map[string]interface{}{
			"env": "cdn-live",
		},
		"contentGateway": map[string]interface{}{
			"productName": "Konflux test product",
			"productCode": "KTestProduct",
			"productVersionName": "KTestProduct 1",
			"components":[]map[string]interface{}{
				{
					"name": devportalComponent.GetName(),
					"description": "Red Hat OpenShift Local Sandbox Test",
					"label": "Checksum File Sandbox Test",
				},
				// It is some hack to get binaries for testing publish-to-cgw task
				{
					"name": "devportal-comp",
				},
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(devportalRPAName, managedNamespace, "", devNamespace, devportalECPName, devportalServiceAccountName, []string{devportalAppName}, true, &tektonutils.PipelineRef{
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
