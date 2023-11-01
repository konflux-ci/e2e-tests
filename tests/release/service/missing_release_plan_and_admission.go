package service

import (
	"strings"

	"github.com/redhat-appstudio/application-api/api/v1alpha1"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
)

var _ = framework.ReleaseServiceSuiteDescribe("[HACBS-2360] Release CR fails when missing ReleasePlan and ReleasePlanAdmission.", Label("release-service", "release-neg", "negMissingReleasePlan", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var devNamespace, managedNamespace string

	var releaseCR *releaseApi.Release
	AfterEach(framework.ReportFailure(&fw))

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework("release-neg-rp-dev")
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		// Create the managed namespace
		managedNamespace = "release-neg-rp-managed"
		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithComponents(snapshotName, "", applicationName, devNamespace, []v1alpha1.SnapshotComponent{})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, "", devNamespace, releaseStrategyPolicy, serviceAccount, []string{applicationName}, true, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: "https://github.com/redhat-appstudio/release-service-catalog"},
				{Name: "revision", Value: "main"},
				{Name: "pathInRepo", Value: "pipelines/e2e/e2e.yaml"},
			},
		}, nil)
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
		It("missing ReleasePlan makes a Release CR set as failed in both IsReleased and IsValid with a proper message to user.", func() {
			Eventually(func() bool {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, "", devNamespace)
				if releaseCR.HasReleaseFinished() {
					return !(releaseCR.IsValid() && releaseCR.IsReleased()) &&
						strings.Contains(releaseCR.Status.Conditions[0].Message, "Release validation failed")
				}
				return false
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})
		It("missing ReleasePlanAdmission makes a Release CR set as failed in both IsReleased and IsValid with a proper message to user.", func() {
			Expect(fw.AsKubeAdmin.ReleaseController.DeleteReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
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
