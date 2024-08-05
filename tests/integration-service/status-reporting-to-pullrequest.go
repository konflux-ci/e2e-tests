package integration

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/devfile/library/v2/pkg/util"
	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = framework.IntegrationServiceSuiteDescribe("Status Reporting of Integration tests", Label("integration-service", "github-status-reporting"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var prNumber int
	var timeout, interval time.Duration
	var prHeadSha string
	var snapshot *appstudioApi.Snapshot
	var snapshotPush *appstudioApi.Snapshot
	var component *appstudioApi.Component
	var spaceRequest *v1alpha1.SpaceRequest
	var pipelineRun, testPipelinerun *pipeline.PipelineRun
	var integrationTestScenarioPass, integrationTestScenarioFail, integrationTestScenarioEnv *integrationv1beta2.IntegrationTestScenario
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

			integrationTestScenarioPass, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoPass)
			Expect(err).ShouldNot(HaveOccurred())

			integrationTestScenarioFail, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathInRepoFail)
			Expect(err).ShouldNot(HaveOccurred())

			integrationTestScenarioEnv, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", applicationName, testNamespace, gitURL, revision, pathIntegrationPipelineWithEnv)
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
			Expect(build.CleanupWebhooks(f, componentRepoNameForStatusReporting)).ShouldNot(HaveOccurred())
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
				component, err = f.AsKubeAdmin.HasController.CreateComponent(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), constants.DefaultDockerBuildPipelineBundle))
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
			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
			})
			It("verifies if the build PipelineRun contains the finalizer", func() {
				Eventually(func() error {
					pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(componentName, applicationName, testNamespace, true, "")
					Expect(err).ShouldNot(HaveOccurred())
					if !controllerutil.ContainsFinalizer(pipelineRun, pipelinerunFinalizerByIntegrationService) {
						return fmt.Errorf("build pipelineRun %s/%s doesn't contain the finalizer: %s yet", pipelineRun.GetNamespace(), pipelineRun.GetName(), pipelinerunFinalizerByIntegrationService)
					}
					return nil
				}, 1*time.Minute, 1*time.Second).Should(Succeed(), "timeout when waiting for finalizer to be added")
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

			It("verifies that the finalizer has been removed from the build pipelinerun", func() {
				Eventually(func() error {
					pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(componentName, applicationName, testNamespace, true, "")
					Expect(err).ShouldNot(HaveOccurred())
					if controllerutil.ContainsFinalizer(pipelineRun, pipelinerunFinalizerByIntegrationService) {
						return fmt.Errorf("build pipelineRun %s/%s still contains the finalizer: %s", pipelineRun.GetNamespace(), pipelineRun.GetName(), pipelinerunFinalizerByIntegrationService)
					}
					return nil
				}, 60*time.Second, 1*time.Second).Should(Succeed(), "timeout when waiting for finalizer to be removed")
			})

			It(fmt.Sprintf("checks if CronJob %s exists", spaceRequestCronJobName), func() {
				spaceRequestCleanerCronJob, err := f.AsKubeAdmin.CommonController.GetCronJob(spaceRequestCronJobNamespace, spaceRequestCronJobName)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(strings.Contains(spaceRequestCleanerCronJob.Name, spaceRequestCronJobName)).Should(BeTrue())
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

				testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenarioEnv.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenarioEnv.Name))
			})
		})

		When("Integration PipelineRuns are created", func() {
			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioPass, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioFail, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenarioEnv, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for an integration pipelinerun for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})

			It("checks if space request is created in namespace", func() {
				spaceRequestsList, err := f.AsKubeAdmin.GitOpsController.GetSpaceRequests(testNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(spaceRequestsList.Items).To(HaveLen(1), "Expected spaceRequestsList.Items to have at least one item")
				spaceRequest = &spaceRequestsList.Items[0]
				Expect(strings.Contains(spaceRequest.Name, spaceRequestNamePrefix)).Should(BeTrue())
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

			It("checks if the finalizer was removed from all of the related Integration pipelineRuns", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForFinalizerToGetRemovedFromAllIntegrationPipelineRuns(testNamespace, applicationName, snapshot)).To(Succeed())
			})

			It("checks that when deleting integration test scenario pipelineRun, spaceRequest is deleted too", func() {
				testPipelinerun, err = f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenarioEnv.Name, snapshot.Name, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(f.AsKubeDeveloper.TektonController.DeletePipelineRun(testPipelinerun.Name, testPipelinerun.Namespace)).To(Succeed())

				timeout = time.Second * 200
				interval = time.Second * 5
				Eventually(func() error {
					currentSpaceRequest, err := f.AsKubeAdmin.GitOpsController.GetSpaceRequest(testNamespace, spaceRequest.Name)
					if err != nil {
						if k8sErrors.IsNotFound(err) {
							return nil
						}
						return fmt.Errorf("failed to get %s/%s spaceRequest: %+v", currentSpaceRequest.Namespace, currentSpaceRequest.Name, err)
					}
					return fmt.Errorf("spaceRequest %s/%s still exists", currentSpaceRequest.Namespace, currentSpaceRequest.Name)
				}, timeout, interval).Should(Succeed())

			})
		})

		It("creates a ReleasePlan", func() {
			_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlan(autoReleasePlan, testNamespace, applicationName, targetReleaseNamespace, "", nil, nil)
			Expect(err).ShouldNot(HaveOccurred())
			testScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			for _, testScenario := range *testScenarios {
				GinkgoWriter.Printf("IntegrationTestScenario %s is found\n", testScenario.Name)
			}
		})

		It("creates an snapshot of push event", func() {
			sampleImage := "quay.io/redhat-appstudio/sample-image@sha256:841328df1b9f8c4087adbdcfec6cc99ac8308805dea83f6d415d6fb8d40227c1"
			snapshotPush, err = f.AsKubeAdmin.IntegrationController.CreateSnapshotWithImage(componentName, applicationName, testNamespace, sampleImage)
			Expect(err).ShouldNot(HaveOccurred())
		})

		When("An snapshot of push event is created", func() {
			It("checks if the global candidate is updated after push event", func() {
				timeout = time.Second * 600
				interval = time.Second * 10
				Eventually(func() error {
					snapshotPush, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshotPush.Name, "", "", testNamespace)
					Expect(err).ShouldNot(HaveOccurred())

					latestComponent, err := f.AsKubeAdmin.HasController.GetComponentByApplicationName(applicationName, testNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(latestComponent.Spec.ContainerImage).ToNot(Equal(component.Spec.ContainerImage))
					return nil

				}, timeout, interval).Should(Succeed(), fmt.Sprintf("time out when waiting for updating the global candidate in %s namespace", testNamespace))
			})

			It("checks if all of the integrationPipelineRuns created by push event passed", Label("slow"), func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName, snapshotPush)).To(Succeed(), "Error when waiting for one of the integration pipelines to finish in %s namespace", testNamespace)
			})

			It("checks if a Release is created successfully", func() {
				timeout = time.Second * 60
				interval = time.Second * 5
				Eventually(func() error {
					_, err := f.AsKubeAdmin.ReleaseController.GetReleases(testNamespace)
					return err
				}, timeout, interval).Should(Succeed(), fmt.Sprintf("time out when waiting for release created for snapshot %s/%s", snapshotPush.GetNamespace(), snapshotPush.GetName()))
			})
		})
	})
})

func createApp(f framework.Framework, testNamespace string) string {
	applicationName := fmt.Sprintf("integ-app-%s", util.GenerateRandomString(4))

	_, err := f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
	Expect(err).NotTo(HaveOccurred())

	return applicationName
}

func cleanup(f framework.Framework, testNamespace, applicationName, componentName string) {
	if !CurrentSpecReport().Failed() {
		Expect(f.AsKubeAdmin.HasController.DeleteApplication(applicationName, testNamespace, false)).To(Succeed())
		Expect(f.AsKubeAdmin.HasController.DeleteComponent(componentName, testNamespace, false)).To(Succeed())
		integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, testNamespace)
		Expect(err).ShouldNot(HaveOccurred())

		for _, testScenario := range *integrationTestScenarios {
			Expect(f.AsKubeAdmin.IntegrationController.DeleteIntegrationTestScenario(&testScenario, testNamespace)).To(Succeed())
		}
		Expect(f.SandboxController.DeleteUserSignup(f.UserName)).To(BeTrue())
	}
}
