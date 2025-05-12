package integration

import (
	"fmt"
	"os"

	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	"github.com/xanzy/go-gitlab"

	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
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

var _ = framework.IntegrationServiceSuiteDescribe("Gitlab Status Reporting of Integration tests", Label("integration-service", "gitlab-status-reporting"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var mrID int
	// var mrNote *gitlab.Note
	var timeout, interval time.Duration
	var mrSha, projectID, gitlabToken string
	var snapshot *appstudioApi.Snapshot
	var component *appstudioApi.Component
	var buildPipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass, integrationTestScenarioFail *integrationv1beta2.IntegrationTestScenario
	var applicationName, componentName, componentBaseBranchName, pacBranchName, testNamespace string

	AfterEach(framework.ReportFailure(&f))

	Describe("Gitlab with status reporting of Integration tests in the assosiated merge request", Ordered, func() {
		BeforeAll(func() {
			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				Skip("Skipping this test due to configuration issue with Spray proxy")
			}

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("gitlab-rep"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			if utils.IsPrivateHostname(f.OpenshiftConsoleHost) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			applicationName = createApp(*f, testNamespace)

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPass, []string{})
			Expect(err).ShouldNot(HaveOccurred())

			integrationTestScenarioFail, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoFail, []string{})
			Expect(err).ShouldNot(HaveOccurred())

			componentName = fmt.Sprintf("%s-%s", "test-comp-pac-gitlab", util.GenerateRandomString(6))
			pacBranchName = constants.PaCPullRequestBranchPrefix + componentName
			componentBaseBranchName = fmt.Sprintf("base-gitlab-%s", util.GenerateRandomString(6))

			projectID = gitlabProjectIDForStatusReporting

			gitlabToken = utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, "")
			Expect(gitlabToken).ShouldNot(BeEmpty(), fmt.Sprintf("'%s' env var is not set", constants.GITLAB_BOT_TOKEN_ENV))

			err = f.AsKubeAdmin.CommonController.Gitlab.CreateGitlabNewBranch(projectID, componentBaseBranchName, componentRevision, componentDefaultBranch)
			Expect(err).ShouldNot(HaveOccurred())

			secretAnnotations := map[string]string{}

			err = build.CreateGitlabBuildSecret(f, "gitlab-build-secret", secretAnnotations, gitlabToken)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {

			// Cleanup test: close MR if opened, delete created branch, delete associated Webhooks
			Expect(f.AsKubeAdmin.CommonController.Gitlab.CloseMergeRequest(projectID, mrID)).NotTo(HaveOccurred())
			Expect(f.AsKubeAdmin.CommonController.Gitlab.DeleteBranch(projectID, componentBaseBranchName)).NotTo(HaveOccurred())
			Expect(f.AsKubeAdmin.CommonController.Gitlab.DeleteWebhooks(projectID, f.ClusterAppDomain)).NotTo(HaveOccurred())

			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.HasController.DeleteApplication(applicationName, testNamespace, false)).To(Succeed())
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).To(BeTrue())
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
								URL:           gitlabComponentGitSourceURLForStatusReporting,
								Revision:      componentBaseBranchName,
								DockerfileURL: "Dockerfile",
							},
						},
					},
				}
				// get the build pipeline bundle annotation
				buildPipelineAnnotation := build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)
				// Create a component with Git Source URL, a specified git branch
				component, err = f.AsKubeAdmin.HasController.CreateComponent(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("triggers a Build PipelineRun", func() {
				timeout = time.Second * 600
				interval = time.Second * 1
				Eventually(func() error {
					buildPipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
						return err
					}
					if !buildPipelineRun.HasStarted() {
						return fmt.Errorf("build pipelinerun %s/%s hasn't started yet", buildPipelineRun.GetNamespace(), buildPipelineRun.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the component %s/%s", testNamespace, componentName))
			})

			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(buildPipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})

			It("should lead to build PipelineRun finishing successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component,
					"", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, buildPipelineRun)).To(Succeed())
			})

			It("should have a related PaC init MR is created", func() {
				timeout = time.Second * 300
				interval = time.Second * 1

				Eventually(func() bool {
					mrs, err := f.AsKubeAdmin.CommonController.Gitlab.GetMergeRequests()
					Expect(err).ShouldNot(HaveOccurred())

					for _, mr := range mrs {
						if mr.SourceBranch == pacBranchName {
							mrID = mr.IID
							mrSha = mr.SHA
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC MR (branch name '%s') to be created in %s repository", pacBranchName, componentRepoNameForStatusReporting))

				// in case the first pipelineRun attempt has failed and was retried, we need to update the value of pipelineRun variable
				buildPipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, mrSha)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It(fmt.Sprintf("the PipelineRun should eventually finish successfully for component %s", componentName), func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "",
					f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, nil)).To(Succeed())
			})

			It("eventually leads to the build PipelineRun's status reported at MR notes", func() {
				expectedNote := fmt.Sprintf("**Pipelines as Code CI/%s-on-pull-request** has successfully validated your commit", componentName)
				f.AsKubeAdmin.HasController.GitLab.ValidateNoteInMergeRequestComment(projectID, expectedNote, mrID)
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
			It("should find the Integration Test Scenario PipelineRun", func() {
				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioPass.Name))

				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))

			})
		})

		When("Integration PipelineRun is created", func() {
			var mergeResult *gitlab.MergeRequest
			var mergeResultSha string
			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioFail, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})

			It("validates the Integration test scenario PipelineRun is reported to merge request CommitStatus, and it pass", func() {
				timeout = time.Second * 420
				interval = time.Second * 1

				Eventually(func() string {
					commitStatuses, _, err := f.AsKubeAdmin.CommonController.Gitlab.GetClient().Commits.GetCommitStatuses(projectID, mrSha, nil)
					Expect(err).ShouldNot(HaveOccurred())

					for _, commitStatus := range commitStatuses {
						commitStatusNames := strings.Split(commitStatus.Name, "/")
						if len(commitStatusNames) > 2 {
							if strings.Contains(integrationTestScenarioPass.Name, strings.TrimSpace(commitStatusNames[1])) {
								return commitStatus.Status
							}
						}
					}
					return ""
				}, timeout, interval).Should(Equal(integrationPipelineRunCommitStatusSuccess), fmt.Sprintf("timed out when waiting for expected commitStatus to be created for sha %s in %s repository", mrSha, componentRepoNameForStatusReporting))
			})

			It("eventually leads to the integration test PipelineRun's Pass status reported at MR notes", func() {
				expectedNote := fmt.Sprintf("Integration test for snapshot %s and scenario %s has passed", snapshot.Name, integrationTestScenarioPass.Name)
				f.AsKubeAdmin.HasController.GitLab.ValidateNoteInMergeRequestComment(projectID, expectedNote, mrID)
			})

			It("validates the Integration test scenario PipelineRun is reported to merge request CommitStatus, and it fails", func() {
				timeout = time.Second * 420
				interval = time.Second * 1

				Eventually(func() string {
					commitStatuses, _, err := f.AsKubeAdmin.CommonController.Gitlab.GetClient().Commits.GetCommitStatuses(projectID, mrSha, nil)
					Expect(err).ShouldNot(HaveOccurred())

					for _, commitStatus := range commitStatuses {
						commitStatusNames := strings.Split(commitStatus.Name, "/")
						if len(commitStatusNames) > 2 {
							if strings.Contains(integrationTestScenarioFail.Name, strings.TrimSpace(commitStatusNames[1])) {
								return commitStatus.Status
							}
						}
					}
					return ""
				}, timeout, interval).Should(Equal(integrationPipelineRunCommitStatusFail), fmt.Sprintf("timed out when waiting for expected commitStatus to be created for sha %s in %s repository", mrSha, componentRepoNameForStatusReporting))
			})

			It("eventually leads to the integration test PipelineRun's Fail status reported at MR notes", func() {
				expectedNote := fmt.Sprintf("Integration test for snapshot %s and scenario %s has failed", snapshot.Name, integrationTestScenarioFail.Name)
				f.AsKubeAdmin.HasController.GitLab.ValidateNoteInMergeRequestComment(projectID, expectedNote, mrID)
			})

			It("merging the PR should be successful", func() {
				Eventually(func() error {
					mergeResult, err = f.AsKubeAdmin.CommonController.Gitlab.AcceptMergeRequest(projectID, mrID)
					return err
				}, time.Minute).Should(BeNil(), fmt.Sprintf("error when merging PaC merge request ID #%d in ProjectID %s", mrID, projectID))

				mergeResultSha = mergeResult.SHA
				GinkgoWriter.Printf("merged result sha: %s for MR #%d\n", mergeResultSha, mrID)

			})
			It("leads to triggering on push PipelineRun", func() {
				timeout = time.Minute * 5
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
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, componentName))
			})

			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioFail, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})

			It("validates the Integration test scenario PipelineRun is reported to merge request CommitStatus, and it pass", func() {
				timeout = time.Second * 300
				interval = time.Second * 1

				Eventually(func() string {
					commitStatuses, _, err := f.AsKubeAdmin.CommonController.Gitlab.GetClient().Commits.GetCommitStatuses(projectID, mrSha, nil)
					Expect(err).ShouldNot(HaveOccurred())

					for _, commitStatus := range commitStatuses {
						commitStatusNames := strings.Split(commitStatus.Name, "/")
						if len(commitStatusNames) > 2 {
							if strings.Contains(integrationTestScenarioPass.Name, strings.TrimSpace(commitStatusNames[1])) {
								return commitStatus.Status
							}
						}
					}
					return ""
				}, timeout, interval).Should(Equal(integrationPipelineRunCommitStatusSuccess), fmt.Sprintf("timed out when waiting for expected commitStatus to be created for sha %s in %s repository", mrSha, componentRepoNameForStatusReporting))
			})

			It("eventually leads to the integration test PipelineRun's Pass status reported at MR notes", func() {
				expectedNote := fmt.Sprintf("Integration test for snapshot %s and scenario %s has passed", snapshot.Name, integrationTestScenarioPass.Name)
				f.AsKubeAdmin.HasController.GitLab.ValidateNoteInMergeRequestComment(projectID, expectedNote, mrID)
			})

			It("validates the Integration test scenario PipelineRun is reported to merge request CommitStatus, and it fails", func() {
				timeout = time.Second * 300
				interval = time.Second * 1

				Eventually(func() string {
					commitStatuses, _, err := f.AsKubeAdmin.CommonController.Gitlab.GetClient().Commits.GetCommitStatuses(projectID, mrSha, nil)
					Expect(err).ShouldNot(HaveOccurred())

					for _, commitStatus := range commitStatuses {
						commitStatusNames := strings.Split(commitStatus.Name, "/")
						if len(commitStatusNames) > 2 {
							if strings.Contains(integrationTestScenarioFail.Name, strings.TrimSpace(commitStatusNames[1])) {
								return commitStatus.Status
							}
						}
					}
					return ""
				}, timeout, interval).Should(Equal(integrationPipelineRunCommitStatusFail), fmt.Sprintf("timed out when waiting for expected commitStatus to be created for sha %s in %s repository", mrSha, componentRepoNameForStatusReporting))
			})

			It("eventually leads to the integration test PipelineRun's Fail status reported at MR notes", func() {
				expectedNote := fmt.Sprintf("Integration test for snapshot %s and scenario %s has failed", snapshot.Name, integrationTestScenarioFail.Name)
				f.AsKubeAdmin.HasController.GitLab.ValidateNoteInMergeRequestComment(projectID, expectedNote, mrID)
			})
		})
	})
})
