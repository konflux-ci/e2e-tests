package release

import (
	"fmt"
	"strings"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-738]test-release-service-default-pipeline", Label("release", "defaultBundle", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var compName string
	var devNamespace = utils.GetGeneratedNamespace("release-dev")
	var managedNamespace = utils.GetGeneratedNamespace("release-managed")
	var component *appservice.Component

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework("release-e2e-bundle")
		Expect(err).NotTo(HaveOccurred())
		kubeController := tekton.KubeController{
			Commonctrl: *fw.AsKubeAdmin.CommonController,
			Tektonctrl: *fw.AsKubeAdmin.TektonController,
		}

		_, err := fw.AsKubeAdmin.CommonController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating devNamespace: %v", err)
		GinkgoWriter.Println("Dev Namespace created: %s ", devNamespace)

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: %v", err)
		GinkgoWriter.Println("Managed Namespace created: %s", managedNamespace)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		destinationAuthJson := utils.GetEnv("QUAY_OAUTH_TOKEN_RELEASE_DESTINATION", "")
		Expect(destinationAuthJson).ToNot(BeEmpty())

		_, err = fw.AsKubeAdmin.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(redhatAppstudioUserSecret, managedNamespace, destinationAuthJson)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, serviceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, redhatAppstudioUserSecret, serviceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		publicKey, err := kubeController.GetTektonChainsPublicKey()
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
				Collections: []string{"minimal"},
				Exclude:     []string{"cve"},
			},
		}

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

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		components := []release.Component{{Name: compName, Repository: releasedImagePushRepo}}
		sc := fw.AsKubeAdmin.ReleaseController.GenerateReleaseStrategyConfig(components)
		scYaml, err := yaml.Marshal(sc)
		Expect(err).ShouldNot(HaveOccurred())

		scPath := "release-default-bundle.yaml"
		Expect(fw.AsKubeAdmin.CommonController.Github.CreateRef("strategy-configs", "main", compName)).To(Succeed())
		_, err = fw.AsKubeAdmin.CommonController.Github.CreateFile("strategy-configs", scPath, string(scYaml), compName)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy(releaseStrategyDefaultName, managedNamespace, releasePipelineNameDefault, constants.ReleasePipelineImageRef, releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, []releaseApi.Params{
			{Name: "extraConfigGitUrl", Value: fmt.Sprintf("https://github.com/%s/strategy-configs.git", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))},
			{Name: "extraConfigPath", Value: scPath},
			{Name: "extraConfigGitRevision", Value: compName},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, "", "", releaseStrategyDefaultName)
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

		component, err = fw.AsKubeAdmin.HasController.CreateComponentFromStubSkipInitialChecks(componentDetected, devNamespace, "", "", applicationNameDefault, false)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		err = fw.AsKubeAdmin.CommonController.Github.DeleteRef("strategy-configs", compName)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
		}
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", 2)).To(Succeed())
		})

		It("verifies that in managed namespace will be created a PipelineRun.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					return false
				}

				return strings.Contains(prList.Items[0].Name, "release")
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies a PipelineRun started in managed namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
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
	})
})
