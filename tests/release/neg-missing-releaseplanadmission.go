package release

import (
	"strings"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-2360] Release CR fails when missing ReleasePlanAdmission.", Label("release", "release-neg", "negMissingReleasePlanAdmission", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var devNamespace, managedNamespace string
	var ecPolicy ecp.EnterpriseContractPolicySpec

	var releaseCR *releaseApi.Release
	AfterEach(framework.ReportFailure(&fw))

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework("release-neg-rpa-dev")
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		// Create the managed namespace
		managedNamespace = "release-neg-rpa-managed"
		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

		// Get the Spec from the default EnterpriseContractPolicy. This resource has up to date
		// references and contains a small set of policy rules that should always pass during
		// normal execution.
		defaultECP, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred(), "Error when fetching the default ECP: %v", err)
		ecPolicy = defaultECP.Spec
		_, err = fw.AsKubeAdmin.HasController.CreateApplication(applicationName, devNamespace)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithComponents(snapshotName, "", applicationName, devNamespace, snapshotComponents)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy, serviceAccount, paramsReleaseStrategy)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationName, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicy, managedNamespace, ecPolicy)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleasePlanName)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
	})

	var _ = Describe("post-release verification.", func() {

		It("tests a Release CR failed in both IsReleased and IsValid with a proper message to user.", func() {
			Eventually(func() bool {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, "", devNamespace)
				if releaseCR.HasReleaseFinished() {
					return !(releaseCR.IsValid() && releaseCR.IsReleased()) &&
						strings.Contains(releaseCR.Status.Conditions[0].Message, "Release validation failed")
				}
				return false
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})

	})
})
