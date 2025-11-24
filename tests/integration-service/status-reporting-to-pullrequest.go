package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-github/v44/github"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"

	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	intgteststat "github.com/konflux-ci/integration-service/pkg/integrationteststatus"
	"github.com/konflux-ci/operator-toolkit/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.IntegrationServiceSuiteDescribe("Status Reporting of Integration tests", Label("integration-service", "github-status-reporting"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var prNumber int
	var mergeResultSha, prHeadSha string
	var snapshot *appstudioApi.Snapshot
	var component *appstudioApi.Component
	var pipelineRun, testPipelinerun, failedPipelineRun *tektonv1.PipelineRun
	var integrationTestScenarioPass, integrationTestScenarioFail, integrationTestScenarioOptional *integrationv1beta2.IntegrationTestScenario
	var applicationName, componentName, componentBaseBranchName, pacBranchName, testNamespace string
	var mergeResult *github.PullRequestMergeResult
	var labels, annotations map[string]string

	AfterEach(framework.ReportFailure(&f))

	Describe("with status reporting of Integration tests in CheckRuns", Ordered, func() {
		BeforeAll(func() {
			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				Skip("Skipping this test due to configuration issue with Spray proxy")
			}

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("stat-rep"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			if utils.IsPrivateHostname(f.OpenshiftConsoleHost) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			applicationName = createApp(*f, testNamespace)

			// Wait for application to be available before creating integration test scenarios
			Eventually(func() error {
				_, err := f.AsKubeAdmin.HasController.GetApplication(applicationName, testNamespace)
				return err
			}, time.Minute*2, time.Second*5).Should(Succeed(),
				fmt.Sprintf("Application %s should be available in namespace %s", applicationName, testNamespace))

			component, componentName, pacBranchName, componentBaseBranchName = createComponent(*f, testNamespace, applicationName, componentRepoNameForStatusReporting, componentGitSourceURLForStatusReporting)

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPass, "", []string{})
			Expect(err).ShouldNot(HaveOccurred())
			integrationTestScenarioFail, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoFail, "", []string{})
			Expect(err).ShouldNot(HaveOccurred())
			integrationTestScenarioOptional, err = f.AsKubeAdmin.IntegrationController.CreateOptionalIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoFail, "", []string{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRunWithType(componentName, applicationName, testNamespace, "build", "", "")
				if err != nil {
					GinkgoWriter.Printf("Build PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
					return err
				}
				if !pipelineRun.HasStarted() {
					return fmt.Errorf("build pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
				}
				return nil
			}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the component %s/%s", testNamespace, componentName))
			labels = pipelineRun.GetLabels()
			annotations = pipelineRun.GetAnnotations()
			fmt.Print(componentBaseBranchName)
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentName, snapshot)
			}

			// Delete new branches created by PaC and a testing branch used as a component's base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForStatusReporting, pacBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForStatusReporting, componentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
		})

		When("a new Component with specified custom branch is created", Label("custom-branch"), func() {
			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})

			It("should have a related PaC init PR created", func() {
				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(componentRepoNameForStatusReporting)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == pacBranchName {
							prNumber = pr.GetNumber()
							prHeadSha = pr.Head.GetSHA()
							return true
						}
					}
					return false
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchName, componentRepoNameForStatusReporting))
				// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRunWithType(componentName, applicationName, testNamespace, "build", prHeadSha, "")
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("initialized integration test status is reported to github", func() {
				Eventually(func() error {
					status, err := f.AsKubeAdmin.CommonController.Github.GetCheckRunStatus(integrationTestScenarioPass.Name, componentRepoNameForStatusReporting, prHeadSha, prNumber)
					if status != "queued" || err != nil {
						return fmt.Errorf("error occurred when checking pending integration test checkRun %v", err)
					}
					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the pending checkrun for the component  %s/%s and integrationTestScenarioPass %s", testNamespace, componentName, integrationTestScenarioPass.Name))
			})

			It("should lead to build PipelineRun finishing successfully", func() {
				isPass, logs := f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineToBeFinished(testNamespace, applicationName, component.Name, "")
				Expect(isPass).Should(Succeed(), fmt.Sprintf("build pipelinerun fails for NameSpace/Application/Component %s/%s/%s with logs: %s", testNamespace, applicationName, componentName, logs))
			})

			It("eventually leads to the build PipelineRun's status reported at Checks tab", func() {
				expectedCheckRunName := fmt.Sprintf("%s-%s", componentName, "on-pull-request")
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, componentRepoNameForStatusReporting, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
			})
		})

		When("the PaC build pipelineRun run succeeded", func() {
			It("checks if the BuildPipelineRun have the annotation of chains signed", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, chainsSignedAnnotation)).To(Succeed())
			})

			It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentName, testNamespace)
				Expect(err).ToNot(HaveOccurred())
			})

			It("checks if the Build PipelineRun got annotated with Snapshot name", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, snapshotAnnotation)).To(Succeed())
			})
		})

		When("the Snapshot was created", func() {
			It("should find both the related Integration PipelineRuns", func() {
				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))

				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioFail.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioFail.Name))

				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioOptional.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioOptional.Name))
			})
		})

		When("Integration PipelineRuns are created", func() {
			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioFail, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioOptional, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})
		})

		When("Integration PipelineRuns completes successfully", func() {
			It("should lead to Snapshot CR being marked as failed", FlakeAttempts(3), func() {
				// Snapshot marked as Failed because one of its Integration test failed (as expected)
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRunWithType(componentName, applicationName, testNamespace, "build", prHeadSha, "")
				Expect(err).Should(Succeed())
				Eventually(func() bool {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRunWithType(componentName, applicationName, testNamespace, "build", prHeadSha, "")
					if err != nil {
						return false
					}
					snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", testNamespace)
					return err == nil && !f.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot)
				}, time.Minute*3, time.Second*5).Should(BeTrue(), fmt.Sprintf("Timed out waiting for Snapshot to be marked as failed %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
			})

			It("eventually leads to the status reported at Checks tab for the successful Integration PipelineRun", func() {
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(integrationTestScenarioPass.Name, componentRepoNameForStatusReporting, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
			})

			It("eventually leads to the status reported at Checks tab for the failed Integration PipelineRun", func() {
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(integrationTestScenarioFail.Name, componentRepoNameForStatusReporting, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionFailure))
			})

			It("eventually leads to the status reported at Checks tab for the optional Integration PipelineRun", func() {
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(integrationTestScenarioOptional.Name, componentRepoNameForStatusReporting, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionNeutral))
			})

			It("checks if the optional Integration Test Scenario status is reported in the Snapshot", func() {
				Eventually(func() error {
					snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", testNamespace)
					Expect(err).ShouldNot(HaveOccurred())

					statusDetail, err := f.AsKubeDeveloper.IntegrationController.GetIntegrationTestStatusDetailFromSnapshot(snapshot, integrationTestScenarioOptional.Name)
					Expect(err).ToNot(HaveOccurred())

					if statusDetail.Status != intgteststat.IntegrationTestStatusTestFail {
						return fmt.Errorf("test status doesn't have expected value %s", intgteststat.IntegrationTestStatusTestFail)
					}
					return nil
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(Succeed())
			})

			It("checks if the finalizer was removed from the optional Integration PipelineRun", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForFinalizerToGetRemovedFromIntegrationPipeline(integrationTestScenarioOptional, snapshot, testNamespace)).To(Succeed())
			})

			It("merging the PR, expected to succeed ", func() {
				Eventually(func() error {
					mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(componentRepoNameForStatusReporting, prNumber)
					return err
				}, time.Minute).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, multiComponentRepoNameForGroupSnapshot))
				mergeResultSha = mergeResult.GetSHA()
				GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)
			})

			It("leads to triggering a push PipelineRun", func() {
				Eventually(func() error {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, mergeResultSha)
					if err != nil {
						GinkgoWriter.Printf("Push PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("push pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, componentName))

			})

			It("verifies that Push PipelineRuns completed", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioFail, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})

			It("validates the Integration test scenario PipelineRun is reported to merge request CheckRuns, and it pass", func() {
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(integrationTestScenarioPass.Name, componentRepoNameForStatusReporting, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))

			})

			It("eventually leads to the status reported at Checks tab for the failed Integration PipelineRun", func() {
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(integrationTestScenarioFail.Name, componentRepoNameForStatusReporting, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionFailure))
			})
		})

		When("The git-provider annotation is missing", func() {
			It("should set the git-reporting-failure annotation correctly", func() {
				// Get snapshot from the cluster
				snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", testNamespace)
				Expect(err).NotTo(HaveOccurred())
				updatedSnapshot := snapshot.DeepCopy()

				// update the provider annotation to a non-existent provider and reset the git-reporting-failure annotation
				err = metadata.SetAnnotation(updatedSnapshot, pipelinesAsCodeGitProviderAnnotation, "not-supported-provider")
				Expect(err).NotTo(HaveOccurred())
				err = metadata.DeleteAnnotation(updatedSnapshot, gitReportingFailureAnnotation)
				Expect(err).NotTo(HaveOccurred())

				// Change lastUpdateTime for one scenario in order to trigger re-reconciliation from statusreport controller
				currentStatus := updatedSnapshot.Annotations[snapshotStatusAnnotation]
				var arr []map[string]any
				err = json.Unmarshal([]byte(currentStatus), &arr)
				Expect(err).NotTo(HaveOccurred())
				Expect(arr).ToNot(BeEmpty())
				arr[0]["lastUpdateTime"] = time.Now().UTC().Format(time.RFC3339)
				newStatusBytes, err := json.Marshal(arr)
				Expect(err).NotTo(HaveOccurred())

				err = metadata.SetAnnotation(updatedSnapshot, snapshotStatusAnnotation, string(newStatusBytes))
				Expect(err).NotTo(HaveOccurred())

				Expect(f.AsKubeAdmin.IntegrationController.PatchSnapshot(snapshot, updatedSnapshot)).Should(Succeed())
				Eventually(func() error {
					reconciledSnapshot, err := f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", snapshot.Namespace)
					if err != nil {
						GinkgoWriter.Printf("failed to get snapshot: %v", err)
						return fmt.Errorf("error occurred when fetching snapshot to check gitReportingFailureAnnotation: %v", err)
					}
					val, ok := reconciledSnapshot.Annotations[gitReportingFailureAnnotation]
					if !ok {
						GinkgoWriter.Printf("gitReportingFailureAnnotation does not exist. Annotations: %+v", reconciledSnapshot.Annotations)
						return fmt.Errorf("gitReportingFailureAnnotation does not exist. Annotations: %+v", reconciledSnapshot.Annotations)
					}
					if val == "" {
						GinkgoWriter.Printf("gitReportingFailureAnnotation is empty. Annotations: %+v", reconciledSnapshot.Annotations)
						return fmt.Errorf("gitReportingFailureAnnotation is empty. Annotations: %+v", reconciledSnapshot.Annotations)
					}
					return nil
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), "git-reporting-failure-annotation was never set")
			})
		})

		When("build pipelinerun fails", func() {
			It("build pipelinerun is created but fails", func() {
				// delete snapshot creation report annotation to create a new build plr manually
				delete(annotations, snapshotCreationReport)
				failedPipelineRun = &tektonv1.PipelineRun{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "failing-build-plr-" + util.GenerateRandomString(4),
						Namespace:   testNamespace,
						Labels:      labels,
						Annotations: annotations,
					},
					Spec: tektonv1.PipelineRunSpec{
						PipelineRef: &tektonv1.PipelineRef{
							ResolverRef: tektonv1.ResolverRef{
								Resolver: "git",
								Params: tektonv1.Params{
									{
										Name:  "url",
										Value: tektonv1.ParamValue{Type: "string", StringVal: "https://github.com/konflux-ci/integration-examples.git"},
									},
									{
										Name:  "revision",
										Value: tektonv1.ParamValue{Type: "string", StringVal: "main"},
									},
									{
										Name:  "pathInRepo",
										Value: tektonv1.ParamValue{Type: "string", StringVal: "pipelines/integration_resolver_pipeline_pass.yaml"},
									},
								},
							},
						},
						TaskRunTemplate: tektonv1.PipelineTaskRunTemplate{
							ServiceAccountName: constants.DefaultPipelineServiceAccount,
						},
					},
				}
				failedPipelineRun.Labels = labels
				failedPipelineRun.Annotations = annotations
				failedPipelineRun, err = f.AsKubeAdmin.TektonController.CreatePipelineRun(failedPipelineRun, testNamespace)
				Expect(err).Should(Succeed())

				// check PipelineRun status
				pipelineRunTimeout := int(time.Duration(20) * time.Minute)
				Expect(f.AsKubeAdmin.TektonController.WatchPipelineRunSucceeded(failedPipelineRun.Name, testNamespace, pipelineRunTimeout)).Should(Succeed())
			})

			It("build pipelinerun failure is reported to integration test checkRun", func() {
				Eventually(func() error {
					text, err := f.AsKubeAdmin.CommonController.Github.GetCheckRunText(integrationTestScenarioPass.Name, componentRepoNameForStatusReporting, prHeadSha, prNumber)
					if err != nil || !strings.Contains(text, "Failed to create snapshot") {
						GinkgoWriter.Printf("failed to check expected checkRun text, actual text is %s: \n", err, text)
						return fmt.Errorf("error occurred when checking failing integration test checkRun text")
					}
					return nil
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the failing checkrun for the component  %s/%s and integrationTestScenarioPass %s", testNamespace, componentName, integrationTestScenarioPass.Name))
			})
		})

	})
})
