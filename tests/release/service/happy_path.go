package service

import (
	"encoding/json"
	"fmt"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/contract"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("Release service happy path", Label("release-service", "happy-path", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
	var err error
	var compName string
	var devNamespace, managedNamespace string
	var verifyEnterpriseContractTaskName = "verify-enterprise-contract"
	var releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"

	var component *appservice.Component
	var releaseCR *releaseApi.Release

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace("happy-path"))
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace
		managedNamespace = utils.GetGeneratedNamespace("happy-path-managed")
		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: %v", err)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(releasecommon.ReleasePipelineServiceAccountDefault, managedNamespace, releasecommon.ManagednamespaceSecret, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(releasecommon.RedhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, releasecommon.HacbsReleaseTestsTokenSecret, constants.DefaultPipelineServiceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, constants.DefaultPipelineServiceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		publicKey, err := fw.AsKubeAdmin.TektonController.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())
		Expect(fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(
			publicKey, releasecommon.PublicSecretNameAuth, managedNamespace)).To(Succeed())

		defaultECP, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred())
		policy := contract.PolicySpecWithSourceConfig(defaultECP.Spec, ecp.SourceConfig{Include: []string{"@slsa3"}, Exclude: []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"}})

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(releasecommon.ApplicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		component = releasecommon.CreateComponent(*fw, devNamespace, releasecommon.ApplicationNameDefault, releasecommon.ComponentName, releasecommon.GitSourceComponentUrl, "", ".", "Dockerfile", constants.DefaultDockerBuildPipelineBundle)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, managedNamespace, "", nil)
		Expect(err).NotTo(HaveOccurred())

		data, err := json.Marshal(map[string]interface{}{
			"mapping": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"component":  compName,
						"repository": releasedImagePushRepo,
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(releasecommon.TargetReleasePlanAdmissionName, managedNamespace, "", devNamespace, releasecommon.ReleaseStrategyPolicyDefault, releasecommon.ReleasePipelineServiceAccountDefault, []string{releasecommon.ApplicationNameDefault}, true, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/e2e/e2e.yaml"},
			},
		}, &runtime.RawExtension{
			Raw: data,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releasecommon.ReleaseStrategyPolicyDefault, managedNamespace, policy)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasecommon.ReleasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", releasecommon.ReleasePipelineServiceAccountDefault, managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
	})

	var _ = Describe("Post-release verification", func() {
		It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "",
				fw.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, nil)).To(Succeed())
		})

		It("verifies that a Release CR should have been created in the dev namespace", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				return err
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
		})

		It("verifies that Release PipelineRun is triggered", func() {
			Expect(fw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
		})

		It("verifies that Enterprise Contract Task has succeeded in the Release PipelineRun", func() {
			Eventually(func() error {
				pr, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				ecTaskRunStatus, err := fw.AsKubeAdmin.TektonController.GetTaskRunStatus(fw.AsKubeAdmin.CommonController.KubeRest(), pr, verifyEnterpriseContractTaskName)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("the status of the %s TaskRun on the release pipeline is: %v", verifyEnterpriseContractTaskName, ecTaskRunStatus.Status.Conditions)
				Expect(tekton.DidTaskSucceed(ecTaskRunStatus)).To(BeTrue())
				return nil
			}, releasecommon.ReleasePipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed())
		})

		It("verifies that a Release is marked as succeeded.", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				if err != nil {
					return err
				}
				if !releaseCR.IsReleased() {
					return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
				}
				return nil
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
		})
	})
})
