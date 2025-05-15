package service

import (
	kubeapi "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
	"strings"

	"github.com/konflux-ci/application-api/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"

	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = framework.ReleaseServiceSuiteDescribe("[HACBS-2360] Release CR fails when missing ReleasePlan and ReleasePlanAdmission.", Label("release-service", "release-neg", "negMissingReleasePlan"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var kubeAdminClient *framework.ControllerHub
	var err error

	var devNamespace = "neg-rp-dev"
	var managedNamespace = "neg-rp-managed"

	var releaseCR *releaseApi.Release
	var snapshotName = "snapshot"
	var destinationReleasePlanAdmissionName = "sre-production"
	var releaseName = "release"

	var testEnvironment = utils.GetEnv("TEST_ENVIRONMENT", releasecommon.UpstreamTestEnvironment)

	AfterEach(framework.ReportFailure(&fw))

	BeforeAll(func() {
		if testEnvironment == releasecommon.DownstreamTestEnvironment {
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
			Expect(err).NotTo(HaveOccurred())
			devNamespace = fw.UserNamespace
			managedNamespace = utils.GetGeneratedNamespace(managedNamespace)
		} else {
			var asAdminClient *kubeapi.CustomClient
			asAdminClient, err = kubeapi.NewAdminKubernetesClient()
			Expect(err).ShouldNot(HaveOccurred())
			kubeAdminClient, err = framework.InitControllerHub(asAdminClient)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubeAdminClient.CommonController.CreateTestNamespace(devNamespace)
			Expect(err).ShouldNot(HaveOccurred())
		}

		_, err = kubeAdminClient.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

		_, err = kubeAdminClient.IntegrationController.CreateSnapshotWithComponents(snapshotName, "", releasecommon.ApplicationName, devNamespace, []v1alpha1.SnapshotComponent{})
		Expect(err).NotTo(HaveOccurred())

		_, err = kubeAdminClient.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, "", devNamespace, releasecommon.ReleaseStrategyPolicy, releasecommon.ReleasePipelineServiceAccountDefault, []string{releasecommon.ApplicationName}, true, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
			},
		}, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = kubeAdminClient.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, releasecommon.SourceReleasePlanName)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(kubeAdminClient.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			if testEnvironment == releasecommon.DownstreamTestEnvironment {
				Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
			}
		}
	})

	var _ = Describe("post-release verification.", func() {
		It("missing ReleasePlan makes a Release CR set as failed in both IsReleased and IsValid with a proper message to user.", func() {
			Eventually(func() bool {
				releaseCR, err = kubeAdminClient.ReleaseController.GetRelease(releaseName, "", devNamespace)
				if releaseCR.HasReleaseFinished() {
					return !(releaseCR.IsValid() && releaseCR.IsReleased()) &&
						strings.Contains(releaseCR.Status.Conditions[0].Message, "Release validation failed")
				}
				return false
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(BeTrue())
		})
		It("missing ReleasePlanAdmission makes a Release CR set as failed in both IsReleased and IsValid with a proper message to user.", func() {
			Expect(kubeAdminClient.ReleaseController.DeleteReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
			Eventually(func() bool {
				releaseCR, err = kubeAdminClient.ReleaseController.GetRelease(releaseName, "", devNamespace)
				if releaseCR.HasReleaseFinished() {
					return !(releaseCR.IsValid() && releaseCR.IsReleased()) &&
						strings.Contains(releaseCR.Status.Conditions[0].Message, "Release validation failed")
				}
				return false
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(BeTrue())
		})
	})
})
