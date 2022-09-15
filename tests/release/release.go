package release

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"

	// appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"

	// klog "k8s.io/klog/v2"
	"knative.dev/pkg/apis"
)

var snapshotComponents = []gitopsv1alpha1.ApplicationSnapshotComponent{
	{"component-1", "quay.io/redhat-appstudio/component1@sha256:d5e85e49c89df42b221d972f5b96c6507a8124717a6e42e83fd3caae1031d514"},
	{"component-2", "quay.io/redhat-appstudio/component2@sha256:a01dfd18cf8ca8b68770b09a9b6af0fd7c6d1f8644c7ab97f0e06c34dfc5860e"},
	{"component-3", "quay.io/redhat-appstudio/component3@sha256:d90a0a33e4c5a1daf5877f8dd989a570bfae4f94211a8143599245e503775b1f"},
}

var _ = framework.ReleaseSuiteDescribe("test-release-service-happy-path", Label("release"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var source_namespace string
	var managedNamespace string

	var _ = Describe("test-release-service-happy-path", func() {
		BeforeAll(func() {
			// Recreate random namespaces names per each test because if using same namespace names, the next test will not be able to create the namespaces as they are terminating
			source_namespace = "user-" + uuid.New().String()
			managedNamespace = "managed-" + uuid.New().String()

			// Create the dev namespace
			_, err := framework.CommonController.CreateTestNamespace(source_namespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", source_namespace, err)

			// Create the managed namespace
			_, err = framework.CommonController.CreateTestNamespace(managedNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)
		})

		AfterAll(func() {
			// Delete the dev and managed namespaces with all the resources created in them
			Expect(framework.CommonController.DeleteNamespace(source_namespace)).NotTo(HaveOccurred())
			Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		})

		var _ = Describe("Creation of the 'Happy path' resources", func() {
			It("Create an ApplicationSnapshot.", func() {
				_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, source_namespace, applicationName, snapshotComponents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create Release Strategy", func() {
				_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy, releaseStrategyServiceAccount)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleasePlan in dev namespace", func() {
				AutoReleaseLabel := ""
				_, err := framework.ReleaseController.CreateReleasePlan(sourceReleasePlanName, source_namespace, applicationName, managedNamespace, AutoReleaseLabel)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleasePlanAdmission in managed namespace", func() {
				AutoReleaseLabel := ""
				_, err := framework.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, source_namespace, applicationName, managedNamespace, AutoReleaseLabel, releaseStrategyName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a Release", func() {
				_, err := framework.ReleaseController.CreateRelease(releaseName, source_namespace, snapshotName, sourceReleasePlanName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		var _ = Describe("Post-release verification", func() {
			It("A PipelineRun should have been created in the managed namespace", func() {
				Eventually(func() error {
					_, err := framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, source_namespace)

					return err
				}, 1*time.Minute, defaultInterval).Should(BeNil())
			})

			It("The PipelineRun should exist and succeed", func() {
				Eventually(func() bool {
					pipelineRun, err := framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, source_namespace)

					if pipelineRun == nil || err != nil {
						return false
					}

					return pipelineRun.HasStarted() && pipelineRun.IsDone() && pipelineRun.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

			It("The Release should have succeeded", func() {
				Eventually(func() bool {
					release, err := framework.ReleaseController.GetRelease(releaseName, source_namespace)

					if err != nil {
						return false
					}

					return release.IsDone() && meta.IsStatusConditionTrue(release.Status.Conditions, "Succeeded")
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

			It("The Release should reference the release PipelineRun", func() {
				var pipelineRun *v1beta1.PipelineRun

				Eventually(func() bool {
					pipelineRun, err = framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, source_namespace)

					return pipelineRun != nil && err == nil
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())

				release, err := framework.ReleaseController.GetRelease(releaseName, source_namespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(release.Status.ReleasePipelineRun).Should(Equal(fmt.Sprintf("%s/%s", pipelineRun.Namespace, pipelineRun.Name)))
			})
		})
	})
})
