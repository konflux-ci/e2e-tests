package release

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"knative.dev/pkg/apis"
)

const (
	snapshotName          = "snapshot"
	sourceReleaseLinkName = "source-release-link"
	targetReleaseLinkName = "target-release-link"
	releaseStrategyName   = "strategy"
	releaseName           = "release"
	releasePipelineName   = "release-pipeline"
	applicationName       = "application"
	releasePipelineBundle = "quay.io/hacbs-release/demo:m5-alpine"
	releaseStrategyPolicy = "policy"

	avgPipelineCompletionTime     = 10 * time.Minute
	defaultInterval               = 100 * time.Millisecond
	failure1ApplicationName       = "test"
	failure1SourceReleaseLinkName = "test-releaselink"
	failure1ReleaseName           = "test-release"
)

var snapshotComponents = []gitopsv1alpha1.ApplicationSnapshotComponent{
	{"component-1", "quay.io/redhat-appstudio/component1@sha256:d5e85e49c89df42b221d972f5b96c6507a8124717a6e42e83fd3caae1031d514"},
	{"component-2", "quay.io/redhat-appstudio/component2@sha256:a01dfd18cf8ca8b68770b09a9b6af0fd7c6d1f8644c7ab97f0e06c34dfc5860e"},
	{"component-3", "quay.io/redhat-appstudio/component3@sha256:d90a0a33e4c5a1daf5877f8dd989a570bfae4f94211a8143599245e503775b1f"},
}

var _ = framework.ReleaseSuiteDescribe("test-demo", func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace = uuid.New().String()
	var managedNamespace = uuid.New().String()

	var failureDevNamespace = "user-" + uuid.New().String()
	var failureManagedNamespace = "managed-" + uuid.New().String()

	BeforeAll(func() {
		// Create the dev namespace
		demo, err := framework.CommonController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", demo.Name, err)

		// Create the managed namespace
		namespace, err := framework.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", namespace.Name, err)
	})

	AfterAll(func() {
		// Delete the dev and managed namespaces with all the resources created in them
		Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
		Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	})

	var _ = Describe("Creation of the 'Happy path' resources", func() {
		It("Create an ApplicationSnapshot.", func() {
			_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Strategy", func() {
			_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ReleaseLink in dev namespace", func() {
			_, err := framework.ReleaseController.CreateReleaseLink(sourceReleaseLinkName, devNamespace, applicationName, managedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ReleaseLink in managed namespace", func() {
			_, err := framework.ReleaseController.CreateReleaseLink(targetReleaseLinkName, managedNamespace, applicationName, devNamespace, releaseStrategyName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create a Release", func() {
			_, err := framework.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleaseLinkName)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	var _ = Describe("Post-release verification", func() {
		It("A PipelineRun should have been created in the managed namespace", func() {
			Eventually(func() error {
				_, err := framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, devNamespace)

				return err
			}, 1*time.Minute, defaultInterval).Should(BeNil())
		})

		It("The PipelineRun should exist and succeed", func() {
			Eventually(func() bool {
				pipelineRun, err := framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, devNamespace)

				if pipelineRun == nil || err != nil {
					return false
				}

				return pipelineRun.HasStarted() && pipelineRun.IsDone() && pipelineRun.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
		})

		It("The Release should have succeeded", func() {
			Eventually(func() bool {
				release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

				if err != nil {
					return false
				}

				return release.IsDone() && meta.IsStatusConditionTrue(release.Status.Conditions, "Succeeded")
			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
		})

		It("The Release should reference the release PipelineRun", func() {
			var pipelineRun *v1beta1.PipelineRun

			Eventually(func() bool {
				pipelineRun, err = framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, devNamespace)

				return pipelineRun != nil && err == nil
			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())

			release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(release.Status.ReleasePipelineRun).Should(Equal(fmt.Sprintf("%s/%s", pipelineRun.Namespace, pipelineRun.Name)))
		})
	})

	var _ = Describe("Failure- Missing matching ReleaseLink", func() {
		BeforeAll(func() {
			// Create the dev namespace
			sourceNamespace, err := framework.CommonController.CreateTestNamespace(failureDevNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", sourceNamespace.Name, err)

			// Create the managed namespace
			managedNamespace, err := framework.CommonController.CreateTestNamespace(failureManagedNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace.Name, err)
		})

		AfterAll(func() {
			// Delete the dev and managed namespaces with all the resources created in them
			Expect(framework.CommonController.DeleteNamespace(failureDevNamespace)).NotTo(HaveOccurred())
			Expect(framework.CommonController.DeleteNamespace(failureManagedNamespace)).NotTo(HaveOccurred())
		})

		var _ = Describe("All required resources are created successfully", func() {
			It("Create an ApplicationSnapshot for M5 application", func() {
				_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, failureDevNamespace, failure1ApplicationName, snapshotComponents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleaseLink in source namespace", func() {
				_, err := framework.ReleaseController.CreateReleaseLink(failure1SourceReleaseLinkName, failureDevNamespace, failure1ApplicationName, failureManagedNamespace, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a Release in source namespace", func() {
				_, err := framework.ReleaseController.CreateRelease(failure1ReleaseName, failureDevNamespace, snapshotName, failure1SourceReleaseLinkName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		var _ = Describe("A ReleaseLink has to have a matching one in a managed workspace", func() {
			release := &v1alpha1.Release{}
			releaseReason := ""
			releaseMessage := ""

			It("The Release have failed with the REASON field set to ReleaseValidationError", func() {
				Eventually(func() bool {
					release, err = framework.ReleaseController.GetRelease(failure1ReleaseName, failureDevNamespace)

					if err != nil || release == nil {
						return false
					}

					releaseReason = release.Status.Conditions[0].Reason

					return release.IsDone() && meta.IsStatusConditionFalse(release.Status.Conditions, "Succeeded") && Expect(releaseReason).To(Equal("ReleaseValidationError"))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

			It("Condition message describes an error finding a matching ReleaseLink", func() {
				Eventually(func() bool {
					release, err = framework.ReleaseController.GetRelease(failure1ReleaseName, failureDevNamespace)

					if err != nil || release == nil {
						return false
					}

					releaseMessage = release.Status.Conditions[0].Message

					return Expect(releaseMessage).Should(ContainSubstring("no ReleaseLink found in target workspace"))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})
		})
	})
})
