package release

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"knative.dev/pkg/apis"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-1571]test-release-e2e-push-image-to-pyxis", Label("release", "pushPyxis", "HACBS"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	var fw *framework.Framework
	var err error
	var kubeController tekton.KubeController

	var devNamespace, managedNamespace string
	var imageIDs []string
	var pyxisKeyDecoded, pyxisCertDecoded []byte
	var releasePrName, additionalReleasePrName string

	BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace("e2e-pyxis"))
		Expect(err).NotTo(HaveOccurred())

		kubeController = tekton.KubeController{
			Commonctrl: *fw.AsKubeAdmin.CommonController,
			Tektonctrl: *fw.AsKubeAdmin.TektonController,
		}

		devNamespace = fw.UserNamespace
		managedNamespace = utils.GetGeneratedNamespace("pyxis-managed")

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: ", err)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		destinationAuthJson := utils.GetEnv("QUAY_OAUTH_TOKEN_RELEASE_DESTINATION", "")
		Expect(destinationAuthJson).ToNot(BeEmpty())

		keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
		Expect(keyPyxisStage).ToNot(BeEmpty())

		certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
		Expect(certPyxisStage).ToNot(BeEmpty())

		// Create secret for the build registry repo "redhat-appstudio-qe".
		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		// Create secret for the release registry repo "hacbs-release-tests".
		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(redhatAppstudioUserSecret, managedNamespace, destinationAuthJson)
		Expect(err).ToNot(HaveOccurred())

		// Linking the build secret to the pipline service account in dev namespace.
		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, serviceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		publicKey, err := kubeController.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())

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

		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
		Expect(err).ToNot(HaveOccurred())

		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

		defaultEcPolicy, err := kubeController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred())

		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   string(publicKey),
			Sources:     defaultEcPolicy.Spec.Sources,
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"minimal", "slsa2"},
				Exclude:     []string{"cve"},
			},
		}

		_, err = fw.AsKubeAdmin.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
		Expect(err).NotTo(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, redhatAppstudioUserSecret, releaseStrategyServiceAccountDefault, true)
		Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-push-to-external-registry-strategy", managedNamespace, "push-to-external-registry", "quay.io/hacbs-release/pipeline-push-to-external-registry:0.8", releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, paramsReleaseStrategyPyxis)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, "", "", "mvp-push-to-external-registry-strategy")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateHasApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateComponent(applicationNameDefault, componentName, devNamespace, gitSourceComponentUrl, "", containerImageUrl, "", "", true)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateComponent(applicationNameDefault, additionalComponentName, devNamespace, additionalGitSourceComponentUrl, "", "", addtionalOutputContainerImage, "", false)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterAll(func() {

		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that PipelineRuns are created in dev namespace.", func() {
			Eventually(func() bool {
				_, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationNameDefault, devNamespace, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}

				_, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(additionalComponentName, applicationNameDefault, devNamespace, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}

				return true
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that the build PipelineRuns in dev namespace succeeded.", func() {
			Eventually(func() bool {
				buildPr, err := fw.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationNameDefault, devNamespace, "")
				if err != nil {
					GinkgoWriter.Printf("\nError getting PipelineRun %s:\n %s", buildPr.Name, err)
					return false
				}
				additionalBuildPr, err := fw.AsKubeAdmin.HasController.GetComponentPipelineRun(additionalComponentName, applicationNameDefault, devNamespace, "")
				if err != nil {
					GinkgoWriter.Printf("\nError getting PipelineRun %s:\n %s", additionalBuildPr.Name, err)
					return false
				}

				return buildPr.HasStarted() && buildPr.IsDone() && buildPr.Status.GetCondition(apis.ConditionSucceeded).IsTrue() &&
					additionalBuildPr.HasStarted() && additionalBuildPr.IsDone() && additionalBuildPr.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that a release PipelineRun for each Component is created in managed namespace.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					GinkgoWriter.Printf("Error getting Release PipelineRun:\n %s", err)
					return false
				}
				foudFirstReleasePr := false
				for _, pr := range prList.Items {
					if strings.Contains(pr.Name, "release-pipelinerun") {
						if !foudFirstReleasePr {
							releasePrName = pr.Name
							foudFirstReleasePr = true
						} else {
							additionalReleasePrName = pr.Name
						}
					}
				}

				return strings.Contains(releasePrName, "release-pipelinerun") &&
					strings.Contains(additionalReleasePrName, "release-pipelinerun")
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies a release PipelineRun for each component started in managed namespace and succeeded.", func() {
			Eventually(func() bool {

				releasePr, err := fw.AsKubeAdmin.TektonController.GetPipelineRun(releasePrName, managedNamespace)
				if err != nil {
					GinkgoWriter.Printf("\nError getting Release PipelineRun %s:\n %s", releasePr, err)
					return false
				}
				additionalReleasePr, err := fw.AsKubeAdmin.TektonController.GetPipelineRun(additionalReleasePrName, managedNamespace)
				if err != nil {
					GinkgoWriter.Printf("\nError getting PipelineRun %s:\n %s", additionalReleasePr, err)
					return false
				}

				return releasePr.HasStarted() && releasePr.IsDone() && releasePr.Status.GetCondition(apis.ConditionSucceeded).IsTrue() &&
					additionalReleasePr.HasStarted() && additionalReleasePr.IsDone() && additionalReleasePr.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("validate the result of task create-pyxis-image contains image ids.", func() {
			Eventually(func() bool {

				releasePr, err := fw.AsKubeAdmin.TektonController.GetPipelineRun(releasePrName, managedNamespace)
				if err != nil {
					GinkgoWriter.Printf("\nError getting Release PipelineRun %s:\n %s", releasePr, err)
					return false
				}
				additionalReleasePr, err := fw.AsKubeAdmin.TektonController.GetPipelineRun(additionalReleasePrName, managedNamespace)
				if err != nil {
					GinkgoWriter.Printf("\nError getting PipelineRun %s:\n %s", additionalReleasePr, err)
					return false
				}
				re := regexp.MustCompile("[a-fA-F0-9]{24}")

				trReleasePr, err := kubeController.GetTaskRunStatus(fw.AsKubeAdmin.CommonController.KubeRest(), releasePr, "create-pyxis-image")
				if err != nil {
					Expect(err).NotTo(HaveOccurred())
				}

				trAdditionalReleasePr, err := kubeController.GetTaskRunStatus(fw.AsKubeAdmin.CommonController.KubeRest(), additionalReleasePr, "create-pyxis-image")
				if err != nil {
					Expect(err).NotTo(HaveOccurred())
				}

				trReleaseImageIDs := re.FindAllString(trReleasePr.Status.TaskRunResults[0].Value.StringVal, -1)
				trAdditionalReleaseIDs := re.FindAllString(trAdditionalReleasePr.Status.TaskRunResults[0].Value.StringVal, -1)

				if len(trReleaseImageIDs) < 1 && len(trAdditionalReleaseIDs) < 1 {
					GinkgoWriter.Printf("\n Invalid ImageID in results of task create-pyxis-image..")
					return false
				}

				if len(trReleaseImageIDs) > len(trAdditionalReleaseIDs) {
					imageIDs = trReleaseImageIDs
				} else {
					imageIDs = trAdditionalReleaseIDs
				}

				return len(imageIDs) == 2
			}, avgControllerQueryTimeout, defaultInterval).Should(BeTrue())
		})

		It("tests a Release should have been created in the dev namespace and succeeded.", func() {
			Eventually(func() bool {
				releaseCreated, err := fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				if releaseCreated == nil || err != nil {
					return false
				}

				return releaseCreated.IsReleased()
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("validates that imageIds from task create-pyxis-image exist in Pyxis.", func() {

			for _, imageID := range imageIDs {
				Eventually(func() bool {

					body, err := fw.AsKubeAdmin.ReleaseController.GetSbomPyxisByImageID(pyxisStageURL, imageID,
						[]byte(pyxisCertDecoded), []byte(pyxisKeyDecoded))
					if err != nil {
						GinkgoWriter.Printf("Error getting response body:", err)
						Expect(err).NotTo(HaveOccurred())
					}

					sbomImage := &release.Image{}
					err = json.Unmarshal(body, sbomImage)
					if err != nil {
						GinkgoWriter.Printf("Error json unmarshal body content.", err)
						Expect(err).NotTo(HaveOccurred())
					}

					if sbomImage.ContentManifestComponents == nil {
						GinkgoWriter.Printf("Content Mainfest Components is empty.")
						return false
					}

					return len(sbomImage.ContentManifestComponents) > 1
				}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
			}

		})
	})
})
