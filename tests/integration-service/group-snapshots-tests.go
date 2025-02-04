package integration

import (
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-github/v44/github"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"

	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

var _ = framework.IntegrationServiceSuiteDescribe("Creation of group snapshots for monorepo and multiple repos", Label("integration-service", "group-snapshot-creation"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error
	var prNumbers = make(map[string]int)
	var multiRepoPRNumber int
	var prHeadShas = make(map[string]string)
	var prRepos = make(map[string]string)
	var mergeResultSha, mergeMultiResultSha string
	var pacBranchNames []string
	var componentNames []string
	var mergeResult *github.PullRequestMergeResult
	var pipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass, integrationTestScenarioFail *integrationv1beta2.IntegrationTestScenario
	var applicationName, testNamespace string
	var multiComponentBaseBranchName, multiComponentPRBranchName string
	var monorepoBaseBranchName string
	var lastCreatedFileSha string
	var componentSnapshots = make(map[string]*appstudioApi.Snapshot)

	AfterEach(framework.ReportFailure(&f))

	Describe("with status reporting of Integration tests in CheckRuns", Ordered, func() {
		BeforeAll(func() {
			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				Skip("Skipping this test due to configuration issue with Spray proxy")
			}

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("group"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			if utils.IsPrivateHostname(f.OpenshiftConsoleHost) {
				Skip("Using private cluster (not reachable from GitHub), skipping...")
			}

			applicationName = createApp(*f, testNamespace)

			// ----- Multi-Component Setup -----
			// Create a unique base branch for the multi-component repo
			multiComponentBaseBranchName = fmt.Sprintf("multi-base-%s", util.GenerateRandomString(6))
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(
				multiComponentRepoNameForGroupSnapshot,
				multiComponentDefaultBranch,
				multiComponentGitRevision,
				multiComponentBaseBranchName,
			)
			Expect(err).ShouldNot(HaveOccurred())

			// Create a PR branch for multi-component repo
			multiComponentPRBranchName = fmt.Sprintf("multi-pr-%s", util.GenerateRandomString(6))

			// ----- Monorepo Setup -----
			monorepoBaseBranchName = fmt.Sprintf("monorepo-base-%s", util.GenerateRandomString(6))

			err = f.AsKubeAdmin.CommonController.Github.CreateRef(
				componentRepoNameForGroupIntegration,
				monorepoComponentDefaultBranch,
				monorepoComponentGitRevision,
				monorepoBaseBranchName,
			)
			Expect(err).ShouldNot(HaveOccurred(), "failed to create base branch for monorepo")

			// ----- Integration Test Scenarios -----
			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(
				"", applicationName, testNamespace, gitURL, revision, pathInRepoPass, []string{},
			)
			Expect(err).ShouldNot(HaveOccurred())

			integrationTestScenarioFail, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(
				"", applicationName, testNamespace, gitURL, revision, pathInRepoFail, []string{},
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			// Clean up all components if the tests haven't failed.
			if !CurrentSpecReport().Failed() {
				for _, compName := range componentNames {
					// Only attempt cleanup if the component still exists
					_, err := f.AsKubeAdmin.HasController.GetComponent(compName, testNamespace)
					if err == nil {
						cleanup(*f, testNamespace, applicationName, compName, componentSnapshots[compName])
					} else {
						GinkgoWriter.Printf("Skipping cleanup for component %q (not found): %v\n", compName, err)
					}
				}
			}

			// Cleanup PaC branches for all components (multi- and monorepo)
			for _, pacBranchName := range pacBranchNames {
				// Delete from multi-component repo
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, pacBranchName)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
				}
				// Delete from monorepo repo
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForGroupIntegration, pacBranchName)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
				}
			}

			// Delete base and PR branches for multi-component repo
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, multiComponentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, multiComponentPRBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}

			// Delete base branch for monorepo
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForGroupIntegration, monorepoBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
		})

		When("creating and testing multiple components", func() {
			for _, contextDir := range multiComponentContextDirs {
				func(contextDir string) { // Anonymous function to prevent variable mutation
					componentName := fmt.Sprintf("%s-%s", contextDir, util.GenerateRandomString(6))
					pacBranchName := constants.PaCPullRequestBranchPrefix + componentName
					pacBranchNames = append(pacBranchNames, pacBranchName) // used later for cleanup

					var component *appstudioApi.Component
					var pipelineRun *pipeline.PipelineRun

					It(fmt.Sprintf("creates component %s", componentName), func() {
						component = createComponentWithCustomBranch(
							*f, testNamespace, applicationName, componentName,
							multiComponentGitSourceURLForGroupSnapshot, multiComponentBaseBranchName, contextDir,
						)
						Expect(component).NotTo(BeNil())
						componentNames = append(componentNames, componentName)
					})

					It(fmt.Sprintf("triggers a Build PipelineRun for %s", componentName), func() {
						timeout := 10 * time.Minute
						interval := time.Second

						Eventually(func() error {
							pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
							if err != nil {
								return err
							}
							if !pipelineRun.HasStarted() {
								return fmt.Errorf("build PipelineRun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
							}
							return nil
						}, timeout, interval).Should(Succeed(), fmt.Sprintf("Timed out waiting for build PipelineRun for %s/%s", testNamespace, componentName))
					})

					// PR tracking for each component
					It(fmt.Sprintf("should lead to a PaC PR creation for %s", componentName), func() {
						timeout := 5 * time.Minute
						interval := time.Second

						Eventually(func() bool {
							prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(multiComponentRepoNameForGroupSnapshot)
							Expect(err).ShouldNot(HaveOccurred())

							for _, pr := range prs {
								if pr.Head.GetRef() == pacBranchName {
									prNumbers[componentName] = pr.GetNumber() // Save PR number for this component
									prHeadShas[componentName] = pr.Head.GetSHA()
									return true
								}
							}
							return false
						}, timeout, interval).Should(BeTrue(), fmt.Sprintf(
							"Timed out when waiting for PaC PR (branch %s) to be created in %s repository",
							pacBranchName, multiComponentRepoNameForGroupSnapshot,
						))

						pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, prHeadShas[componentName])
						Expect(err).ShouldNot(HaveOccurred())
					})

					It(fmt.Sprintf("should lead to build PipelineRun finishing successfully for %s", componentName), func() {
						component, err := f.AsKubeAdmin.HasController.GetComponent(componentName, testNamespace)
						Expect(err).ShouldNot(HaveOccurred())

						Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(
							component, "", f.AsKubeAdmin.TektonController,
							&has.RetryOptions{Retries: 2, Always: true}, pipelineRun,
						)).To(Succeed())
					})

					When(fmt.Sprintf("the Build PLR for %s is finished successfully", componentName), func() {
						var localSnapshot *appstudioApi.Snapshot

						It("checks if the Snapshot is created", func() {
							localSnapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", pipelineRun.Name, componentName, testNamespace)
							componentSnapshots[componentName] = localSnapshot
							Expect(err).ShouldNot(HaveOccurred())
							Expect(localSnapshot).NotTo(BeNil())
						})

						It("should find the related Integration PipelineRuns", func() {
							Expect(localSnapshot).NotTo(BeNil()) // Confirm snapshot is already fetched
							testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(
								integrationTestScenarioPass.Name, localSnapshot.Name, testNamespace,
							)
							Expect(err).ToNot(HaveOccurred())
							Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(localSnapshot.Name))
							Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))
						})

						It(fmt.Sprintf("integration pipeline for %s should end with success", componentName), func() {
							timeout := 10 * time.Minute

							Eventually(func() error {
								integrationPipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
								if err != nil {
									return err
								}
								if !integrationPipelineRun.HasStarted() {
									return fmt.Errorf("integration PipelineRun %s/%s hasn't started yet", integrationPipelineRun.GetNamespace(), integrationPipelineRun.GetName())
								}
								return nil
							}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("Timed out waiting for Integration PipelineRun to start for %s/%s", testNamespace, componentName))
						})

						It(fmt.Sprintf("should merge the init PaC PR successfully for %s", componentName), func() {
							Eventually(func() error {
								mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(multiComponentRepoNameForGroupSnapshot, prNumbers[componentName])
								return err
							}, time.Minute).Should(BeNil(), fmt.Sprintf("Error merging PaC pull request #%d in repo %s", prNumbers[componentName], multiComponentRepoNameForGroupSnapshot))

							mergeResultSha = mergeResult.GetSHA()
							GinkgoWriter.Printf("Merged result sha: %s for PR #%d\n", mergeResultSha, prNumbers[componentName])
						})
					})
				}(contextDir)
			}
		})

		// monorepo (single-component) tests
		When("creating and testing the monorepo component", func() {
			var monorepoComponent *appstudioApi.Component
			var prNumber int
			var prHeadSha string
			var monorepoPACBranch string

			It("creates the monorepo component successfully", func() {
				monorepoComponent = createComponentWithCustomBranch(
					*f, testNamespace, applicationName,
					monorepoRepoNameForGroupSnapshot+"-"+util.GenerateRandomString(6),
					componentGitSourceURLForGroupIntegration, // ✅ Use the constant here
					monorepoBaseBranchName,
					"",
				)
				Expect(monorepoComponent).NotTo(BeNil())

				// compute and record PAC branch
				monorepoPACBranch = constants.PaCPullRequestBranchPrefix + monorepoComponent.Name
				pacBranchNames = append(pacBranchNames, monorepoPACBranch)
				componentNames = append(componentNames, monorepoComponent.Name)
			})

			It("waits for the PR to be created", func() {
				timeout := 10 * time.Minute
				interval := time.Second

				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(
						componentRepoNameForGroupIntegration,
					)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == monorepoPACBranch {
							prNumber = pr.GetNumber()
							prHeadSha = pr.Head.GetSHA()
							prNumbers[monorepoComponent.Name] = prNumber
							prHeadShas[monorepoComponent.Name] = prHeadSha
							prRepos[monorepoComponent.Name] = componentRepoNameForGroupIntegration
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf(
					"timed out waiting for PaC PR %q", monorepoPACBranch,
				))
			})

			It("waits for the build PipelineRun to be created", func() {
				timeout := 2 * time.Minute
				interval := 5 * time.Second

				Eventually(func() error {
					_, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(
						monorepoComponent.Name, applicationName,
						testNamespace, prHeadSha,
					)
					return err
				}, timeout, interval).Should(Succeed(), fmt.Sprintf(
					"timed out waiting for build PipelineRun for component %q", monorepoComponent.Name,
				))
			})

			It("waits for the build PipelineRun to finish successfully", func() {
				monorepoComponent, err := f.AsKubeAdmin.HasController.GetComponent(monorepoComponent.Name, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(
					monorepoComponent.Name, applicationName, testNamespace, prHeadSha,
				)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(
					monorepoComponent, "", f.AsKubeAdmin.TektonController,
					&has.RetryOptions{Retries: 2, Always: true}, pipelineRun,
				)).To(Succeed())
			})

			It("reports the build status via a successful GitHub CheckRun", func() {
				expectedCheckRunName := fmt.Sprintf(
					"%s-on-pull-request", monorepoComponent.Name,
				)
				Eventually(func() (string, error) {
					return f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(
						expectedCheckRunName,
						componentRepoNameForGroupIntegration,
						prHeadSha,
						prNumber,
					)
				}, 2*time.Minute, 5*time.Second).Should(Equal(constants.CheckrunConclusionSuccess), "CheckRun for monorepo component did not succeed")
			})

			It("waits for Snapshot & integration Pipelines and then merges the PR", func() {
				var localSnapshot *appstudioApi.Snapshot

				Eventually(func() error {
					localSnapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", prHeadSha, monorepoComponent.Name, testNamespace)
					return err
				}, 8*time.Minute, 5*time.Second).Should(Succeed(), "Timed out waiting for Snapshot to be created for monorepo component")

				Expect(localSnapshot).NotTo(BeNil())
				componentSnapshots[monorepoComponent.Name] = localSnapshot

				Eventually(func() error {
					_, err := f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(
						integrationTestScenarioPass.Name,
						localSnapshot.Name,
						testNamespace,
					)
					return err
				}, 3*time.Minute, 5*time.Second).Should(Succeed(), "Timed out waiting for Integration PipelineRun to start for monorepo component")

				Eventually(func() error {
					_, err := f.AsKubeAdmin.CommonController.Github.MergePullRequest(
						componentRepoNameForGroupIntegration,
						prNumber,
					)
					return err
				}, 2*time.Minute, 5*time.Second).Should(Succeed(), fmt.Sprintf("Timed out waiting to merge PR #%d for monorepo component", prNumber))
			})
		})

		// verifying build CheckRun statuses (multi- and monorepo)
		When("verifying build PipelineRun CheckRun statuses for all components", func() {
			for comp, num := range prNumbers {
				comp := comp
				num := num
				It(fmt.Sprintf("reports correct CheckRun for %s build", comp), func() {
					expected := fmt.Sprintf("%s-on-pull-request", comp)
					status, err := f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(
						expected,
						prRepos[comp],
						prHeadShas[comp],
						num,
					)
					Expect(err).NotTo(HaveOccurred())
					Expect(status).To(Equal(constants.CheckrunConclusionSuccess))
				})
			}
		})
	})

	When("Integration PipelineRuns completes successfully", func() {
		It("should lead to Snapshot CR being marked as failed", FlakeAttempts(3), func() {
			// Snapshot is marked as Failed because one of its Integration tests failed.
			var localSnapshot *appstudioApi.Snapshot
			Eventually(func() bool {
				localSnapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", testNamespace)
				return err == nil && !f.AsKubeAdmin.CommonController.HaveTestsSucceeded(localSnapshot)
			}, time.Minute*3, time.Second*5).Should(BeTrue(), fmt.Sprintf("Timed out waiting for Snapshot to be marked as failed %s/%s", localSnapshot.GetNamespace(), localSnapshot.GetName()))
		})

		// Iterate over all components to check the Integration PipelineRun statuses
		for compName, prNum := range prNumbers {
			compName := compName // capture loop variable
			prSha := prHeadShas[compName]
			It(fmt.Sprintf("should report the correct CheckRun status for successful Integration PipelineRun for component %s", compName), func() {
				status, err := f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(
					integrationTestScenarioPass.Name,
					multiComponentRepoNameForGroupSnapshot,
					prSha,
					prNum,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(constants.CheckrunConclusionSuccess))
			})
			It(fmt.Sprintf("should report the correct CheckRun status for failed Integration PipelineRun for component %s", compName), func() {
				status, err := f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(
					integrationTestScenarioFail.Name,
					multiComponentRepoNameForGroupSnapshot,
					prSha,
					prNum,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(constants.CheckrunConclusionFailure))
			})
		}
	})

	When("both the init PaC PRs are merged", func() {
		var localGroupSnapshots *appstudioApi.SnapshotList
		// Update root folder for monorepo
		It("should make changes to the root folder", func() {
			// Use mergeResultSha for monorepo (latest merged PR SHA)
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(
				multiComponentRepoNameForGroupSnapshot, multiComponentDefaultBranch, mergeResultSha, multiComponentPRBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			var lastCreatedFileSha string

			for _, component := range componentNames {
				fileToCreatePath := fmt.Sprintf("%s/sample-file-for-%s.txt", component, component)
				createdFile, err := f.AsKubeAdmin.CommonController.Github.CreateFile(
					multiComponentRepoNameForGroupSnapshot, fileToCreatePath, "Test content for component", multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file: %s", fileToCreatePath))

				if createdFile.SHA != nil { // Prevents panic if SHA is nil
					lastCreatedFileSha = *createdFile.SHA
				}
			}

			pr, err := f.AsKubeAdmin.CommonController.Github.CreatePullRequest(
				multiComponentRepoNameForGroupSnapshot, "SingleRepo multi-component PR", "sample PR body",
				multiComponentPRBranchName, multiComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			GinkgoWriter.Printf("pr #%d got created with sha %s\n", pr.GetNumber(), lastCreatedFileSha)
		})

		// Update files for multi-repo
		It("should make changes to the multiple-repo", func() {
			// Delete all the PipelineRuns in the namespace before sending the PR.
			Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())

			// Use mergeMultiResultSha for multi-repo (latest merged PR SHA for multi-repo)
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(
				componentRepoNameForGroupIntegration, multiComponentDefaultBranch, mergeMultiResultSha, multiComponentPRBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			for _, component := range componentNames {
				fileToCreatePath := fmt.Sprintf("%s/sample-file-for-%s.txt", component, component)
				createdFile, err := f.AsKubeAdmin.CommonController.Github.CreateFile(
					componentRepoNameForGroupIntegration, fileToCreatePath, "Sometimes I drink water to surprise my liver", multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file: %s", fileToCreatePath))

				if createdFile.SHA != nil { // Prevents panic if SHA is nil
					lastCreatedFileSha = *createdFile.SHA
				}
			}

			pr, err := f.AsKubeAdmin.CommonController.Github.CreatePullRequest(
				componentRepoNameForGroupIntegration, "Multirepo component PR", "sample pr body",
				multiComponentPRBranchName, multiComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			multiRepoPRNumber = pr.GetNumber() // capture the multi-repo PR number for later use
			GinkgoWriter.Printf("PR #%d got created\n", multiRepoPRNumber, lastCreatedFileSha)
		})

		It("merges the multi-repo PR successfully", func() {
			var mergeMultiResult *github.PullRequestMergeResult
			Eventually(func() error {
				mergeMultiResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(componentRepoNameForGroupIntegration, multiRepoPRNumber)
				return err
			}, time.Minute).Should(BeNil(), "Error merging multi-repo pull request")

			mergeMultiResultSha = mergeMultiResult.GetSHA()
			GinkgoWriter.Printf("Merged multi-repo result sha: %s\n", mergeMultiResultSha)
		})

		It("waits for the last components' builds to finish", func() {
			for _, component := range componentNames {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineToBeFinished(
					testNamespace, applicationName, component, "")).To(Succeed())
			}
		})

		It("gets all group snapshots and checks if pr-group annotation contains all components", func() {
			Eventually(func() error {
				localGroupSnapshots, err = f.AsKubeAdmin.HasController.GetAllGroupSnapshotsForApplication(applicationName, testNamespace)
				if localGroupSnapshots == nil {
					GinkgoWriter.Println("No group snapshot exists at the moment: %v", err)
					return err
				}
				if err != nil {
					GinkgoWriter.Println("Failed to get all group snapshots: %v", err)
					return err
				}
				return nil
			}, time.Minute*20, constants.PipelineRunPollingInterval).Should(Succeed(), "timeout while waiting for group snapshot")

			annotation := localGroupSnapshots.Items[0].GetAnnotations()
			if annotation, ok := annotation[testGroupSnapshotAnnotation]; ok {
				for _, component := range componentNames {
					Expect(annotation).To(ContainSubstring(component))
				}
			}
		})

		It("makes sure that the group snapshot contains the last build PipelineRun for each component", func() {
			Expect(localGroupSnapshots).NotTo(BeNil(), "Expected localGroupSnapshots to be populated by previous test")

			for _, component := range componentNames {
				pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(
					component, applicationName, testNamespace, false, "")
				Expect(err).ShouldNot(HaveOccurred())

				annotation := localGroupSnapshots.Items[0].GetAnnotations()
				if annotation, ok := annotation[testGroupSnapshotAnnotation]; ok {
					Expect(annotation).To(ContainSubstring(pipelineRun.Name))
				}
			}
		})
	})
})
