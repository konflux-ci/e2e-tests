package release

import (
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
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

	var devNamespace string
	var managedNamespace string
	// var kubeController tekton.KubeController

	var _ = Describe("HACBS-1132: test-release-service-happy-path", Label("release"), func() {
		BeforeAll(func() {
			// Recreate random namespaces names per each test because if using same namespace names, the next test will not be able to create the namespaces as they are terminating
			devNamespace = "user-" + uuid.New().String()
			managedNamespace = "managed-" + uuid.New().String()

			// Create the dev namespace
			_, err := framework.CommonController.CreateTestNamespace(devNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", devNamespace, err)

			// Create the managed namespace
			_, err = framework.CommonController.CreateTestNamespace(managedNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)
		})

		AfterAll(func() {
			// Delete the dev and managed namespaces with all the resources created in them
			Expect(framework.ReleaseController.DeleteRelease(releaseName, devNamespace, true)).NotTo(HaveOccurred())
			Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		})

		var _ = Describe("Creation of the 'Happy path' resources", func() {
			It("Create an ApplicationSnapshot.", func() {
				_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a Release Strategy", func() {
				_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy, "", "", "", "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a ReleasePlan in dev namespace", func() {
				_, err := framework.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationName, managedNamespace, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a ReleasePlanAdmission in managed namespace", func() {
				_, err := framework.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationName, managedNamespace, "", releaseStrategyName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a Release", func() {
				_, err := framework.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleasePlanName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		var _ = Describe("Post-release verification", func() {
			It("A PipelineRun should have been created in the managed namespace", func() {
				Eventually(func() bool {
					pipelineRun, err := framework.ReleaseController.GetPipelineRunInNamespace(applicationName, managedNamespace, releaseName, devNamespace)

					if pipelineRun == nil || err != nil {
						return false
					}

					return true
				}, 1*time.Minute, defaultInterval).Should(BeTrue())
			})

			// The pipelineRun should fail because "serviceaccounts "m7-service-account" not found"
			// This snippet does not hold the all happy-path needed steps to succeed
			// Therefore, I stop the "Post-release verification" here.
		})
	})
})
