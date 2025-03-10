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
	var mergeResultSha, mergeMultiResultSha string
	var pacBranchNames []string
	var componentNames []string
	var snapshot *appstudioApi.Snapshot
	var groupSnapshots *appstudioApi.SnapshotList
	var mergeResult *github.PullRequestMergeResult
	var pipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass *integrationv1beta2.IntegrationTestScenario
	var applicationName, testNamespace string
	var multiComponentBaseBranchName, multiComponentPRBranchName string

	AfterEach(framework.ReportFailure(&f))

	Describe("with status reporting of Integration tests in CheckRuns", Ordered, func() {
		BeforeAll(func() {
			prNumbers = make(map[string]int)

			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				Skip("Skipping this test due to configuration issue with Spray proxy")
			}

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("group"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			if utils.IsPrivateHostname(f.OpenshiftConsoleHost) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			applicationName = createApp(*f, testNamespace)

			// Create base branches for multi-component definitions
			multiComponentBaseBranchName = fmt.Sprintf("multi-repo-%s", util.GenerateRandomString(6))
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(multiComponentRepoNameForGroupSnapshot, multiComponentDefaultBranch, multiComponentGitRevision, multiComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			err = f.AsKubeAdmin.CommonController.Github.CreateRef(componentRepoNameForGeneralIntegration, multiComponentDefaultBranch, multiRepoComponentGitRevision, multiComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			// PR Branch creation
			multiComponentPRBranchName = fmt.Sprintf("pr-branch-%s", util.GenerateRandomString(6))

			// Create Integration Test Scenario
			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPass, []string{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				for _, component := range componentNames {
					cleanup(*f, testNamespace, applicationName, component, snapshot)
				}
			}

			// Cleanup branches created by PaC
			for _, pacBranchName := range pacBranchNames {
				_ = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, pacBranchName)
				_ = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForGeneralIntegration, pacBranchName)
			}

			// Cleanup base and PR branches
			_ = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, multiComponentBaseBranchName)
			_ = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForGeneralIntegration, multiComponentBaseBranchName)
			_ = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, multiComponentPRBranchName)
			_ = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForGeneralIntegration, multiComponentPRBranchName)
		})

		When("creating and testing multiple components", func() {
			for _, contextDir := range multiComponentContextDirs {
				func(contextDir string) { // Anonymous function to prevent variable mutation
					componentName := fmt.Sprintf("%s-%s", contextDir, util.GenerateRandomString(6))
					pacBranchName := constants.PaCPullRequestBranchPrefix + componentName
					pacBranchNames = append(pacBranchNames, pacBranchName) // 'pacBranchNames' stores the names of all PAC branches for use during cleanup

					var component *appstudioApi.Component
					var prHeadSha string
					var pipelineRun *pipeline.PipelineRun
					var err error

					It(fmt.Sprintf("creates component %s", componentName), func() {
						component = createComponentWithCustomBranch(
							*f, testNamespace, applicationName, componentName,
							multiComponentGitSourceURLForGroupSnapshot, multiComponentBaseBranchName, contextDir,
						)
						Expect(component).NotTo(BeNil())
						componentNames = append(componentNames, component.Name)
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
									prHeadSha = pr.Head.GetSHA()
									return true
								}
							}
							return false
						}, timeout, interval).Should(BeTrue(), fmt.Sprintf(
							"Timed out when waiting for PaC PR (branch %s) to be created in %s repository",
							pacBranchName, multiComponentRepoNameForGroupSnapshot,
						))

						pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, prHeadSha)
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

					It(fmt.Sprintf("eventually leads to build PipelineRun status reported at Checks tab for %s", componentName), func() {
						expectedCheckRunName := fmt.Sprintf("%s-%s", componentName, "on-pull-request")
						Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(
							expectedCheckRunName, multiComponentRepoNameForGroupSnapshot, prHeadSha, prNumbers[componentName],
						)).To(Equal(constants.CheckrunConclusionSuccess))
					})

					When(fmt.Sprintf("the Build PLR for %s is finished successfully", componentName), func() {
						It("checks if the Snapshot is created", func() {
							snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", pipelineRun.Name, componentName, testNamespace)
							Expect(err).ShouldNot(HaveOccurred())
						})

						It("should find the related Integration PipelineRuns", func() {
							testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
							Expect(err).ToNot(HaveOccurred())
							Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
							Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))
						})

						It(fmt.Sprintf("integration pipeline for %s should end with success", componentName), func() {
							timeout := 10 * time.Minute

							Eventually(func() error {
								integrationPipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
								if err != nil {
									return err
								}
								if !pipelineRun.HasStarted() {
									return fmt.Errorf("integration PipelineRun %s/%s hasn't started yet", integrationPipelineRun.GetNamespace(), pipelineRun.GetName())
								}
								return nil
							}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("Timed out waiting for Integration PipelineRun to start for %s/%s", testNamespace, componentName))
						})
					})

					When(fmt.Sprintf("the Snapshot testing for %s is completed successfully", componentName), func() {
						It("should merge the init PaC PR successfully", func() {
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
	})

	When("both the init PaC PRs are merged", func() {
		// 🔹 Update root folder for monorepo
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

		// 🔹 Update files for multi-repo
		It("should make changes to the multiple-repo", func() {
			// Use mergeMultiResultSha for multi-repo (latest merged PR SHA for multi-repo)
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(
				componentRepoNameForGeneralIntegration, multiComponentDefaultBranch, mergeMultiResultSha, multiComponentPRBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			var lastCreatedFileSha string

			for _, component := range componentNames {
				fileToCreatePath := fmt.Sprintf("%s/sample-file-for-%s.txt", component, component)
				createdFile, err := f.AsKubeAdmin.CommonController.Github.CreateFile(
					componentRepoNameForGeneralIntegration, fileToCreatePath, "Sometimes I drink water to surprise my liver", multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file: %s", fileToCreatePath))

				if createdFile.SHA != nil { // Prevents panic if SHA is nil
					lastCreatedFileSha = *createdFile.SHA
				}
			}

			pr, err := f.AsKubeAdmin.CommonController.Github.CreatePullRequest(
				componentRepoNameForGeneralIntegration, "Multirepo component PR", "sample pr body",
				multiComponentPRBranchName, multiComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			GinkgoWriter.Printf("PR #%d got created with sha %s\n", pr.GetNumber(), lastCreatedFileSha)
		})

		It("waits for the last components' builds to finish", func() {
			for _, component := range componentNames {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineToBeFinished(
					testNamespace, applicationName, component)).To(Succeed())
			}
		})

		It("gets all group snapshots and checks if pr-group annotation contains all components", func() {
			Eventually(func() error {
				groupSnapshots, err = f.AsKubeAdmin.HasController.GetAllGroupSnapshotsForApplication(applicationName, testNamespace)
				if groupSnapshots == nil {
					GinkgoWriter.Println("No group snapshot exists at the moment: %v", err)
					return err
				}
				if err != nil {
					GinkgoWriter.Println("Failed to get all group snapshots: %v", err)
					return err
				}
				return nil
			}, time.Minute*20, constants.PipelineRunPollingInterval).Should(Succeed(), "timeout while waiting for group snapshot")

			annotation := groupSnapshots.Items[0].GetAnnotations()
			if annotation, ok := annotation[testGroupSnapshotAnnotation]; ok {
				for _, component := range componentNames {
					Expect(annotation).To(ContainSubstring(component))
				}
			}
		})

		It("makes sure that the group snapshot contains the last build PipelineRun for each component", func() {
			for _, component := range componentNames {
				pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(
					component, applicationName, testNamespace, false, "")
				Expect(err).ShouldNot(HaveOccurred())

				annotation := groupSnapshots.Items[0].GetAnnotations()
				if annotation, ok := annotation[testGroupSnapshotAnnotation]; ok {
					Expect(annotation).To(ContainSubstring(pipelineRun.Name))
				}
			}
		})
	})
})
