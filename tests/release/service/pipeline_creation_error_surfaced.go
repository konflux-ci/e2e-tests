package service

import (
	"context"
	"strings"

	"github.com/konflux-ci/application-api/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"

	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("[KONFLUX-12127] Release CR surfaces pipeline creation errors to its status.", ginkgo.Label("release-service", "release-neg", "konflux-12127"), func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	var err error

	var devNamespace = "neg-plr-dev"
	var managedNamespace = "neg-plr-managed"

	var releaseCR *releaseApi.Release
	var snapshotName = "snapshot"
	var destinationReleasePlanAdmissionName = "sre-production"
	var releaseName = "release"

	ginkgo.AfterEach(framework.ReportFailure(&fw))

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

		// Prevent PipelineRun creation in the managed namespace by enforcing a zero quota.
		// When the release controller attempts to create a PipelineRun, the API server returns
		// a 403 Forbidden (quota exceeded), which is classified as a non-retriable creation error
		// and should be surfaced to the Release CR status (KONFLUX-12127 fix).
		_, err = fw.AsKubeAdmin.CommonController.KubeInterface().CoreV1().ResourceQuotas(managedNamespace).Create(
			context.Background(),
			&corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deny-pipelineruns",
					Namespace: managedNamespace,
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						corev1.ResourceName("count/pipelineruns.tekton.dev"): resource.MustParse("0"),
					},
				},
			},
			metav1.CreateOptions{},
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithComponents(snapshotName, "", releasecommon.ApplicationNameDefault, devNamespace, []v1alpha1.SnapshotComponent{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, managedNamespace, "", nil, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, "", devNamespace, releasecommon.ReleaseStrategyPolicy, releasecommon.ReleasePipelineServiceAccountDefault, []string{releasecommon.ApplicationNameDefault}, false, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
			},
		}, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, releasecommon.SourceReleasePlanName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(gomega.Succeed())
		}
	})

	var _ = ginkgo.Describe("post-release verification.", func() {
		ginkgo.It("Release CR status is updated with the pipeline creation error instead of requeueing indefinitely.", func() {
			gomega.Eventually(func() bool {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, "", devNamespace)
				if err != nil || releaseCR == nil {
					return false
				}
				if !releaseCR.HasReleaseFinished() || releaseCR.IsReleased() {
					return false
				}
				for _, condition := range releaseCR.Status.Conditions {
					if strings.Contains(condition.Message, "Release processing failed on managed pipelineRun creation") {
						return true
					}
				}
				return false
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(gomega.BeTrue())
		})
	})
})
