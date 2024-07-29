package pipelines

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

const (
	multiarchServiceAccountName  = "release-service-account"
	multiarchCatalogPathInRepo   = "pipelines/rh-advisories/rh-advisories.yaml"
	multiarchGitSourceURL        = "https://github.com/redhat-appstudio-qe/multi-platform-test-prod"
	multiarchGitSourceRepoName   = "multi-platform-test-prod"
	multiarchGitSrcSHA           = "fd4b6c28329ab3df77e7ad7beebac1829836561d"
)

var multiarchComponentName = "e2e-multi-platform-test"

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for multi arch test for rh-advisories release pipeline", Label("release-pipelines", "multiarch_advisories"), func() {
	defer GinkgoRecover()
	var pyxisKeyDecoded, pyxisCertDecoded []byte

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var multiarchApplicationName = "e2e-multi-platform-test-prod"
	var multiarchReleasePlanName = "e2e-multiarch-rp"
	var multiarchReleasePlanAdmissionName = "e2e-multiarch-rpa"
	var multiarchEnterpriseContractPolicyName = "e2e-multiarch-policy"

	var snapshotPush *appservice.Snapshot
	var releaseCR *releaseapi.Release

	AfterEach(framework.ReportFailure(&devFw))

	Describe("Multi-arch happy path", Label("MultiArchAdvisories"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			// Create a ticker that ticks every 3 minutes
			ticker := time.NewTicker(3 * time.Minute)
			// Schedule the stop of the ticker after 15 minutes
			time.AfterFunc(30*time.Minute, func() {
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

			keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
			Expect(keyPyxisStage).ToNot(BeEmpty())

			certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
			Expect(certPyxisStage).ToNot(BeEmpty())

			// Creating k8s secret to access Pyxis stage based on base64 decoded of key and cert
			pyxisKeyDecoded, err = base64.StdEncoding.DecodeString(string(keyPyxisStage))
			Expect(err).ToNot(HaveOccurred())

			pyxisCertDecoded, err = base64.StdEncoding.DecodeString(string(certPyxisStage))
			Expect(err).ToNot(HaveOccurred())

			_, err = managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, releasecommon.RedhatAppstudioQESecret)
			if errors.IsNotFound(err) {
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

			_, err = devFw.AsKubeDeveloper.HasController.GetApplication(multiarchApplicationName, devNamespace)
			if errors.IsNotFound(err) {
				GinkgoWriter.Printf("The Application %s needs to be setup before the test\n", multiarchApplicationName)
			}
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.GetComponent(multiarchComponentName, devNamespace)
			if errors.IsNotFound(err) {
				GinkgoWriter.Printf("The component %s tighting to repo %s needs to be setup with PaC configuration before the test\n", multiarchComponentName, multiarchGitSourceURL)
			}
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.GetReleasePlan(multiarchReleasePlanName, devNamespace)
			if errors.IsNotFound(err) {
				createMultiArchReleasePlan(multiarchReleasePlanName, *devFw, devNamespace, multiarchApplicationName, managedNamespace, "true")
			}

			_, err = managedFw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(multiarchReleasePlanAdmissionName, managedNamespace)
			if errors.IsNotFound(err) {
				createMultiArchReleasePlanAdmission(multiarchReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, multiarchApplicationName, multiarchEnterpriseContractPolicyName, multiarchCatalogPathInRepo)
			}

			_, err = managedFw.AsKubeDeveloper.TektonController.GetEnterpriseContractPolicy(multiarchEnterpriseContractPolicyName, managedNamespace)
			if errors.IsNotFound(err) {
				createMultiArchEnterpriseContractPolicy(multiarchEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
			}

			sampleImage := "quay.io/redhat-user-workloads-stage/dev-release-team-tenant/e2e-multi-platform-test-prod/e2e-multi-platform-test@sha256:23ce99c70f86f879c67a82ef9aa088c7e9a52dc09630c5913587161bda6259e2"
			snapshotPush, err = devFw.AsKubeAdmin.IntegrationController.CreateSnapshotWithImageSource(multiarchComponentName, multiarchApplicationName, devNamespace, sampleImage, multiarchGitSourceURL, multiarchGitSrcSHA)
                        Expect(err).ShouldNot(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {

			It("verifies the multiarch release pipelinerun is running and succeeds", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return err
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())

				Eventually(func() error {
					pipelineRun, err := managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
					if err != nil {
						return fmt.Errorf("PipelineRun has not been created yet for release %s/%s", releaseCR.GetNamespace(), releaseCR.GetName())
					}
					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pipelineRun.Name, condition.Reason)
					}

					if !pipelineRun.IsDone() {
						return fmt.Errorf("PipelineRun has still not finished yet")
					}

					if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
						return nil
					} else {
						return fmt.Errorf(tekton.GetFailedPipelineRunLogs(managedFw.AsKubeAdmin.ReleaseController.KubeRest(), managedFw.AsKubeAdmin.ReleaseController.KubeInterface(), pipelineRun))
					}
				}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the release PipelineRun to be finished for the release %s/%s", releaseCR.GetName(), releaseCR.GetNamespace()))
			})

			It("verifies release CR completed and set succeeded", func() {
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
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the Release CR %s/%s completed", releaseCR.GetName(), releaseCR.GetNamespace()))
			})
		})
	})
})

func createMultiArchEnterpriseContractPolicy(multiarchECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
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
			//Exclude: []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies", "slsa_source_correlated.expected_source_code_reference:git+https://github.com/redhat-appstudio-qe/multi-platform-test-prod.git@sha1:fd4b6c28329ab3df77e7ad7beebac1829836561d"},
			Include: []string{"@slsa3"},
		},
	}

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(multiarchECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())
}

func createMultiArchReleasePlan(multiarchReleasePlanName string, devFw framework.Framework, devNamespace, multiarchAppName, managedNamespace string, autoRelease string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"releaseNotes": map[string]interface{}{
			"description": "releaseNotes description",
			"references":  []string{"https://server.com/ref1", "http://server2.com/ref2"},
			"solution":    "some solution",
			"synopsis":    "test synopsis",
			"topic":       "test topic",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(multiarchReleasePlanName, devNamespace, multiarchAppName,
		managedNamespace, autoRelease, &runtime.RawExtension{
			Raw: data,
		}, nil)
	Expect(err).NotTo(HaveOccurred())
}


func createMultiArchReleasePlanAdmission(multiarchRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, multiarchAppName, multiarchECPName, pathInRepoValue string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"name": multiarchComponentName,
					"repository": "quay.io/redhat-pending/rhtap----konflux-release-e2e",
					"tags": []string{"latest"},
					"source": map[string]interface{}{
						"git": map[string]interface{}{
							"url": multiarchGitSourceURL,
						},
					},
				},
			},
		},
		"pyxis": map[string]interface{}{
			"server": "stage",
			"secret": "pyxis",
		},
		"releaseNotes": map[string]interface{}{
			"cpe":             "cpe:/a:example.com",
			"product_id":      "555",
			"product_name":    "test product",
			"product_stream":  "rhtas-tp1",
			"product_version": "v1.0",
			"type":            "RHSA",
		},
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(multiarchRPAName, managedNamespace, "", devNamespace, multiarchECPName, multiarchServiceAccountName, []string{multiarchAppName}, true, &tektonutils.PipelineRef{
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
