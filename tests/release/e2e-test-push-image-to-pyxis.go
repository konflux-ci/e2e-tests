package release

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/devfile/library/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
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
	var devNamespace, managedNamespace, compName, additionalCompName string
	var imageIDs []string
	var pyxisKeyDecoded, pyxisCertDecoded []byte
	var releasePrName, additionalReleasePrName string
	scGitRevision := fmt.Sprintf("test-pyxis-%s", util.GenerateRandomString(4))

	var component1, component2 *appservice.Component

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

		// Linking the build secret to the pipeline service account in dev namespace.
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

		// using cdq since git ref is not known
		compName = componentName
		var componentDetected appservice.ComponentDetectionDescription
		cdq, err := fw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(compName, devNamespace, gitSourceComponentUrl, "", "", "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

		for _, compDetected := range cdq.Status.ComponentDetected {
			compName = compDetected.ComponentStub.ComponentName
			componentDetected = compDetected
		}

		// using cdq since git ref is not known
		additionalCompName = additionalComponentName
		var additionalComponentDetected appservice.ComponentDetectionDescription
		cdq, err = fw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(additionalCompName, devNamespace, additionalGitSourceComponentUrl, "", "", "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

		for _, compDetected := range cdq.Status.ComponentDetected {
			additionalCompName = compDetected.ComponentStub.ComponentName
			additionalComponentDetected = compDetected
		}

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		components := []release.Component{{Name: compName, Repository: releasedImagePushRepo}, {Name: additionalCompName, Repository: additionalReleasedImagePushRepo}}
		sc := fw.AsKubeAdmin.ReleaseController.GenerateReleaseStrategyConfig(components)
		scYaml, err := yaml.Marshal(sc)
		Expect(err).ShouldNot(HaveOccurred())

		scPath := "release-push-to-pyxis.yaml"
		Expect(fw.AsKubeAdmin.CommonController.Github.CreateRef("strategy-configs", "main", scGitRevision)).To(Succeed())
		_, err = fw.AsKubeAdmin.CommonController.Github.CreateFile("strategy-configs", scPath, string(scYaml), scGitRevision)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-push-to-external-registry-strategy", managedNamespace, "push-to-external-registry", "quay.io/hacbs-release/pipeline-push-to-external-registry:0.8", releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, []releaseApi.Params{
			{Name: "extraConfigGitUrl", Value: fmt.Sprintf("https://github.com/%s/strategy-configs.git", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))},
			{Name: "extraConfigPath", Value: scPath},
			{Name: "extraConfigGitRevision", Value: scGitRevision},
			{Name: "pyxisServerType", Value: "stage"},
			{Name: "pyxisSecret", Value: "pyxis"},
			{Name: "tag", Value: "latest"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, "", "", "mvp-push-to-external-registry-strategy")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", releaseStrategyServiceAccountDefault, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateHasApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		component1, err = fw.AsKubeAdmin.HasController.CreateComponentFromStub(componentDetected, devNamespace, "", "", applicationNameDefault)
		Expect(err).NotTo(HaveOccurred())

		component2, err = fw.AsKubeAdmin.HasController.CreateComponentFromStub(additionalComponentDetected, devNamespace, "", "", applicationNameDefault)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		err = fw.AsKubeAdmin.CommonController.Github.DeleteRef("strategy-configs", scGitRevision)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
		}
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that build PipelineRuns are created for each Component in dev namespace and both succeed", func() {
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component1, "", 2)).To(Succeed())
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component2, "", 2)).To(Succeed())
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
