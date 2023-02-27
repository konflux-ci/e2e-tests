package release

import (
	"os"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"knative.dev/pkg/apis"

	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-1571]test-release-e2e-push-image-to-pyxis", Label("release", "pushPyxis", "HACBS"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework("release-e2e-pyxis")
	Expect(err).NotTo(HaveOccurred())

	kubeController := tekton.KubeController{
		Commonctrl: *framework.AsKubeAdmin.CommonController,
		Tektonctrl: *framework.AsKubeAdmin.TektonController,
	}

	var devNamespace = framework.UserNamespace
	var managedNamespace = utils.GetGeneratedNamespace("release-pushpx-managed")

	BeforeAll(func() {

		GinkgoWriter.Println("Dev Namespace created: ", devNamespace)
		_, err = framework.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: ", err)
		GinkgoWriter.Println("Managed Namespace created: ", managedNamespace)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		destinationAuthJson := utils.GetEnv("QUAY_OAUTH_TOKEN_RELEASE_DESTINATION", "")
		Expect(destinationAuthJson).ToNot(BeEmpty())

		keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
		Expect(keyPyxisStage).ToNot(BeEmpty())

		certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
		Expect(certPyxisStage).ToNot(BeEmpty())

		_, err = framework.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		_, err = framework.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(redhatAppstudioUserSecret, managedNamespace, destinationAuthJson)
		Expect(err).ToNot(HaveOccurred())

		err = framework.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, "pipeline")
		Expect(err).ToNot(HaveOccurred())

		publicKey, err := kubeController.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())

		_, err = framework.AsKubeAdmin.CommonController.CreateRegistryAuthSecretKeyValueDecoded("pyxis", managedNamespace, string(keyPyxisStage), string(certPyxisStage))
		Expect(err).ToNot(HaveOccurred())

		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

		defaultEcPolicy := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   string(publicKey),
			Sources: []ecp.Source{
				{
					Name: "ec-policies",
					Policy: []string{
						"git::https://github.com/hacbs-contract/ec-policies.git//policy/lib",
						"git::https://github.com/hacbs-contract/ec-policies.git//policy/release",
					},
					Data: []string{
						"git::https://github.com/hacbs-contract/ec-policies.git//data",
					},
				},
			},
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"minimal"},
				Exclude:     []string{"cve"},
			},
		}

		_, err = framework.AsKubeAdmin.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
		Expect(err).NotTo(HaveOccurred())

		err = framework.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, redhatAppstudioUserSecret, releaseStrategyServiceAccountDefault)
		Expect(err).ToNot(HaveOccurred())

		_, err = framework.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		_, err = framework.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-push-to-external-registry-strategy", managedNamespace, "push-to-external-registry", "quay.io/hacbs-release/pipeline-push-to-external-registry:main", releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, paramsReleaseStrategyPyxis)
		Expect(err).NotTo(HaveOccurred())

		_, err = framework.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, "", "", "mvp-push-to-external-registry-strategy")
		Expect(err).NotTo(HaveOccurred())

		_, err = framework.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicy)
		Expect(err).NotTo(HaveOccurred())

		_, err = framework.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = framework.AsKubeAdmin.HasController.CreateHasApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = framework.AsKubeAdmin.HasController.CreateComponent(applicationNameDefault, componentName, devNamespace, gitSourceComponentUrl, "", containerImageUrl, "", "", true)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterAll(func() {

		if !CurrentSpecReport().Failed() {
			Expect(framework.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(framework.SandboxController.DeleteUserSignup(framework.UserName)).NotTo(BeFalse())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that a PipelineRun is created in dev namespace.", func() {
			Eventually(func() bool {
				prList, err := framework.AsKubeAdmin.TektonController.ListAllPipelineRuns(devNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					return false
				}

				return strings.Contains(prList.Items[0].Name, componentName)
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that the PipelineRun in dev namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := framework.AsKubeAdmin.TektonController.ListAllPipelineRuns(devNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that a PipelineRun is created in managed namespace.", func() {
			Eventually(func() bool {
				prList, err := framework.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					return false
				}

				return strings.Contains(prList.Items[0].Name, "release")
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies a PipelineRun started in managed namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := framework.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("validate the result of task create-pyxis-image contains id and succeeded.", func() {
			Eventually(func() bool {
				prList, err := framework.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				pr, err := kubeController.Tektonctrl.GetPipelineRun(prList.Items[0].Name, managedNamespace)
				Expect(err).NotTo(HaveOccurred())

				tr, err := kubeController.GetTaskRunStatus(pr, "create-pyxis-image")
				Expect(err).NotTo(HaveOccurred())

				re := regexp.MustCompile("^[0-9a-z]{10,50}")
				returnedImageID := re.FindString(tr.Status.TaskRunResults[0].Value.StringVal)

				return strings.Contains(string(tr.Status.Status.Conditions[0].Status), "True") &&
					strings.Contains(string(tr.Status.TaskRunResults[0].Name), "containerImageIDs") && len(returnedImageID) > 10
			}).Should(BeTrue())
		})

		It("tests a Release should have been created in the dev namespace and succeeded.", func() {
			Eventually(func() bool {
				releaseCreated, err := framework.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				if releaseCreated == nil || err != nil {
					return false
				}

				return releaseCreated.HasStarted() && releaseCreated.IsDone() && releaseCreated.Status.Conditions[0].Status == "True"
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})
	})
})
