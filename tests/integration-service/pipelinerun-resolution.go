package integration

import (
	"fmt"
	"strings"
	"time"

	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	intgteststat "github.com/konflux-ci/integration-service/pkg/integrationteststatus"

	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Pending temporarily disabled till STONEINTG-1333 is resolved
var _ = framework.IntegrationServiceSuiteDescribe("Integration Service E2E tests ITS PipelineRun Resolution", Pending, Label("integration-service", "pipelinerun-resolution"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var prHeadSha string
	var integrationTestScenario *integrationv1beta2.IntegrationTestScenario
	var failingIntegrationTestScenario *integrationv1beta2.IntegrationTestScenario
	var timeout, interval time.Duration
	var originalComponent *appstudioApi.Component
	var pipelineRun *pipeline.PipelineRun
	var snapshot *appstudioApi.Snapshot
	var applicationName, componentName, componentBaseBranchName, pacBranchName, testNamespace string

	AfterEach(framework.ReportFailure(&f))

	Describe("with happy path for general flow of Integration service", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("resolution"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			applicationName = createApp(*f, testNamespace)
			originalComponent, componentName, pacBranchName, componentBaseBranchName = createComponent(*f, testNamespace, applicationName, componentRepoNameForResolution, componentGitSourceURLForRosuResolution)

			integrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPassPipelinerun, "pipelinerun", []string{"application"})
			Expect(err).ShouldNot(HaveOccurred())

			failingIntegrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoTask, "pipelinerun", []string{"application"})
			Expect(err).ShouldNot(HaveOccurred())

		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentName, snapshot)
			}

			// Delete new branches created by PaC and a testing branch used as a component's base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForResolution, pacBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForResolution, componentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
		})

		When("a new Component is created", func() {
			It("should have a related PaC init PR created", func() {
				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(componentRepoNameForResolution)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == pacBranchName {
							prHeadSha = pr.Head.GetSHA()
							pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, prHeadSha)
							if err == nil {
								return true
							}
						}
					}
					return false
				}, longTimeout, constants.PipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchName, componentRepoNameForStatusReporting))

				// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable

			})

			It("triggers a build PipelineRun", Label("integration-service"), func() {
				pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("verifies if the build PipelineRun contains the finalizer", Label("integration-service"), func() {
				Eventually(func() error {
					pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
					Expect(err).ShouldNot(HaveOccurred())
					if !controllerutil.ContainsFinalizer(pipelineRun, pipelinerunFinalizerByIntegrationService) {
						return fmt.Errorf("build pipelineRun %s/%s doesn't contain the finalizer: %s yet", pipelineRun.GetNamespace(), pipelineRun.GetName(), pipelinerunFinalizerByIntegrationService)
					}
					return nil
				}, 1*time.Minute, 1*time.Second).Should(Succeed(), "timeout when waiting for finalizer to be added")
			})

			It("waits for build PipelineRun to succeed", Label("integration-service"), func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
				Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(originalComponent, "", "", "",
					f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
			})
		})

		When("the build pipelineRun run succeeded", func() {
			It("checks if the BuildPipelineRun have the annotation of chains signed", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, chainsSignedAnnotation)).To(Succeed())
			})

			It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())

			})

			It("checks if the Build PipelineRun got annotated with Snapshot name", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, snapshotAnnotation)).To(Succeed())
			})

			It("verifies that the finalizer has been removed from the build pipelinerun", func() {
				timeout = time.Second * 300
				interval = time.Second * 5
				Eventually(func() error {
					pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
					Expect(err).ShouldNot(HaveOccurred())
					if controllerutil.ContainsFinalizer(pipelineRun, pipelinerunFinalizerByIntegrationService) {
						return fmt.Errorf("build pipelineRun %s/%s still contains the finalizer: %s", pipelineRun.GetNamespace(), pipelineRun.GetName(), pipelinerunFinalizerByIntegrationService)
					}
					return nil
				}, timeout, interval).Should(Succeed(), "timeout when waiting for finalizer to be removed")
			})

			It("checks if all of the integrationPipelineRuns passed", Label("slow"), func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName, snapshot, []string{integrationTestScenario.Name})).To(Succeed())
			})

			It("checks if the passed status of integration test is reported in the Snapshot", func() {
				Eventually(func() error {
					snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", testNamespace)
					Expect(err).ShouldNot(HaveOccurred())

					statusDetail, err := f.AsKubeDeveloper.IntegrationController.GetIntegrationTestStatusDetailFromSnapshot(snapshot, integrationTestScenario.Name)
					Expect(err).ToNot(HaveOccurred())

					fmt.Printf("statusDetail: %+v\n", statusDetail)
					if statusDetail.Status != intgteststat.IntegrationTestStatusTestPassed {
						return fmt.Errorf("test status for scenario: %s, doesn't have expected value %s with timeout %s, within the snapshot: %s, has actual result %s", integrationTestScenario.Name, intgteststat.IntegrationTestStatusTestPassed, timeout, snapshot.Name, statusDetail.Status)
					}
					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed())
			})

			It("checks if the finalizer was removed from all of the related Integration pipelineRuns", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForFinalizerToGetRemovedFromAllIntegrationPipelineRuns(testNamespace, applicationName, snapshot, []string{integrationTestScenario.Name})).To(Succeed())
			})
		})

		When("integration pipelineRun is created it passes, annotations and labels not overwritten by integration service", func() {

			It("checks integration pipelineRun passed", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName, snapshot, []string{integrationTestScenario.Name})).To(Succeed())
			})

			It("verifies that existing labels and annotations are not overwritten by integration service", func() {
				integrationPipelineRun, err := f.AsKubeDeveloper.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot.Name, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				// Verify critical PipelinesAsCode annotations are preserved
				expectedAnnotations := map[string]string{
					"pipelinesascode.tekton.dev/on-target-branch": "[main]",
					"pipelinesascode.tekton.dev/on-event":         "[push]",
					"pipelinesascode.tekton.dev/max-keep-runs":    "5",
				}

				for key, expectedValue := range expectedAnnotations {
					actualValue, exists := integrationPipelineRun.Annotations[key]
					Expect(exists).To(BeTrue(), fmt.Sprintf("Expected annotation %s to exist", key))
					Expect(actualValue).To(Equal(expectedValue), fmt.Sprintf("Expected annotation %s to have value %s, but got %s", key, expectedValue, actualValue))
				}

				// Verify critical labels are preserved
				expectedLabels := map[string]string{
					"pipelines.appstudio.openshift.io/type":   "test",
					"test.appstudio.openshift.io/test":        "component",
					"pipelinesascode.tekton.dev/event-type":   "push",
					"pipelinesascode.tekton.dev/state":        "completed",
					"pipelinesascode.tekton.dev/git-provider": "github",
				}

				for key, expectedValue := range expectedLabels {
					actualValue, exists := integrationPipelineRun.Labels[key]
					Expect(exists).To(BeTrue(), fmt.Sprintf("Expected label %s to exist", key))
					Expect(actualValue).To(Equal(expectedValue), fmt.Sprintf("Expected label %s to have value %s, but got %s", key, expectedValue, actualValue))
				}

				// Verify that dynamic labels set by integration service are also present
				dynamicLabels := []string{
					"appstudio.openshift.io/snapshot",
					"appstudio.openshift.io/component",
					"appstudio.openshift.io/application",
					"test.appstudio.openshift.io/scenario",
				}

				for _, labelKey := range dynamicLabels {
					_, exists := integrationPipelineRun.Labels[labelKey]
					Expect(exists).To(BeTrue(), fmt.Sprintf("Expected integration service label %s to exist", labelKey))
				}
			})

			It("verifies that PipelinesAsCode specific annotations with dynamic values are preserved", func() {
				integrationPipelineRun, err := f.AsKubeDeveloper.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot.Name, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				// Verify PaC annotations with dynamic values exist (but don't check exact values since they're dynamic)
				pacAnnotationsToCheck := []string{
					"pipelinesascode.tekton.dev/repo-url",
					"pipelinesascode.tekton.dev/sha-title",
					"pipelinesascode.tekton.dev/sha-url",
					"pipelinesascode.tekton.dev/git-auth-secret",
					"pipelinesascode.tekton.dev/installation-id",
				}

				for _, annotation := range pacAnnotationsToCheck {
					_, exists := integrationPipelineRun.Annotations[annotation]
					Expect(exists).To(BeTrue(), fmt.Sprintf("Expected PaC annotation %s to be preserved", annotation))
				}

				// Verify PaC labels with dynamic values exist
				pacLabelsToCheck := []string{
					"pipelinesascode.tekton.dev/sender",
					"pipelinesascode.tekton.dev/check-run-id",
					"pipelinesascode.tekton.dev/branch",
					"pipelinesascode.tekton.dev/url-org",
					"pipelinesascode.tekton.dev/original-prname",
					"pipelinesascode.tekton.dev/url-repository",
					"pipelinesascode.tekton.dev/repository",
					"pipelinesascode.tekton.dev/sha",
				}

				for _, label := range pacLabelsToCheck {
					_, exists := integrationPipelineRun.Labels[label]
					Expect(exists).To(BeTrue(), fmt.Sprintf("Expected PaC label %s to be preserved", label))
				}
			})

			// TODO: After STONEINTG-1166 is done, we can remove the Pending label
			It("verifies that ResolutionRequest is deleted after pipeline resolution", Pending, func() {
				interval = time.Second * 5

				Eventually(func() error {
					relatedResolutionRequests, err := f.AsKubeDeveloper.IntegrationController.GetRelatedResolutionRequests(testNamespace, integrationTestScenario)
					if err != nil {
						// If ResolutionRequest CRD doesn't exist, consider this as success since the feature might not be enabled
						if strings.Contains(err.Error(), "ResolutionRequest CRD not available") {
							return nil
						}
						return fmt.Errorf("failed to get related ResolutionRequests: %v", err)
					}

					if len(relatedResolutionRequests) > 0 {
						names := f.AsKubeDeveloper.IntegrationController.GetResolutionRequestNames(relatedResolutionRequests)
						return fmt.Errorf("found %d ResolutionRequest(s) still present in namespace %s for scenario %s: %v",
							len(relatedResolutionRequests), testNamespace, integrationTestScenario.Name, names)
					}

					return nil
				}, shortTimeout, interval).Should(Succeed(), "ResolutionRequest objects should be cleaned up after pipeline resolution is complete")
			})

			// TODO: After STONEINTG-1166 is done, we can remove the Pending label
			It("verifies that no orphaned ResolutionRequests remain in namespace after test completion", Pending, func() {
				// Check for any ResolutionRequests that might have been left behind
				relatedResolutionRequests, err := f.AsKubeDeveloper.IntegrationController.GetRelatedResolutionRequests(testNamespace, integrationTestScenario)
				if err != nil {
					// Skip if ResolutionRequest CRD is not available
					if strings.Contains(err.Error(), "ResolutionRequest CRD not available") {
						Skip("ResolutionRequest CRD not available in cluster, skipping orphan check")
						return
					}
					Expect(err).NotTo(HaveOccurred(), "Failed to check for orphaned ResolutionRequests")
				}

				// Should be nil or empty at this point
				if len(relatedResolutionRequests) > 0 {
					names := f.AsKubeDeveloper.IntegrationController.GetResolutionRequestNames(relatedResolutionRequests)
					// Log for debugging but only fail if these are old ResolutionRequests (older than 5 minutes)
					fmt.Printf("Found %d ResolutionRequest(s) in namespace %s: %v\n", len(relatedResolutionRequests), testNamespace, names)

					// Check if any are older than expected cleanup time
					currentTime := time.Now()
					oldResolutionRequests := []string{}

					for _, rr := range relatedResolutionRequests {
						creationTime := rr.GetCreationTimestamp()
						if currentTime.Sub(creationTime.Time) > 5*time.Minute {
							oldResolutionRequests = append(oldResolutionRequests, rr.GetName())
						}
					}

					Expect(oldResolutionRequests).To(BeEmpty(), "Found old ResolutionRequest objects that should have been cleaned up")
				}
			})

			It("validates that second integration pipelineRun failed due to resolution not pipelinerun", func() {
				snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", testNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				statusDetail, err := f.AsKubeDeveloper.IntegrationController.GetIntegrationTestStatusDetailFromSnapshot(snapshot, failingIntegrationTestScenario.Name)

				Expect(err).ToNot(HaveOccurred())
				Expect(statusDetail.Details).To(ContainSubstring("Creation of pipelineRun failed during creation due to: resolution for"))

			})

		})

	})

})
