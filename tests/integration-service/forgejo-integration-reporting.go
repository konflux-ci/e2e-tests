package integration

import (
	"fmt"
	"os"
	"strings"
	"time"

	forgejoapi "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"

	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

var _ = framework.IntegrationServiceSuiteDescribe("Forgejo Status Reporting of Integration tests", ginkgo.Label("integration-service", "forgejo-status-reporting"), func() {
	defer ginkgo.GinkgoRecover()

	var f *framework.Framework
	var err error

	var mrID int
	var mrSha, projectID, forgejoToken string
	var snapshot *appstudioApi.Snapshot
	var component *appstudioApi.Component
	var buildPipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass *integrationv1beta2.IntegrationTestScenario
	var applicationName, componentName, componentBaseBranchName, pacBranchName, testNamespace string
	var mergeResult *forgejoapi.PullRequest
	var mergeResultSha string

	ginkgo.AfterEach(framework.ReportFailure(&f))

	ginkgo.Describe("Forgejo with status reporting of Integration tests in the associated merge request", ginkgo.Ordered, func() {
		ginkgo.BeforeAll(func() {
			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				ginkgo.Skip("Skipping this test due to configuration issue with Spray proxy")
			}

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("forgejo-rep"))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			testNamespace = f.UserNamespace

			if utils.IsPrivateHostname(f.OpenshiftConsoleHost) {
				ginkgo.Skip("Using private cluster (not reachable from Forgejo/Codeberg), skipping...")
			}

			applicationName = createApp(*f, testNamespace)

			// Wait for the Application to be available before creating integration test scenarios,
			// matching the pattern used in the GitLab suite to avoid race conditions.
			gomega.Eventually(func() error {
				_, err := f.AsKubeAdmin.HasController.GetApplication(applicationName, testNamespace)
				return err
			}, time.Minute*2, time.Second*5).Should(gomega.Succeed(),
				fmt.Sprintf("Application %s should be available in namespace %s", applicationName, testNamespace))

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPass, "", []string{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			componentName = fmt.Sprintf("%s-%s", "test-comp-pac-forgejo", util.GenerateRandomString(6))
			pacBranchName = constants.PaCPullRequestBranchPrefix + componentName
			componentBaseBranchName = fmt.Sprintf("base-forgejo-%s", util.GenerateRandomString(6))

			projectID = forgejoProjectIDForStatusReporting

			forgejoToken = utils.GetEnv(constants.CODEBERG_BOT_TOKEN_ENV, "")
			gomega.Expect(forgejoToken).ShouldNot(gomega.BeEmpty(), fmt.Sprintf("'%s' env var is not set", constants.CODEBERG_BOT_TOKEN_ENV))
			gomega.Expect(f.AsKubeAdmin.CommonController.Forgejo).NotTo(gomega.BeNil())

			exists, err := f.AsKubeAdmin.CommonController.Forgejo.ExistsBranch(projectID, componentDefaultBranch)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			if !exists {
				err = f.AsKubeAdmin.CommonController.Forgejo.CreateBranch(projectID, componentDefaultBranch, fallbackBranchName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}

			err = f.AsKubeAdmin.CommonController.Forgejo.CreateBranch(projectID, componentBaseBranchName, componentDefaultBranch)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			err = build.CreateCodebergBuildSecret(f, "forgejo-build-secret", map[string]string{}, forgejoToken)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// Pre-clean any stale webhooks left by a previous test run. When a webhook
			// with the same URL already exists on the Codeberg repo, build-service
			// detects it and skips re-registration — meaning pipelines-as-code-webhooks-secret
			// retains the old HMAC secret while Codeberg keeps signing events with it, causing
			// signature validation to fail for the entire new run.
			// We delete both the cluster-domain hook (gosmee) and the PaC SaaS relay
			// (hook.pipelinesascode.com) to cover all deployment topologies.
			// Errors are ignored: if no stale webhook exists this is a no-op.
			_ = f.AsKubeAdmin.CommonController.Forgejo.DeleteWebhooks(projectID, f.ClusterAppDomain)
			_ = f.AsKubeAdmin.CommonController.Forgejo.DeleteWebhooks(projectID, "hook.pipelinesascode.com")
		})

		ginkgo.AfterAll(func() {
			// Guard against mrID == 0: if the test failed before the PaC init PR was
			// discovered, mrID is never set. Calling ClosePullRequest with ID 0 returns
			// a 404 and panics here, preventing the webhook cleanup below from running
			// and leaving a stale webhook that breaks the next test run.
			// Only close the PR if it was never merged during the test. MergePullRequest
			// returns nil on any failure path, so mergeResult != nil is a reliable signal
			// that the PR was already merged. Forgejo rejects ClosePullRequest on a
			// merged PR with "cannot change state of this pull request, it was already merged".
			if mrID != 0 && mergeResult == nil {
				gomega.Expect(f.AsKubeAdmin.CommonController.Forgejo.ClosePullRequest(projectID, int64(mrID))).To(gomega.Succeed())
			}
			gomega.Expect(f.AsKubeAdmin.CommonController.Forgejo.DeleteBranch(projectID, componentBaseBranchName)).NotTo(gomega.HaveOccurred())
			// Delete webhooks for both gosmee (cluster-domain URL) and the PaC SaaS
			// relay (hook.pipelinesascode.com). On ROSA clusters the PaC relay is used
			// instead of gosmee, so without the second deletion a stale webhook persists
			// across runs and causes signature validation failures on the next run.
			_ = f.AsKubeAdmin.CommonController.Forgejo.DeleteWebhooks(projectID, f.ClusterAppDomain)
			_ = f.AsKubeAdmin.CommonController.Forgejo.DeleteWebhooks(projectID, "hook.pipelinesascode.com")

			if !ginkgo.CurrentSpecReport().Failed() {
				gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*2)).To(gomega.Succeed())
				gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*2)).To(gomega.Succeed())
			}
		})

		ginkgo.When("a new Component with specified custom branch is created", ginkgo.Label("custom-branch"), func() {
			ginkgo.BeforeAll(func() {
				componentObj := appstudioApi.ComponentSpec{
					ComponentName: componentName,
					Application:   applicationName,
					Source: appstudioApi.ComponentSource{
						ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
							GitSource: &appstudioApi.GitSource{
								URL:           forgejoComponentGitSourceURLForStatusReporting,
								Revision:      componentBaseBranchName,
								DockerfileURL: "Dockerfile",
							},
						},
					},
				}
				buildPipelineAnnotation := build.GetBuildPipelineBundleAnnotation(constants.DockerBuildOciTAMin)
				gitProviderAnnotation := map[string]string{"git-provider": "forgejo"}

				component, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, false,
					utils.MergeMaps(
						utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation),
						gitProviderAnnotation,
					))
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// Wait for the PaC Repository CR to be created by build-service. This confirms
			// that the PaC webhook has been registered and its secret written to
			// pipelines-as-code-webhooks-secret.
			gomega.Eventually(func() error {
				_, getErr := f.AsKubeAdmin.TektonController.GetRepositoryParams(componentName, testNamespace)
				return getErr
			}, time.Minute*5, time.Second*5).Should(gomega.Succeed(),
				fmt.Sprintf("timed out waiting for PaC Repository CR for component %s in namespace %s", componentName, testNamespace))

			// Add the Forgejo bot user to the Repository CR's pull_request policy
			// allowlist. PaC's Gitea authorization check calls the Gitea API's
			// IsCollaborator endpoint, which returns 404 for users who have access
			// via org membership rather than an explicit collaborator entry. This
			// causes every pull_request event from build-service's bot account to be
			// rejected with "not allowed to trigger CI". Patching the policy.pull_request
			// list here ensures PaC trusts the bot without requiring infrastructure
			// changes to the Codeberg repository's collaborator settings.
			botUser, _, err := f.AsKubeAdmin.CommonController.Forgejo.GetClient().GetMyUserInfo()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to get Forgejo bot username")
			gomega.Expect(f.AsKubeAdmin.TektonController.PatchRepositoryPullRequestPolicy(
				componentName, testNamespace, []string{botUser.UserName},
			)).To(gomega.Succeed(),
				fmt.Sprintf("failed to patch PaC Repository CR pull_request policy for component %s", componentName))

			// Wait for build-service to open the PaC init PR. By the time the PR is
			// visible, PaC's informer cache will have had time to refresh with the new
			// webhook secret. The initial push and pull_request events fired by Codeberg
			// arrive within seconds of webhook registration — before PaC's cache is
			// updated — causing signature validation failures for those early events.
			// Waiting here ensures the retrigger commit below is processed with a
			// valid, up-to-date secret.
				gomega.Eventually(func() bool {
					prs, listErr := f.AsKubeAdmin.CommonController.Forgejo.GetPullRequests(projectID)
					if listErr != nil {
						ginkgo.GinkgoWriter.Printf("error listing pull requests while waiting for PaC init PR: %v\n", listErr)
						return false
					}
					for _, pr := range prs {
						if pr.Head != nil && pr.Head.Ref == pacBranchName {
							mrID = int(pr.Index)
							return true
						}
					}
					return false
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(gomega.BeTrue(),
					fmt.Sprintf("timed out waiting for PaC init PR (branch %s) to be created in %s", pacBranchName, forgejoComponentRepoName))

				// Push a retrigger commit to the PaC init branch so PaC processes the
				// pull_request event with its now-refreshed webhook secret cache. Without
				// this, the test relies on the initial events which arrive before the cache
				// is updated and are rejected with a signature validation failure.
				_, err = f.AsKubeAdmin.CommonController.Forgejo.CreateFile(
					projectID, "retrigger.txt",
					fmt.Sprintf("retrigger PaC build for component %s", componentName),
					pacBranchName,
				)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.It("triggers a Build PipelineRun", func() {
				gomega.Eventually(func() error {
					buildPipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
					if err != nil {
						ginkgo.GinkgoWriter.Printf("Build PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
						return err
					}
					if !buildPipelineRun.HasStarted() {
						return fmt.Errorf("build pipelinerun %s/%s hasn't started yet", buildPipelineRun.GetNamespace(), buildPipelineRun.GetName())
					}
					return nil
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to start for the component %s/%s", testNamespace, componentName))
			})

			ginkgo.It("does not contain an annotation with a Snapshot Name", func() {
				gomega.Expect(buildPipelineRun.Annotations[snapshotAnnotation]).To(gomega.Equal(""))
			})

			ginkgo.It("should lead to build PipelineRun finishing successfully", func() {
				gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", "",
					"", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, buildPipelineRun)).To(gomega.Succeed())
			})

			ginkgo.It("should have a related PaC init MR created", func() {
				gomega.Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Forgejo.GetPullRequests(projectID)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					for _, pr := range prs {
						if pr.Head != nil && pr.Head.Ref == pacBranchName {
							mrID = int(pr.Index)
							mrSha = pr.Head.Sha
							return true
						}
					}
					return false
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for init PaC MR (branch name '%s') to be created in %s repository", pacBranchName, forgejoComponentRepoName))

				// In case the first pipelineRun attempt failed and was retried, refresh the variable.
				buildPipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, mrSha)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.It(fmt.Sprintf("the PipelineRun should eventually finish successfully for component %s", componentName), func() {
				gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", "", "",
					f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, nil)).To(gomega.Succeed())
			})
		})

		ginkgo.When("the PaC build pipelineRun run succeeded", func() {
			ginkgo.It("checks if the BuildPipelineRun has the annotation of chains signed", func() {
				gomega.Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, chainsSignedAnnotation)).To(gomega.Succeed())
			})

			ginkgo.It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentName, testNamespace)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})

			ginkgo.It("checks if the Build PipelineRun got annotated with Snapshot name", func() {
				gomega.Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, snapshotAnnotation)).To(gomega.Succeed())
			})
		})

		ginkgo.When("the Snapshot was created", func() {
			ginkgo.It("should find the Integration Test Scenario PipelineRun", func() {
				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioPass.Name, snapshot.Name, testNamespace)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(testPipelinerun.Labels[snapshotAnnotation]).To(gomega.ContainSubstring(snapshot.Name))
				gomega.Expect(testPipelinerun.Labels[scenarioAnnotation]).To(gomega.ContainSubstring(integrationTestScenarioPass.Name))
			})
		})

		ginkgo.When("Integration PipelineRun is created", func() {
			ginkgo.It("should eventually complete successfully", func() {
				gomega.Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(gomega.Succeed(),
					fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})

			ginkgo.It("eventually leads to the integration test PipelineRun's Pass status reported at MR commit status", func() {
				gomega.Expect(
					f.AsKubeAdmin.HasController.Forgejo.GetCommitStatusConclusion(integrationTestScenarioPass.Name, projectID, mrSha, int64(mrID)),
				).To(gomega.Equal(integrationPipelineRunCommitStatusSuccess))
			})

			ginkgo.It("validates the integration test result is reported as a passing commit status on the MR", func() {
				owner, repo, ok := strings.Cut(projectID, "/")
				gomega.Expect(ok).To(gomega.BeTrue(),
					fmt.Sprintf("invalid projectID %q: expected 'owner/repo' format", projectID))

				// The ForgejoReporter publishes integration test results as Forgejo commit
				// statuses (not as PR comments). Verify that a status whose context contains
				// the integration test scenario name transitions to the expected success state.
				gomega.Eventually(func() bool {
					combined, _, err := f.AsKubeAdmin.CommonController.Forgejo.GetClient().GetCombinedStatus(owner, repo, mrSha)
					if err != nil {
						ginkgo.GinkgoWriter.Printf("failed to get combined commit status for %s: %v\n", mrSha, err)
						return false
					}
					for _, s := range combined.Statuses {
						if strings.Contains(s.Context, integrationTestScenarioPass.Name) &&
							string(s.State) == integrationPipelineRunCommitStatusSuccess {
							return true
						}
					}
					return false
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(gomega.BeTrue(),
					fmt.Sprintf("no passing commit status found for scenario %q on commit %s in project %s",
						integrationTestScenarioPass.Name, mrSha, forgejoComponentRepoName))
			})

			ginkgo.It("merging the PR should be successful", func() {
				gomega.Eventually(func() error {
					mergeResult, err = f.AsKubeAdmin.CommonController.Forgejo.MergePullRequest(projectID, int64(mrID))
					return err
				}, shortTimeout, constants.PipelineRunPollingInterval).ShouldNot(gomega.HaveOccurred(),
					fmt.Sprintf("error when merging PaC merge request ID #%d in ProjectID %s", mrID, projectID))

				if mergeResult != nil && mergeResult.MergedCommitID != nil {
					mergeResultSha = *mergeResult.MergedCommitID
				} else if mergeResult != nil {
					mergeResultSha = mergeResult.Head.Sha
				}
				ginkgo.GinkgoWriter.Printf("merged result sha: %s for MR #%d\n", mergeResultSha, mrID)
			})

			ginkgo.It("leads to triggering a push PipelineRun", func() {
				gomega.Eventually(func() error {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, mergeResultSha)
					if err != nil {
						ginkgo.GinkgoWriter.Printf("Push PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("push pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, shortTimeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(),
					fmt.Sprintf("timed out when waiting for the push PipelineRun to start for the component %s/%s", testNamespace, componentName))
			})
		})

		ginkgo.When("Run integration tests after Merged MR", func() {
			ginkgo.It("should eventually complete successfully", func() {
				gomega.Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(gomega.Succeed(),
					fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})

			ginkgo.It("eventually leads to the integration test PipelineRun's Pass status reported at MR commit status", func() {
				gomega.Expect(
					f.AsKubeAdmin.HasController.Forgejo.GetCommitStatusConclusion(integrationTestScenarioPass.Name, projectID, mrSha, int64(mrID)),
				).To(gomega.Equal(integrationPipelineRunCommitStatusSuccess))
			})
		})
	})
})
