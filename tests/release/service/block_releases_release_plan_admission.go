package service

import (
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

var _ = framework.ReleaseServiceSuiteDescribe("[RELEASE-2136] Release CR fails when block-releases true in ReleasePlanAdmission.", Label("release-service", "release-neg", "negBlockReleases"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var devNamespace = "block-rp-dev"
	var managedNamespace = "block-rp-managed"

	var releaseCR *releaseApi.Release
	var snapshotName = "snapshot"
	var destinationReleasePlanAdmissionName = "sre-production"
	var releaseName = "release"

	AfterEach(framework.ReportFailure(&fw))

	BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithComponents(snapshotName, "", releasecommon.ApplicationName, devNamespace, []v1alpha1.SnapshotComponent{})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, managedNamespace, "", nil, nil, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, "", devNamespace, releasecommon.ReleaseStrategyPolicy, releasecommon.ReleasePipelineServiceAccountDefault, []string{releasecommon.ApplicationName}, true, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
			},
		}, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, releasecommon.SourceReleasePlanName)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(Succeed())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(Succeed())
		}
	})

	var _ = Describe("post-release verification.", func() {
		It("block-releases true in ReleasePlanAdmission makes a Release CR set as failed in both IsReleased and IsValid with a proper message to user.", func() {
			Eventually(func() bool {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, "", devNamespace)
				if releaseCR.HasReleaseFinished() {
					return !(releaseCR.IsValid() && releaseCR.IsReleased()) &&
						strings.Contains(releaseCR.Status.Conditions[0].Message, "Release validation failed")
				}
				return false
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(BeTrue())
		})
	})
})
