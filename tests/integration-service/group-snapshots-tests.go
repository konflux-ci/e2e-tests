package integration

import (
	"fmt"
	"os"
	"time"
	"strings"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-github/v44/github"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/clients/integration"
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

	var prNumber int
	var prHeadSha, mergeResultSha, mergeMultiResultSha, secondFileSha string
	var pacBranchNames []string
	var componentsList []*appstudioApi.Component
	var snapshot *appstudioApi.Snapshot
	var componentA *appstudioApi.Component
	var componentB *appstudioApi.Component
	var componentC *appstudioApi.Component
	var groupSnapshots *appstudioApi.SnapshotList
	var componentSnapshots *[]appstudioApi.Snapshot
	var groupSnapshot *appstudioApi.Snapshot
	var mergeResult *github.PullRequestMergeResult
	var pipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass, invalidIntegrationTestScenario *integrationv1beta2.IntegrationTestScenario
	var applicationName, testNamespace, multiComponentBaseBranchName, multiComponentPRBranchName string

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
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			applicationName = createApp(*f, testNamespace)

			// The base branch or a ToBranch where all multi-component definitions will live
			multiComponentBaseBranchName = fmt.Sprintf("love-triangle-%s", util.GenerateRandomString(6))
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(multiComponentRepoNameForGroupSnapshot, multiComponentDefaultBranch, multiComponentGitRevision, multiComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			// The base branch or ToBranch where different repo component definition will live
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(componentRepoNameForGroupIntegration, multiComponentDefaultBranch, multiRepoComponentGitRevision, multiComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			//Branch for creating pull request
			multiComponentPRBranchName = fmt.Sprintf("%s-%s", "pr-branch", util.GenerateRandomString(6))

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPassPipelinerun, "pipelinerun", []string{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentA.Name, snapshot)
			}

			// Delete new branches created by PaC and a testing branch used as a component's base branch
			for _, pacBranchName := range pacBranchNames {
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, pacBranchName)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
				}
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForGroupIntegration, pacBranchName)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
				}
			}

			// Delete the created base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, multiComponentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
			// Delete the created base branch for multi-repo
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForGroupIntegration, multiComponentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}

			// Delete the created pr branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshot, multiComponentPRBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}

			// Delete the created pr branch for multi-repo
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepoNameForGroupIntegration, multiComponentPRBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
		})

		/*  /\
		   /  \
		  / /\ \
		 / ____ \
		/_/    \_\ */
		When("we start creation of a new Component A", func() {
			It("creates the Component A successfully", func() {
				componentA = createComponentWithCustomBranch(*f, testNamespace, applicationName, multiComponentContextDirs[0]+"-"+util.GenerateRandomString(6), multiComponentGitSourceURLForGroupSnapshotA, multiComponentBaseBranchName, multiComponentContextDirs[0])

				// Record the PaC branch names for cleanup
				pacBranchName := constants.PaCPullRequestBranchPrefix + componentA.Name
				pacBranchNames = append(pacBranchNames, pacBranchName)
			})

			It(fmt.Sprintf("triggers a Build PipelineRun for componentA %s", multiComponentContextDirs[0]), func() {
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentA.Name, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the componentA %s/%s\n", testNamespace, componentA.Name)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("build pipelinerunA %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the componentA %s/%s", testNamespace, componentA.Name))
			})

			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})

			It("should lead to build PipelineRunA finishing successfully", func() {
				Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(componentA, "", "", "",
					f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
			})

			It(fmt.Sprintf("should lead to a PaC PR creation for componentA %s", multiComponentContextDirs[0]), func() {
				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(multiComponentRepoNameForGroupSnapshot)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == pacBranchNames[0] {
							prNumber = pr.GetNumber()
							prHeadSha = pr.Head.GetSHA()
							return true
						}
					}
					return false
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchNames[0], multiComponentRepoNameForGroupSnapshot))

				// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentA.Name, applicationName, testNamespace, prHeadSha)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("eventually leads to the build PipelineRun'sA status reported at Checks tab", func() {
				expectedCheckRunName := fmt.Sprintf("%s-%s", componentA.Name, "on-pull-request")
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, multiComponentRepoNameForGroupSnapshot, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
			})
		})

		When("the Build PLRA is finished successfully", func() {
			It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", pipelineRun.Name, componentA.Name, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should find the related Integration PipelineRuns", func() {
				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))
			})

			It("integration pipeline should end up with success", func() {
				Eventually(func() error {
					integrationPipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentA.Name, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Integraiton PipelineRun has not been created yet for the componentA %s/%s\n", testNamespace, componentA.Name)
						return err
					}
					if !integrationPipelineRun.HasStarted() {
						return fmt.Errorf("integration pipelinerun %s in namespace %s hasn't started yet", integrationPipelineRun.GetName(), integrationPipelineRun.GetNamespace())
					}
					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the componentA %s/%s", testNamespace, componentA.Name))
			})
		})

		When("the Snapshot testing is completed successfully", func() {
			It("should merge the init PaC PR successfully", func() {
				Eventually(func() error {
					mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(multiComponentRepoNameForGroupSnapshot, prNumber)
					return err
				}, shortTimeout, constants.PipelineRunPollingInterval).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, multiComponentRepoNameForGroupSnapshot))

				mergeResultSha = mergeResult.GetSHA()
				GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)
			})

		})

		/*____
		|  _ \
		| |_) |
		|  _ <
		| |_) |
		|____/ */
		When("we start creation of a new Component B", func() {
			It("creates the Component B successfully", func() {
				componentB = createComponentWithCustomBranch(*f, testNamespace, applicationName, multiComponentContextDirs[1]+"-"+util.GenerateRandomString(6), multiComponentGitSourceURLForGroupSnapshotB, multiComponentBaseBranchName, multiComponentContextDirs[1])

				// Recording the PaC branch names so they can cleaned in the AfterAll block
				pacBranchName := constants.PaCPullRequestBranchPrefix + componentB.Name
				pacBranchNames = append(pacBranchNames, pacBranchName)
			})

			It(fmt.Sprintf("triggers a Build PipelineRun for component %s", multiComponentContextDirs[1]), func() {
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentB.Name, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the componentB %s/%s\n", testNamespace, componentB.Name)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("build pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the componentB %s/%s", testNamespace, componentB.Name))
			})

			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})

			It("should lead to build PipelineRun finishing successfully", func() {
				Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(componentB, "", "", "",
					f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
			})

			It(fmt.Sprintf("should lead to a PaC PR creation for component %s", multiComponentContextDirs[1]), func() {
				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(multiComponentRepoNameForGroupSnapshot)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == pacBranchNames[1] {
							prNumber = pr.GetNumber()
							prHeadSha = pr.Head.GetSHA()
							return true
						}
					}
					return false
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchNames[1], multiComponentRepoNameForGroupSnapshot))

				// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentB.Name, applicationName, testNamespace, prHeadSha)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("eventually leads to the build PipelineRun's status reported at Checks tab", func() {
				expectedCheckRunName := fmt.Sprintf("%s-%s", componentB.Name, "on-pull-request")
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, multiComponentRepoNameForGroupSnapshot, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
			})
		})

		When("the Build PLR is finished successfully", func() {
			It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", pipelineRun.Name, componentB.Name, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should find the related Integration PipelineRuns", func() {
				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))
			})

			It("integration pipeline should end up with success", func() {
				Eventually(func() error {
					integrationPipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentB.Name, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Integraiton PipelineRun has not been created yet for the componentB %s/%s\n", testNamespace, componentB.Name)
						return err
					}
					if !integrationPipelineRun.HasStarted() {
						return fmt.Errorf("integration pipelinerun %s in namespace %s hasn't started yet", integrationPipelineRun.GetName(), integrationPipelineRun.GetNamespace())
					}
					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the componentB %s/%s", testNamespace, componentB.Name))
			})
		})

		When("the Snapshot testing is completed successfully", func() {
			It("should merge the init PaC PR successfully", func() {
				Eventually(func() error {
					mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(multiComponentRepoNameForGroupSnapshot, prNumber)
					return err
				}, shortTimeout, constants.PipelineRunPollingInterval).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, multiComponentRepoNameForGroupSnapshot))

				mergeResultSha = mergeResult.GetSHA()
				GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)
			})

		})
		/*    _____
		//   / ____|
		//  | |
		//  | |
		//  | |____
		//   \_____|*/

		When("we start creation of a new Component C", func() {
			It("creates the Component C successfully", func() {
				componentC = createComponentWithCustomBranch(*f, testNamespace, applicationName, componentRepoNameForGroupIntegration+"-"+util.GenerateRandomString(6), componentGitSourceURLForGroupIntegration, multiComponentBaseBranchName, "")

				// Recording the PaC branch names so they can cleaned in the AfterAll block
				pacBranchName := constants.PaCPullRequestBranchPrefix + componentC.Name
				pacBranchNames = append(pacBranchNames, pacBranchName)
			})

			It(fmt.Sprintf("triggers a Build PipelineRun for componentC %s", componentRepoNameForGroupIntegration), func() {
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentC.Name, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the componentC %s/%s\n", testNamespace, componentC.Name)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("build pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the componentC %s/%s", testNamespace, componentC.Name))
			})

			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})

			It("should lead to build PipelineRun finishing successfully", func() {
				Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(componentC, "", "", "",
					f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
			})

			It(fmt.Sprintf("should lead to a PaC PR creation for componentC %s", componentRepoNameForGroupIntegration), func() {
				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(componentRepoNameForGroupIntegration)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == pacBranchNames[2] {
							prNumber = pr.GetNumber()
							prHeadSha = pr.Head.GetSHA()
							return true
						}
					}
					return false
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchNames[2], componentRepoNameForGroupIntegration))

				// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentC.Name, applicationName, testNamespace, prHeadSha)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("eventually leads to the build PipelineRun's status reported at Checks tab", func() {
				expectedCheckRunName := fmt.Sprintf("%s-%s", componentC.Name, "on-pull-request")
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, componentRepoNameForGroupIntegration, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
			})
		})

		When("the Build PLR is finished successfully", func() {
			It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", pipelineRun.Name, componentC.Name, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should find the related Integration PipelineRuns", func() {
				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))
			})

			It("integration pipeline should end up with success", func() {
				Eventually(func() error {
					integrationPipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentC.Name, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Integraiton PipelineRun has not been created yet for the componentC %s/%s\n", testNamespace, componentC.Name)
						return err
					}
					if !integrationPipelineRun.HasStarted() {
						return fmt.Errorf("integration pipelinerun %s in namespace %s hasn't started yet", integrationPipelineRun.GetName(), integrationPipelineRun.GetNamespace())
					}
					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the componentC %s/%s", testNamespace, componentC.Name))
			})
		})

		When("the Snapshot testing is completed successfully", func() {
			It("should merge the init PaC PR successfully", func() {
				Eventually(func() error {
					mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(componentRepoNameForGroupIntegration, prNumber)
					return err
				}, shortTimeout, constants.PipelineRunPollingInterval).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, componentRepoNameForGroupIntegration))

				mergeMultiResultSha = mergeResult.GetSHA()
				GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeMultiResultSha, prNumber)
			})

		})

		//  ______ _____ _   _
		// |  ____|_   _| \ | |
		// | |__    | | |  \| |
		// |  __|   | | | . ` |
		// | |     _| |_| |\  |
		// |_|    |_____|_| \_|

		When("both the init PaC PRs are merged", func() {
			It("should make change to the root folder", func() {

				//Create the ref, add the files and create the PR - monorepo
				err = f.AsKubeAdmin.CommonController.Github.CreateRef(multiComponentRepoNameForGroupSnapshot, multiComponentDefaultBranch, mergeResultSha, multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred())

				fileToCreatePathForCompA := fmt.Sprintf("%s/sample-file-for-componentA.txt", multiComponentContextDirs[0])
				_, err := f.AsKubeAdmin.CommonController.Github.CreateFile(multiComponentRepoNameForGroupSnapshot, fileToCreatePathForCompA, "Sleep is for weak, and I'm weak", multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file: %s", fileToCreatePathForCompA))

				fileToCreatePathForCompB := fmt.Sprintf("%s/sample-file-for-componentB.txt", multiComponentContextDirs[1])
				createdFileSha, err := f.AsKubeAdmin.CommonController.Github.CreateFile(multiComponentRepoNameForGroupSnapshot, fileToCreatePathForCompB, "Sometimes I drink water to surprise my liver", multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file: %s", fileToCreatePathForCompB))

				pr, err := f.AsKubeAdmin.CommonController.Github.CreatePullRequest(multiComponentRepoNameForGroupSnapshot, "SingleRepo multi-component PR", "sample pr body", multiComponentPRBranchName, multiComponentBaseBranchName)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("PR #%d got created with sha %s\n", pr.GetNumber(), createdFileSha.GetSHA())
			})
			It("should make change to the multiple-repo", func() {
				// Delete all the pipelineruns in the namespace before sending PR
				//Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())

				//Create the ref, add the files and create the PR - multirepo
				err = f.AsKubeAdmin.CommonController.Github.CreateRef(componentRepoNameForGroupIntegration, multiComponentDefaultBranch, mergeMultiResultSha, multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred())

				fileToCreatePathForCompC := fmt.Sprintf("%s/sample-file-for-componentC.txt", componentC.Name)
				createdFileSha, err := f.AsKubeAdmin.CommonController.Github.CreateFile(componentRepoNameForGroupIntegration, fileToCreatePathForCompC, "People say nothing is impossible, but I do nothing every day", multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file in multirepo: %s", fileToCreatePathForCompC))

				pr, err := f.AsKubeAdmin.CommonController.Github.CreatePullRequest(componentRepoNameForGroupIntegration, "Multirepo component PR", "sample pr body", multiComponentPRBranchName, multiComponentBaseBranchName)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("PR #%d got created with sha %s\n", pr.GetNumber(), createdFileSha.GetSHA())
			})
			It("wait for the last components build to finish", func() {
				componentsList = []*appstudioApi.Component{componentA, componentB, componentC}
				for _, component := range componentsList {
					Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(component, "", "", "",
						f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
				}
			})

			It("wait for all component snapshots to be created with proper PR group annotations", func() {
				Eventually(func() error {
					componentSnapshots, err := f.AsKubeAdmin.HasController.GetAllComponentSnapshotsForApplication(applicationName, testNamespace)
					if err != nil {
						GinkgoWriter.Printf("Failed to get component snapshots: %v\n", err)
						return err
					}

					// We expect at least 3 component snapshots (for components A, B, and C)
					if len(componentSnapshots.Items) < 3 {
						GinkgoWriter.Printf("Expected at least 3 component snapshots, got %d\n", len(componentSnapshots.Items))
						return fmt.Errorf("insufficient component snapshots: expected at least 3, got %d", len(componentSnapshots.Items))
					}

					// Check that component snapshots have PR group annotations
					snapshotsWithPRGroup := 0
					for _, snapshot := range componentSnapshots.Items {
						annotations := snapshot.GetAnnotations()
						if prGroup, exists := annotations[groupSnapshotAnnotation]; exists && prGroup != "" {
							snapshotsWithPRGroup++
							GinkgoWriter.Printf("Component snapshot %s has PR group annotation: %s\n", snapshot.Name, prGroup)
						} else {
							GinkgoWriter.Printf("Component snapshot %s is missing PR group annotation\n", snapshot.Name)
						}
					}

					if snapshotsWithPRGroup < 3 {
						return fmt.Errorf("expected at least 3 component snapshots with PR group annotations, got %d", snapshotsWithPRGroup)
					}

					GinkgoWriter.Printf("All component snapshots are ready with PR group annotations\n")
					return nil
				}, time.Minute*10, 15*time.Second).Should(Succeed(), "Timeout while waiting for component snapshots with PR group annotations")
			})

			It("get all group snapshots and check if pr-group annotation contains all components", func() {
				// Wait for group snapshots with enhanced debugging and retry logic
				Eventually(func() error {
					GinkgoWriter.Printf("Attempting to find group snapshots for application %s in namespace %s\n", applicationName, testNamespace)

					// First, let's check the current state of component snapshots
					compSnapshots, err := f.AsKubeAdmin.HasController.GetAllComponentSnapshotsForApplication(applicationName, testNamespace)
					if err != nil {
						GinkgoWriter.Printf("Failed to get component snapshots: %v\n", err)
						return err
					}

					GinkgoWriter.Printf("Found %d component snapshots:\n", len(compSnapshots.Items))
					prGroupsFound := make(map[string]int)
					for _, snapshot := range compSnapshots.Items {
						annotations := snapshot.GetAnnotations()
						prGroup := annotations[groupSnapshotAnnotation]

						if prGroup != "" {
							prGroupsFound[prGroup]++
						}
					}

					// Log PR groups summary
					GinkgoWriter.Printf("PR Groups found: %v\n", prGroupsFound)

					// Now attempt to get group snapshots
					groupSnapshots, err = f.AsKubeAdmin.HasController.GetAllGroupSnapshotsForApplication(applicationName, testNamespace)
					if err != nil {
						// Check if it's a "not found" error vs other errors
						if err.Error() == fmt.Sprintf("no snapshot found for application %s", applicationName) {
							GinkgoWriter.Printf("No group snapshots found yet. Component snapshots may not have been processed by integration service controller yet.\n")
							return fmt.Errorf("group snapshots not yet created from component snapshots")
						}
						GinkgoWriter.Printf("Error getting group snapshots: %v\n", err)
						return err
					}

					if len(groupSnapshots.Items) == 0 {
						GinkgoWriter.Printf("No group snapshots exist yet. Integration service controller may still be processing component snapshots.\n")
						return fmt.Errorf("no group snapshots found - controller may still be processing")
					}

					GinkgoWriter.Printf("Found %d group snapshots!\n", len(groupSnapshots.Items))
					for i, snapshot := range groupSnapshots.Items {
						annotations := snapshot.GetAnnotations()
						labels := snapshot.GetLabels()
						GinkgoWriter.Printf("  Group Snapshot %d: %s (type: %s)\n", i, snapshot.Name, labels["test.appstudio.openshift.io/type"])
						GinkgoWriter.Printf("    Group Test Info: %s\n", annotations[testGroupSnapshotAnnotation])
					}

					return nil
				}, time.Minute*30, 30*time.Second).Should(Succeed(), "Timeout while waiting for group snapshot creation")

				// Validate the group snapshot annotations
				Expect(groupSnapshots.Items).ToNot(BeEmpty(), "Expected at least one group snapshot")

				annotation := groupSnapshots.Items[0].GetAnnotations()
				groupTestInfo, exists := annotation[testGroupSnapshotAnnotation]
				Expect(exists).To(BeTrue(), "Group snapshot should have test.appstudio.openshift.io/group-test-info annotation")
				Expect(groupTestInfo).ToNot(BeEmpty(), "Group test info annotation should not be empty")

				// Check that the annotation contains all expected components
				GinkgoWriter.Printf("Validating group test info annotation: %s\n", groupTestInfo)

				// konflux-test (multirepo component)
				Expect(groupTestInfo).To(ContainSubstring(componentRepoNameForGroupIntegration),
					"Group test info should contain multirepo component name")

				// go-component (monorepo component A)
				Expect(groupTestInfo).To(ContainSubstring(multiComponentContextDirs[0]),
					"Group test info should contain first monorepo component context dir")

				// python-component (monorepo component B)
				Expect(groupTestInfo).To(ContainSubstring(multiComponentContextDirs[1]),
					"Group test info should contain second monorepo component context dir")

				GinkgoWriter.Printf("Group snapshot validation completed successfully\n")
			})
			It("make sure that group snapshot contains last build pipelinerun for each component", func() {
				for _, component := range componentsList {
					pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(component.Name, applicationName, testNamespace, false, "")
					Expect(err).ShouldNot(HaveOccurred())
					annotation := groupSnapshots.Items[0].GetAnnotations()
					if annotation, ok := annotation[testGroupSnapshotAnnotation]; ok {
						Expect(annotation).To(ContainSubstring(pipelineRun.Name))
					}
				}
			})
		})

		When("Older snapshot and integration pipelinerun should be cancelled once new snapshot is created", func() {
			It("make change to the multiple-repo to trigger a new cycle of testing", func() {
				newFile, err := f.AsKubeAdmin.HasController.Github.CreateFile(multiComponentRepoNameForGroupSnapshot, util.GenerateRandomString(5), "test", multiComponentPRBranchName)
				secondFileSha = newFile.GetSHA()
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file in multirepo: %s", secondFileSha))
			})

			It("wait for the components A and B build to finish", func() {
				GinkgoWriter.Printf("Waiting for build pipelineRun to be created for app %s/%s, sha: %s\n", testNamespace, applicationName, secondFileSha)
				componentsList = []*appstudioApi.Component{componentA, componentB}
				for _, component := range componentsList {
					Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(component, "", "", "",
						f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
				}
			})

			It("get all component snapshots for component A and check if older snapshot has been cancelled", func() {
				// get all component snapshots for component A
				Eventually(func() error {
					componentSnapshots, err = f.AsKubeAdmin.HasController.GetAllComponentSnapshotsForApplicationAndComponent(applicationName, testNamespace, componentA.Name)

					if componentSnapshots == nil {
						GinkgoWriter.Println("No component snapshot exists at the moment: %v", err)
						return err
					}
					if err != nil {
						GinkgoWriter.Println("failed to get all component snapshots: %v", err)
						return err
					}
					if len(*componentSnapshots) < 2 {
						return fmt.Errorf("The length of component snapshot is %d, less than expected 2", len(*componentSnapshots))
					}
					isCancelled, err := f.AsKubeAdmin.IntegrationController.IsOlderSnapshotAndIntegrationPlrCancelled(*componentSnapshots, integrationTestScenarioPass.Name)
					if err != nil {
						return err
					}
					if !isCancelled {
						return fmt.Errorf("older component snasphot/integration test has not been cancelled")
					}
					return nil
				}, superLongTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), "timeout while waiting for component snapshot and integration pipelinerun to be cancelled")
			})

			It("get all group snapshots and check if older group snapshot is cancelled", func() {
				// get all group snapshots
				Eventually(func() error {
					groupSnapshots, err = f.AsKubeAdmin.HasController.GetAllGroupSnapshotsForApplication(applicationName, testNamespace)

					if groupSnapshots == nil {
						GinkgoWriter.Println("No group snapshot exists at the moment: %v", err)
						return err
					}
					if err != nil {
						GinkgoWriter.Println("failed to get all group snapshots: %v", err)
						return err
					}
					if len(groupSnapshots.Items) < 2 {
						return fmt.Errorf("The length of group snapshot is %d, less than expected 2", len(groupSnapshots.Items))
					}
					isCancelled, err := f.AsKubeAdmin.IntegrationController.IsOlderSnapshotAndIntegrationPlrCancelled(groupSnapshots.Items, integrationTestScenarioPass.Name)
					if err != nil {
						return err
					}
					if !isCancelled {
						return fmt.Errorf("older group snasphot/integration test has not been cancelled")
					}
					return nil
				}, superLongTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), "timeout while waiting for group snapshot and integration pipelinerun to be cancelled")
			})

		})

		When("ResolutionRequest is deleted after pipeline completes", func() {
			It("verifies that ResolutionRequest is deleted after pipeline resolution", func() {
				Eventually(func() error {
					relatedResolutionRequests, err := f.AsKubeDeveloper.IntegrationController.GetRelatedResolutionRequests(testNamespace, integrationTestScenarioPass)
					if err != nil {
						if strings.Contains(err.Error(), "ResolutionRequest CRD not available") {
							return nil
						}
						return fmt.Errorf("failed to get related ResolutionRequests: %v", err)
					}

					if len(relatedResolutionRequests) > 0 {
						names := f.AsKubeDeveloper.IntegrationController.GetResolutionRequestNames(relatedResolutionRequests)
						return fmt.Errorf("found %d ResolutionRequest(s) still present in namespace %s for scenario %s: %v",
							len(relatedResolutionRequests), testNamespace, integrationTestScenarioPass.Name, names)
					}

					return nil
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), "ResolutionRequest objects should be cleaned up after pipeline resolution is complete")
			})

			It("verifies that no orphaned ResolutionRequests remain in namespace after test completion", func() {
				// Check for any ResolutionRequests that might have been left behind
				relatedResolutionRequests, err := f.AsKubeDeveloper.IntegrationController.GetRelatedResolutionRequests(testNamespace, integrationTestScenarioPass)
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
		})


		When("IntegrationTestScenario reference to task as pipelinerun resolution", func() {
			BeforeAll(func() {
				invalidIntegrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPass, "pipelinerun", []string{"application"})
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("trigger pipelinerun for invalid integrationTestScenario by annotating snapshot and verify failing to create integration pipelinerun", func() {
				groupSnapshot = &groupSnapshots.Items[0]
				Eventually(func() error {
					err = f.AsKubeAdmin.IntegrationController.AddIntegrationTestRerunLabel(groupSnapshot, invalidIntegrationTestScenario.Name)
					if err != nil {
						return fmt.Errorf("failed to set annotation %s to group snapshot %s/%s", integration.SnapshotIntegrationTestRun, groupSnapshot.Namespace, groupSnapshot.Name)
					}

					groupSnapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(groupSnapshot.Name, "", "", testNamespace)
					if err != nil {
						return fmt.Errorf("failing to get snapshot %s/%s", groupSnapshot.Namespace, groupSnapshot.Name)
					}

					statusDetail, err := f.AsKubeDeveloper.IntegrationController.GetIntegrationTestStatusDetailFromSnapshot(groupSnapshot, invalidIntegrationTestScenario.Name)
					if err != nil {
						return fmt.Errorf("failing to get snapshot integration test status detail %s/%s", groupSnapshot.Namespace, groupSnapshot.Name)
					}

					if !strings.Contains(statusDetail.Details, "denied the request: validation failed: expected exactly one, got neither: spec.pipelineRef, spec.pipelineSpec.") {
						return fmt.Errorf("failing to find the integration test status detail %s/%s for invalid resolution", groupSnapshot.Namespace, groupSnapshot.Name)
					}

					return nil
				}, longTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), "timeout while waiting for group snapshot and failing integration pipelinerun with invalid resolution")

			})
		})
	})
})
