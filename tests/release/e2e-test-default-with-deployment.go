package release

import (
	"fmt"
	"os"
	"strings"

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
	"knative.dev/pkg/apis"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-1199]test-release-e2e-with-deployment", Label("release", "withDeployment", "HACBS"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	var fw *framework.Framework
	var err error
	var compName string
	var devNamespace = utils.GetGeneratedNamespace("release-dev")
	var managedNamespace = utils.GetGeneratedNamespace("release-managed")
	var component *appservice.Component

	BeforeAll(func() {
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
		GinkgoWriter.Println("Managed Namespace created: %s ", managedNamespace)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(redhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, serviceAccount, true)
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

		scPath := "release-default-with-deployment.yaml"
		Expect(fw.AsKubeAdmin.CommonController.Github.CreateRef("strategy-configs", "main", compName)).To(Succeed())
		_, err = fw.AsKubeAdmin.CommonController.Github.CreateFile("strategy-configs", scPath, string(scYaml), compName)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-deploy-strategy", managedNamespace, "deploy-release", "quay.io/hacbs-release/pipeline-deploy-release:0.3", releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, []releaseApi.Params{
			{Name: "extraConfigGitUrl", Value: fmt.Sprintf("https://github.com/%s/strategy-configs.git", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))},
			{Name: "extraConfigPath", Value: scPath},
			{Name: "extraConfigGitRevision", Value: compName},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.GitOpsController.CreateEnvironment(releaseEnvironment, managedNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, releaseEnvironment, "", "mvp-deploy-strategy")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(devNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", releaseStrategyServiceAccountDefault, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		component, err = fw.AsKubeAdmin.HasController.CreateComponentFromStub(componentDetected, devNamespace, "", "", applicationNameDefault)
		Expect(err).NotTo(HaveOccurred())

		workingDir, err := os.Getwd()
		if err != nil {
			GinkgoWriter.Printf(err.Error())
		}

		// Download copy-applications.sh script from release-utils repo
		args := []string{"https://raw.githubusercontent.com/hacbs-release/release-utils/main/copy-application.sh", "-o", "copy-application.sh"}
		err = utils.ExecuteCommandInASpecificDirectory("curl", args, workingDir)
		Expect(err).NotTo(HaveOccurred())

		args = []string{"775", "copy-application.sh"}
		err = utils.ExecuteCommandInASpecificDirectory("chmod", args, workingDir)
		Expect(err).NotTo(HaveOccurred())

		// Copying application in dev namespace to managed namespace
		args = []string{managedNamespace, "-a", devNamespace + "/" + applicationNameDefault}
		err = utils.ExecuteCommandInASpecificDirectory("./copy-application.sh", args, workingDir)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterAll(func() {
		err = fw.AsKubeAdmin.CommonController.Github.DeleteRef("strategy-configs", compName)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
		}
		// TODO: Uncomment this lines once: https://issues.redhat.com/browse/RHTAPBUGS-409 is solved
		/*if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		}*/
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

	It("tests a Release should report the deployment was successful.", func() {
		Eventually(func() bool {
			releaseCreated, err := fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
			if releaseCreated == nil || err != nil || len(releaseCreated.Status.Conditions) < 2 {
				return false
			}

			return releaseCreated.IsDeployed()
		}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
	})
})
