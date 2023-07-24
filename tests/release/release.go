package release

import (
	"fmt"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

var snapshotComponents = []applicationapiv1alpha1.SnapshotComponent{
	{Name: "component-1", ContainerImage: "quay.io/redhat-appstudio/component1@sha256:d5e85e49c89df42b221d972f5b96c6507a8124717a6e42e83fd3caae1031d514"},
	{Name: "component-2", ContainerImage: "quay.io/redhat-appstudio/component2@sha256:a01dfd18cf8ca8b68770b09a9b6af0fd7c6d1f8644c7ab97f0e06c34dfc5860e"},
	{Name: "component-3", ContainerImage: "quay.io/redhat-appstudio/component3@sha256:d90a0a33e4c5a1daf5877f8dd989a570bfae4f94211a8143599245e503775b1f"},
}

var _ = framework.ReleaseSuiteDescribe("[HACBS-1108]test-release-service-happy-path", Label("release", "happy-path", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var devNamespace, managedNamespace string
	var ecPolicy ecp.EnterpriseContractPolicySpec

	var releaseCR *releaseApi.Release
	var pr *v1beta1.PipelineRun

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework("release-service-e2e")
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		// Create the managed namespace
		managedNamespace = "release-service-e2e-managed"
		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

		// Get the Spec from the default EnterpriseContractPolicy. This resource has up to date
		// references and contains a small set of policy rules that should always pass during
		// normal execution.
		k := tekton.KubeController{Tektonctrl: *fw.AsKubeAdmin.TektonController}
		defaultECP, err := k.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred(), "Error when fetching the default ECP: %v", err)
		ecPolicy = defaultECP.Spec

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithComponents(snapshotName, devNamespace, applicationName, snapshotComponents)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy, serviceAccount, paramsReleaseStrategyM6)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationName, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicy, managedNamespace, ecPolicy)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, devNamespace, applicationName, managedNamespace, "", "", releaseStrategyName)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleasePlanName)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
		}
	})

	var _ = Describe("post-release verification.", func() {

		It("tests an associated Release CR is created", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, "", devNamespace)
				return err
			}, releaseCreationTimeout, defaultInterval).Should(Succeed())
		})

		It("makes sure a Release PipelineRun is created in the managed namespace", func() {
			Eventually(func() error {
				_, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				return err
			}, releasePipelineRunCreationTimeout, constants.PipelineRunPollingInterval).Should(Succeed())
		})

		It("makes sure the PipelineRun exists and succeeded", func() {
			Eventually(func() error {
				pr, err = fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(utils.HasPipelineRunFailed(pr)).ToNot(BeTrue(), fmt.Sprintf("Release PipelineRun %s/%s failed", pr.GetNamespace(), pr.GetName()))
				if !pr.IsDone() {
					return fmt.Errorf("release PipelineRun %s/%s did not finish yet", pr.GetNamespace(), pr.GetName())
				}
				Expect(utils.HasPipelineRunSucceeded(pr)).To(BeTrue(), fmt.Sprintf("Release PipelineRun %s/%s did not succceed", pr.GetNamespace(), pr.GetName()))
				return nil
			}, releasePipelineRunCompletionTimeout, constants.PipelineRunPollingInterval).Should(Succeed())
		})

		It("tests a Release CR is marked as successful", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, "", devNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				if !releaseCR.IsReleased() {
					return fmt.Errorf("release CR %s/%s did not finish successfully yet", releaseCR.GetNamespace(), releaseCR.GetName())
				}
				return nil
			}, releaseCreationTimeout, defaultInterval).Should(Succeed())
		})

		It("makes sure the Release references the release PipelineRun.", func() {
			Expect(releaseCR.Status.Processing.PipelineRun).To(Equal(fmt.Sprintf("%s/%s", pr.GetNamespace(), pr.GetName())))
		})
	})
})
