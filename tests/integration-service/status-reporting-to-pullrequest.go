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

var _ = framework.IntegrationServiceSuiteDescribe("Status Reporting of Integration tests", Label("integration-service", "github-status-reporting"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var prNumber int
	var timeout, interval time.Duration
	var prHeadSha string
	var snapshot *appstudioApi.Snapshot
	var component *appstudioApi.Component
	var pipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass, integrationTestScenarioFail *integrationv1beta2.IntegrationTestScenario
	var applicationName, componentName, componentBaseBranchName, pacBranchName, testNamespace string

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
			component, componentName, pacBranchName, componentBaseBranchName = createComponent(*f, testNamespace, applicationName, componentRepoNameForStatusReporting, componentGitSourceURLForStatusReporting)

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPass, []string{})
			Expect(err).ShouldNot(HaveOccurred())
			integrationTestScenarioFail, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoFail, []string{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentName)
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
			It("triggers a Build PipelineRun", func() {
				timeout = time.Second * 600
				interval = time.Second * 1
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("build pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the component %s/%s", testNamespace, componentName))
			})

			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})

			It("should lead to build PipelineRun finishing successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component,
					"", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, pipelineRun)).To(Succeed())
			})

			It("should have a related PaC init PR created", func() {
				timeout = time.Second * 300
				interval = time.Second * 1

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
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchName, componentRepoNameForStatusReporting))

				// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, prHeadSha)
				Expect(err).ShouldNot(HaveOccurred())
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
			})
		})

		When("Integration PipelineRuns are created", func() {
			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioFail, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})
		})

		When("Integration PipelineRuns completes successfully", func() {
			It("should lead to Snapshot CR being marked as failed", FlakeAttempts(3), func() {
				// Snapshot marked as Failed because one of its Integration test failed (as expected)
				Eventually(func() bool {
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
		})
	})
})
