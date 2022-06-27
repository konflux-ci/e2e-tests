package release

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	ApplicationName       = "test"
	SourceReleaseLinkName = "test-releaselink"
	ReleaseName           = "test-release"
)

var _ = framework.ReleaseSuiteDescribe("test-release-service-failures", func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace = "user-" + uuid.New().String()
	var managedNamespace = "managed-" + uuid.New().String()

	var _ = Describe("Failure - Missing matching ReleaseLink", func() {
		BeforeAll(func() {
			// Create the dev namespace
			sourceNamespace, err := framework.CommonController.CreateTestNamespace(devNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", sourceNamespace.Name, err)

			// Create the managed namespace
			managedNamespace, err := framework.CommonController.CreateTestNamespace(managedNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace.Name, err)
		})

		AfterAll(func() {
			// Delete the dev and managed namespaces with all the resources created in them
			Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		})

		var _ = Describe("All required resources are created successfully", func() {
			It("Create an ApplicationSnapshot", func() {
				_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, ApplicationName, snapshotComponents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleaseLink in source namespace", func() {
				_, err := framework.ReleaseController.CreateReleaseLink(SourceReleaseLinkName, devNamespace, ApplicationName, managedNamespace, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a Release in source namespace", func() {
				_, err := framework.ReleaseController.CreateRelease(ReleaseName, devNamespace, snapshotName, SourceReleaseLinkName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		var _ = Describe("A ReleaseLink has to have a matching one in a managed workspace", func() {
			It("The Release have failed with the REASON field set to ReleaseValidationError", func() {
				Eventually(func() bool {
					release, err := framework.ReleaseController.GetRelease(ReleaseName, devNamespace)

					if err != nil || release == nil {
						return false
					}

					releaseReason := release.Status.Conditions[0].Reason
					return release.IsDone() && meta.IsStatusConditionFalse(release.Status.Conditions, "Succeeded") && Expect(releaseReason).To(Equal("ReleaseValidationError"))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

			It("Condition message describes an error finding a matching ReleaseLink", func() {
				Eventually(func() bool {
					release, err := framework.ReleaseController.GetRelease(ReleaseName, devNamespace)

					if err != nil || release == nil {
						return false
					}

					releaseMessage := release.Status.Conditions[0].Message
					return Expect(releaseMessage).Should(ContainSubstring("no ReleaseLink found in target workspace"))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})
		})
	})
})
