package integration

import (
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

var _ = framework.IntegrationServiceSuiteDescribe("Gitlab Status Reporting of Integration tests", Label("integration-service", "HACBS", "gitlab-status-reporting"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var prNumber int
	var timeout, interval time.Duration
	var osConsoleHost, prHeadSha string
	var snapshot *appstudioApi.Snapshot
	var component *appstudioApi.Component
	var pipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass, integrationTestScenarioFail *integrationv1beta1.IntegrationTestScenario
	var applicationName, componentName, componentBaseBranchName, pacBranchName, testNamespace string

	AfterEach(framework.ReportFailure(&f))

	Describe("Gitlab with status reporting of Integration tests in CheckRuns", Ordered, func() {
		BeforeAll(func() {
			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				Skip("Skipping this test due to configuration issue with Spray proxy")
			}

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("gitlab-rep"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			consoleRoute, err := f.AsKubeAdmin.CommonController.GetOpenshiftRoute("console", "openshift-console")
			Expect(err).ShouldNot(HaveOccurred())
			osConsoleHost = consoleRoute.Spec.Host

			if utils.IsPrivateHostname(osConsoleHost) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			applicationName = createApp(*f, testNamespace)

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, testNamespace, gitURL, revision, pathInRepoPass)
			Expect(err).ShouldNot(HaveOccurred())

			integrationTestScenarioFail, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, testNamespace, gitURL, revision, pathInRepoFail)
			Expect(err).ShouldNot(HaveOccurred())

			componentName = fmt.Sprintf("%s-%s", "test-comp-pac-gitlab", util.GenerateRandomString(6))
			pacBranchName = constants.PaCPullRequestBranchPrefix + componentName
			componentBaseBranchName = fmt.Sprintf("base-gitlab-%s", util.GenerateRandomString(6))

			// Expect(f.AsKubeAdmin.CommonController.Gitlab.DeleteAllBranchesOfProjectID("56586709")).To(BeTrue(), "Could not delete all branches for Project ID: 56586709")

			err = f.AsKubeAdmin.CommonController.Gitlab.CreateGitlabNewBranch("56586709", componentBaseBranchName, componentRevision, componentDefaultBranch) //CreateBranch("56586709", componentBaseBranchName, componentDefaultBranch)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentName)
			}

			// Delete new branches created by PaC and a testing branch used as a component's base branch

		})

		When("a new Component with specified custom branch is created", Label("custom-branch"), func() {
			BeforeAll(func() {
				componentObj := appstudioApi.ComponentSpec{
					ComponentName: componentName,
					Application:   applicationName,
					Source: appstudioApi.ComponentSource{
						ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
							GitSource: &appstudioApi.GitSource{
								URL:        gitlabComponentGitSourceURLForStatusReporting,
								Revision:   componentBaseBranchName,
								DevfileURL: gitlabComponentSourceForGitlabReportingDevFile,
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
			fmt.Println(prNumber, interval, prHeadSha, snapshot, component, testPipelinerun, integrationTestScenarioPass, integrationTestScenarioFail, pacBranchName)
		})

	})
})
