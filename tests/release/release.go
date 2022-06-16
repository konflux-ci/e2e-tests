package release

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
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

	avgPipelineCompletionTime = 2 * time.Minute
	defaultInterval           = 100 * time.Millisecond

	Failure1ApplicationName       = "test"
	Failure1SourceNamespace       = "matching-scenario-user"
	Failure1ManagedNamespace      = "matching-scenario-managed"
	Failure1SourceReleaseLinkName = "test-releaselink"
	Failure1ReleaseName           = "test-release"
)

var snapshotImages = []v1alpha1.Image{
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

	BeforeAll(func() {
		// Create the dev namespace
		demo, err := framework.HasController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", demo.Name, err)

		// Create the managed namespace
		namespace, err := framework.HasController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", namespace.Name, err)
	})

	AfterAll(func() {
		// Delete the dev and managed namespaces with all the resources created in them
		Expect(framework.ReleaseController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
		Expect(framework.ReleaseController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	})

	var _ = Describe("Creation of the 'Happy path' resources", func() {
		It("Create an ApplicationSnapshot.", func() {
			_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, applicationName, snapshotImages)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Strategy", func() {
			_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Link in dev namespace", func() {
			_, err := framework.ReleaseController.CreateReleaseLink(sourceReleaseLinkName, devNamespace, applicationName, managedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Link in managed namespace", func() {
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

	var _ = Describe("Failure test #1", func() {

		BeforeAll(func() {
			// Create the dev namespace
			sourceNamespace, err := framework.HasController.CreateTestNamespace(Failure1SourceNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", sourceNamespace.Name, err)

			// Create the managed namespace
			managedNamespace, err := framework.HasController.CreateTestNamespace(Failure1ManagedNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace.Name, err)

			// wait until both namespaces have been created
			Eventually(func() bool {
				// demo and managed namespaces should not exist
				ret1 := framework.ReleaseController.CheckIfNamespaceExists(Failure1SourceNamespace)
				ret2 := framework.ReleaseController.CheckIfNamespaceExists(Failure1ManagedNamespace)

				// return True if only one namespace still exists
				// return False if both demo and managed namespaces don't exist
				return ret1 && ret2
			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue(), "Timedout creating namespaces")
		})

		AfterAll(func() {
			// Delete the dev and managed namespaces with all the resources created in them
			Expect(framework.ReleaseController.DeleteNamespace(Failure1SourceNamespace)).NotTo(HaveOccurred())
			Expect(framework.ReleaseController.DeleteNamespace(Failure1ManagedNamespace)).NotTo(HaveOccurred())
		})

		// ***
		// Create resources for Failure test #1
		// Missing matching ReleaseLink in managed workspace
		/*
			   Missing matching ReleaseLink in managed workspace
			   A Release cannot be executed unless there is a ReleaseLink in the managed workspace which matches the one in the user workspace.
			   Two ReleaseLinks are considered matching if they both specify the same application and each of them target the other's workspace.
			   This scenario shows the situation where a Release is created in the user workspace with a ReleaseLink to a managed workspace,
			   but there is no matching ReleaseLink in the managed workspace.

			   Tests Prereq

				$ git clone https://github.com/redhat-appstudio/release-service.git
				$ cd release-service/demo/m4/failures
				$ oc apply -f missing_matching_release_link.yaml

			   	What to expect -

				After running the commands above, you should expect to find:
				a ReleaseLink test-releaselink in the matching-scenario-user namespace;
				a Release test-release in the matching-scenario-user namespace and failed;
				The following fields are set as follows:
				Status="False"
				REASON="ReleaseValidationError"
				Message="no ReleaseLink found in target workspace 'matching-scenario-managed' with target 'matching-scenario-user' and application 'test'"
		*/

		var _ = Describe("Failure test #1 - create resources", func() {
			It("Create a an ApplicationSnapshot for M5 failure#1 application", func() {
				_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, Failure1SourceNamespace, Failure1ApplicationName, snapshotImages)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create Release Link in failure#1 source namespace", func() {
				_, err := framework.ReleaseController.CreateReleaseLink(Failure1SourceReleaseLinkName, Failure1SourceNamespace, Failure1ApplicationName, Failure1ManagedNamespace, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create a Release in failure#1 source namespace", func() {
				_, err := framework.ReleaseController.CreateRelease(Failure1ReleaseName, Failure1SourceNamespace, snapshotName, Failure1SourceReleaseLinkName)
				Expect(err).NotTo(HaveOccurred())
			})

		})

		// Verification of test failures
		/* What to expect -

		After running the commands above, you should expect to find:
		a ReleaseLink test-releaselink in the matching-scenario-user namespace;
		a Release test-release in the matching-scenario-user namespace and failed;
		the REASON field of the test-release Release set to Error.

		*/
		var _ = Describe("Failure test #1 Verification", func() {

			release := &v1alpha1.Release{}
			releaseReason := ""

			// Check if there is a ReleaseLink in managed namespace
			It("The ReleaseLink test-releaselink has been created in the failure#1 source namespace ", func() {
				Eventually(func() bool {
					releaseLink, err := framework.ReleaseController.GetReleaseLink(Failure1SourceReleaseLinkName, Failure1SourceNamespace)

					return releaseLink != nil && err == nil
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

			It("The Release have been created and failed with the REASON field set to ReleaseValidationError", func() {
				Eventually(func() bool {
					release, err = framework.ReleaseController.GetRelease(Failure1ReleaseName, Failure1SourceNamespace)

					if err != nil {
						return false
					}

					if release != nil {
						releaseReason = release.Status.Conditions[0].Reason
					} else {
						return false
					}

					return release.IsDone() && meta.IsStatusConditionFalse(release.Status.Conditions, "Succeeded") && Expect(releaseReason).To(Equal("ReleaseValidationError"))
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

		})

	})

})
