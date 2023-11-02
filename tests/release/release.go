package release

import (
	"encoding/json"
	"fmt"

	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = framework.ReleaseSuiteDescribe("Release service happy path", Label("release", "happy-path", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
	var err error
	var compName string
	var devNamespace string
	var managedNamespace = utils.GetGeneratedNamespace("release-service-e2e-managed")

	var component *appservice.Component
	var releaseCR *releaseApi.Release

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework("release-service-e2e")
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: %v", err)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(releasePipelineServiceAccountDefault, managedNamespace, managednamespaceSecret, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(devNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(redhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, serviceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, redhatAppstudioUserSecret, serviceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		publicKey, err := fw.AsKubeAdmin.TektonController.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())
		Expect(fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

		defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
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
		Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "Expected length of the detected Components was not 1")

		for _, compDetected := range cdq.Status.ComponentDetected {
			compName = compDetected.ComponentStub.ComponentName
			componentDetected = compDetected
		}

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		component, err = fw.AsKubeAdmin.HasController.CreateComponent(componentDetected.ComponentStub, devNamespace, "", "", applicationNameDefault, false, map[string]string{})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
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

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, managedNamespace, "", devNamespace, releaseStrategyPolicyDefault, releasePipelineServiceAccountDefault, []string{applicationNameDefault}, true, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: "https://github.com/redhat-appstudio/release-service-catalog"},
				{Name: "revision", Value: "main"},
				{Name: "pathInRepo", Value: "pipelines/e2e/e2e.yaml"},
			},
		}, &runtime.RawExtension{
			Raw: data,
		})
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

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", releasePipelineServiceAccountDefault, managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
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
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", 2, fw.AsKubeAdmin.TektonController)).To(Succeed())
		})

		It("verifies that a Release CR should have been created in the dev namespace", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				return err
			}, releaseCreationTimeout, defaultInterval).Should(Succeed())
		})

		It("verifies that Release PipelineRun is triggered", func() {
			Eventually(func() error {
				pr, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				if err != nil {
					GinkgoWriter.Printf("release pipelineRun for release '%s' in namespace '%s' not created yet: %+v\n", releaseCR.GetName(), releaseCR.GetNamespace(), err)
					return err
				}
				if !pr.HasStarted() {
					return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
				}
				return nil
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(Succeed(), fmt.Sprintf("timed out waiting for a pipelinerun to start for a release %s/%s", releaseCR.GetName(), releaseCR.GetNamespace()))
		})

		It("verifies that Release PipelineRun should eventually succeed", func() {
			Eventually(func() error {
				pr, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				if !pr.IsDone() {
					return fmt.Errorf("release pipelinerun %s/%s did not finish yet", pr.GetNamespace(), pr.GetName())
				}
				Expect(utils.HasPipelineRunSucceeded(pr)).To(BeTrue(), fmt.Sprintf("release pipelinerun %s/%s did not succeed", pr.GetNamespace(), pr.GetName()))
				return nil
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(Succeed())
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
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(Succeed())
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
			}, releaseCreationTimeout, defaultInterval).Should(Succeed())
		})
	})
})
