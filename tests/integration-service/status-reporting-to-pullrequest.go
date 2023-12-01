package integration

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-github/v44/github"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
)

var _ = framework.IntegrationServiceSuiteDescribe("Status Reporting of Integration tests", Label("integration-service", "HACBS", "status-reporting"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var prNumber int
	var timeout, interval time.Duration
	var osConsoleHost, prHeadSha string
	var snapshot *appstudioApi.Snapshot
	var component *appstudioApi.Component
	var pipelineRun, testPipelinerun *v1beta1.PipelineRun
	var integrationTestScenarioPass, integrationTestScenarioFail *integrationv1beta1.IntegrationTestScenario
	var applicationName, componentName, componentBaseBranchName, pacBranchName, testNamespace string

	AfterEach(framework.ReportFailure(&f))

	Describe("with status reporting of Integration tests in CheckRuns", Ordered, func() {
		BeforeAll(func() {
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("stat-rep"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			consoleRoute, err := f.AsKubeAdmin.CommonController.GetOpenshiftRoute("console", "openshift-console")
			Expect(err).ShouldNot(HaveOccurred())
			osConsoleHost = consoleRoute.Spec.Host

			if utils.IsPrivateHostname(osConsoleHost) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			applicationName = createApp(*f, testNamespace)

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario_beta1(applicationName, testNamespace, gitURL, revision, pathInRepoForReportingPass)
			Expect(err).ShouldNot(HaveOccurred())

			integrationTestScenarioFail, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario_beta1(applicationName, testNamespace, gitURL, revision, pathInRepoForReportingFail)
			Expect(err).ShouldNot(HaveOccurred())

			componentName = fmt.Sprintf("%s-%s", "test-component-pac", util.GenerateRandomString(6))
			pacBranchName = constants.PaCPullRequestBranchPrefix + componentName
			componentBaseBranchName = fmt.Sprintf("base-%s", util.GenerateRandomString(6))

			err = f.AsKubeAdmin.CommonController.Github.CreateRef(componentRepoNameForStatusReporting, componentDefaultBranch, componentRevision, componentBaseBranchName)
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
			BeforeAll(func() {
				componentObj := appstudioApi.ComponentSpec{
					ComponentName: componentName,
					Application:   applicationName,
					Source: appstudioApi.ComponentSource{
						ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
							GitSource: &appstudioApi.GitSource{
								URL:      componentGitSourceURLForStatusReporting,
								Revision: componentBaseBranchName,
							},
						},
					},
				}
				// Create a component with Git Source URL, a specified git branch
				component, err = f.AsKubeAdmin.HasController.CreateComponent(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo))
				Expect(err).ShouldNot(HaveOccurred())
			})
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
			It("should lead to a PaC init PR creation", func() {
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
			})
			It("the build PipelineRun should eventually finish successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component,
					"", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true})).To(Succeed())
			})
			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})
			It("eventually leads to the build PipelineRun's status reported at Checks tab", func() {
				validateCheckRun(*f, componentName, checkrunConclusionSuccess, componentRepoNameForStatusReporting, prHeadSha, prNumber)
			})
		})

		When("the PaC build pipelineRun run succeeded", func() {
			It("checks if the BuildPipelineRun is signed", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToBeSigned(testNamespace, applicationName, componentName)).To(Succeed())
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
				validateCheckRun(*f, integrationTestScenarioPass.Name, checkrunConclusionSuccess, componentRepoNameForStatusReporting, prHeadSha, prNumber)
			})
			It("eventually leads to the status reported at Checks tab for the failed Integration PipelineRun", func() {
				validateCheckRun(*f, integrationTestScenarioFail.Name, checkrunConclusionFailure, componentRepoNameForStatusReporting, prHeadSha, prNumber)
			})
		})
	})
})

// validateCheckRun fetches a specific CheckRun within a given repo
// by matching the CheckRun's name with the given checkRunName, and
// then validates that it got completed and its conclusion is as expected
func validateCheckRun(f framework.Framework, checkRunName, conclusion, repoName, prHeadSha string, prNumber int) {
	var checkRun *github.CheckRun
	var timeout, interval time.Duration
	var err error

	timeout = time.Minute * 10
	interval = time.Second * 2

	Eventually(func() *github.CheckRun {
		checkRuns, err := f.AsKubeAdmin.CommonController.Github.ListCheckRuns(repoName, prHeadSha)
		Expect(err).ShouldNot(HaveOccurred())
		for _, cr := range checkRuns {
			if strings.Contains(cr.GetName(), checkRunName) {
				checkRun = cr
				return cr
			}
		}
		return nil
	}, timeout, interval).ShouldNot(BeNil(), fmt.Sprintf("timed out when waiting for the PaC CheckRun, with `Name` field containing the substring %s, to appear in the PR #%d of the Component repo %s", checkRunName, prNumber, repoName))

	Eventually(func() string {
		checkRun, err = f.AsKubeAdmin.CommonController.Github.GetCheckRun(repoName, checkRun.GetID())
		Expect(err).ShouldNot(HaveOccurred())
		return checkRun.GetStatus()
	}, timeout, interval).Should(Equal(checkrunStatusCompleted), fmt.Sprintf("timed out when waiting for the PaC Check suite status to be 'completed' in the Component repo %s in PR #%d", repoName, prNumber))
	Expect(checkRun.GetConclusion()).To(Equal(conclusion), fmt.Sprintf("the PR %d in %s repo doesn't contain the expected conclusion (%s) of the CheckRun", prNumber, repoName, conclusion))
}
