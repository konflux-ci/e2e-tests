package integration

import (
	"fmt"
	"os"
	"time"

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

	var prNumber int
	var timeout, interval time.Duration
	var prHeadSha string
	var snapshot *appstudioApi.Snapshot
	var componentA *appstudioApi.Component
	var componentB *appstudioApi.Component
	var pipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass *integrationv1beta2.IntegrationTestScenario
	var applicationName, componentAName, componentBName, toBranchName, fromBranchNameA, fromBranchNameB, testNamespace, toBranch string

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
			toBranch := "test-group-branch"
			contextDirA := "go-component/docker"
			contextDirB := "python-component/docker"
			applicationName = createApp(*f, testNamespace)
			componentA, componentAName, fromBranchNameA, toBranchName = createComponentWithCustomBranch(*f, testNamespace, applicationName, multiComponentRepoNameForGroupSnapshotA, multiComponentGitSourceURLForGroupSnapshotA, toBranch, contextDirA)
			componentB, componentBName, fromBranchNameB, toBranchName = createComponentWithCustomBranch(*f, testNamespace, applicationName, multiComponentRepoNameForGroupSnapshotB, multiComponentGitSourceURLForGroupSnapshotB, toBranch, contextDirB)

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPass, []string{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentAName)
				cleanup(*f, testNamespace, applicationName, componentBName)
			}

			// Delete new branches created by PaC and a testing branch used as a component's A base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshotA, fromBranchNameA)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshotA, toBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}

			// Delete new branches created by PaC and a testing branch used as a component's B base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshotB, fromBranchNameB)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentRepoNameForGroupSnapshotB, toBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring(referenceDoesntExist))
			}
		})
		/*  /\
		   /  \
		  / /\ \
		 / ____ \
		/_/    \_\ */
		When("a new Component A with specified custom branch(same as component B) is created", Label("custom-branch"), func() {
			It("triggers a Build PipelineRun", func() {
				timeout = time.Second * 600
				interval = time.Second * 1
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentAName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the componentA %s/%s\n", testNamespace, componentAName)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("build pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the componentA %s/%s", testNamespace, componentAName))
			})

			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})

			It("should lead to build PipelineRun finishing successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(componentA,
					"", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
			})

			It("should have a related PaC init PR created", func() {
				timeout = time.Second * 300
				interval = time.Second * 1

				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(multiComponentRepoNameForGroupSnapshotA)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == fromBranchNameA {
							prNumber = pr.GetNumber()
							prHeadSha = pr.Head.GetSHA()
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", fromBranchNameA, multiComponentRepoNameForGroupSnapshotA))

				// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentAName, applicationName, testNamespace, prHeadSha)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("eventually leads to the build PipelineRun's status reported at Checks tab", func() {
				expectedCheckRunName := fmt.Sprintf("%s-%s", componentAName, "on-pull-request")
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, multiComponentRepoNameForGroupSnapshotA, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
			})
		})

		When("the Snapshot was created", func() {
			It("should find both the related Integration PipelineRuns", func() {
				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))
			})
		})

		/*____
		|  _ \
		| |_) |
		|  _ <
		| |_) |
		|____/ */
		When("a new Component B with specified custom branch(same as component A) is created", Label("custom-branch"), func() {
			It("triggers a Build PipelineRun", func() {
				timeout = time.Second * 600
				interval = time.Second * 1
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentBName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the componentB %s/%s\n", testNamespace, componentBName)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("build pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the componentB %s/%s", testNamespace, componentBName))
			})

			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})

			It("should lead to build PipelineRun finishing successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(componentB,
					"", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
			})

			// It("should have a related PaC init PR created", func() {
			// 	timeout = time.Second * 300
			// 	interval = time.Second * 1

			// 	Eventually(func() bool {
			// 		prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(multiComponentRepoNameForGroupSnapshotB)
			// 		Expect(err).ShouldNot(HaveOccurred())

			// 		for _, pr := range prs {
			// 			if pr.Head.GetRef() == pacBranchNameB {
			// 				prNumber = pr.GetNumber()
			// 				prHeadSha = pr.Head.GetSHA()
			// 				return true
			// 			}
			// 		}
			// 		return false
			// 	}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchNameB, multiComponentRepoNameForGroupSnapshotB))

			// 	// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable
			// 	pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentAName, applicationName, testNamespace, prHeadSha)
			// 	Expect(err).ShouldNot(HaveOccurred())
			// })

			// It("eventually leads to the build PipelineRun's status reported at Checks tab", func() {
			// 	expectedCheckRunName := fmt.Sprintf("%s-%s", componentAName, "on-pull-request")
			// 	Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, multiComponentRepoNameForGroupSnapshotB, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
			// })
		})

		When("the PaC build pipelineRun run succeeded", func() {
			It("checks if the BuildPipelineRun have the annotation of chains signed", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentAName, chainsSignedAnnotation)).To(Succeed())
			})

			It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentAName, testNamespace)
				Expect(err).ToNot(HaveOccurred())
			})

			It("checks if the Build PipelineRun got annotated with Snapshot name", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentAName, snapshotAnnotation)).To(Succeed())
			})
		})

		When("the Snapshot was created", func() {
			It("should find both the related Integration PipelineRuns", func() {
				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))

			})
		})

		When("Integration PipelineRuns are created", func() {
			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})
		})

		When("Integration PipelineRuns completes successfully", func() {
			It("should lead to Snapshot CR being marked as success", FlakeAttempts(3), func() {
				// Snapshot marked as Success, this could be the place where we check for group snapshot
				Eventually(func() bool {
					snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", testNamespace)
					return err == nil && !f.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot)
				}, time.Minute*3, time.Second*5).Should(BeTrue(), fmt.Sprintf("Timed out waiting for Snapshot to be marked as success %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
			})
			It("snapshot should contain group-pr annnotation", func() {
				Expect(snapshot.Annotations[groupSnapshotAnnotation]).To(ContainSubstring(toBranch))
			})

			It("eventually leads to the status reported at Checks tab for the successful Integration PipelineRun", func() {
				Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(integrationTestScenarioPass.Name, componentRepoNameForStatusReporting, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
			})
		})
	})
})
