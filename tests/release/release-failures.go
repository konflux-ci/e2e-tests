package release

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"k8s.io/apimachinery/pkg/api/meta"
)

var _ = framework.ReleaseSuiteDescribe("test-release-service-failures", func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace string
	var managedNamespace string

	var _ = Describe("Failure - Missing matching ReleaseLink", func() {
		BeforeAll(func() {
			// Recreate random namespaces names per each test because if using same namespaces names, the next test will not be able to create the namespaces as they are terminating
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
			Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		})

		var _ = Describe("All required resources are created successfully", func() {
			It("Create an ApplicationSnapshot", func() {
				_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleaseLink in source namespace", func() {
				_, err := framework.ReleaseController.CreateReleaseLink(sourceReleaseLinkName, devNamespace, applicationName, managedNamespace, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a Release in source namespace", func() {
				_, err := framework.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleaseLinkName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		var _ = Describe("A ReleaseLink has to have a matching one in a managed workspace", func() {
			It("The Release have failed with the REASON field set to ReleaseValidationError", func() {
				Eventually(func() bool {
					release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

					if err != nil || release == nil {
						return false
					}

					releaseReason := release.Status.Conditions[0].Reason
					return release.IsDone() && meta.IsStatusConditionFalse(release.Status.Conditions, "Succeeded") && Expect(releaseReason).To(Equal("ReleaseValidationError"))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

			It("Condition message describes an error finding a matching ReleaseLink", func() {
				Eventually(func() bool {
					release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

					if err != nil || release == nil {
						return false
					}

					releaseMessage := release.Status.Conditions[0].Message
					return Expect(releaseMessage).Should(ContainSubstring("no ReleaseLink found in target workspace"))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})
		})
	})

	var _ = Describe("Failure - Missing release pipeline", func() {
		BeforeAll(func() {
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
			Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		})

		var _ = Describe("All required resources are created successfully", func() {
			It("Create an ApplicationSnapshot", func() {
				_, err := framework.ReleaseController.CreateApplicationSnapshot(failureSnapshotName, devNamespace, failureApplicationName, snapshotComponents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleaseLink in source namespace", func() {
				_, err := framework.ReleaseController.CreateReleaseLink(failureSourceReleaseLinkName, devNamespace, failureApplicationName, managedNamespace, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleaseLink in managed namespace", func() {
				_, err := framework.ReleaseController.CreateReleaseLink(failureManagedReleaseLinkName, managedNamespace, failureApplicationName, devNamespace, failureStrategyName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleaseStrategy in managed namespace", func() {
				_, err := framework.ReleaseController.CreateReleaseStrategy(failureStrategyName, managedNamespace, failureMissingPipelineName, releasePipelineBundle, releaseStrategyPolicy)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a Release in source namespace", func() {
				_, err := framework.ReleaseController.CreateRelease(failureReleaseName, devNamespace, failureSnapshotName, failureSourceReleaseLinkName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		var _ = Describe("A Release must relate to an existing pipeline in the managed workspace", func() {
			It("The Release have failed with the REASON field set to ReleasePipelineFailed", func() {
				Eventually(func() bool {
					release, err := framework.ReleaseController.GetRelease(failureReleaseName, devNamespace)

					if err != nil || release == nil {
						return false
					}

					releaseReason := release.Status.Conditions[0].Reason
					return release.IsDone() && meta.IsStatusConditionFalse(release.Status.Conditions, "Succeeded") && Expect(releaseReason).To(Equal("ReleasePipelineFailed"))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

			It("Condition message describes an error retrieving pipeline", func() {
				Eventually(func() bool {
					release, err := framework.ReleaseController.GetRelease(failureReleaseName, devNamespace)

					if err != nil || release == nil {
						return false
					}

					tmpMessage := "could not find object in image with kind: pipeline and name: " + failureMissingPipelineName
					releaseMessage := release.Status.Conditions[0].Message
					return Expect(releaseMessage).Should(ContainSubstring("Error retrieving pipeline")) && Expect(releaseMessage).Should(ContainSubstring(tmpMessage))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})
		})
	})
})
