package integration

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	containerImageSource = "quay.io/redhat-appstudio-qe/busybox-loop:latest"
	gitSourceRepoName    = "devfile-sample-python-basic"
	gitSourceURL         = "https://github.com/redhat-appstudio-qe/" + gitSourceRepoName
	bundleURL            = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass"
	inPipelineName       = "integration-pipeline-pass"
)

var _ = framework.IntegrationServiceSuiteDescribe("Integration Service E2E tests", Label("integration-service", "HACBS"), func() {
	defer GinkgoRecover()

	var applicationName, componentName, appStudioE2EApplicationsNamespace, outputContainerImage string
	var timeout, interval time.Duration
	var applicationSnapshot *appstudioApi.Snapshot
	var applicationSnapshot_push *appstudioApi.Snapshot
	var env *appstudioApi.Environment

	var defaultBundleConfigMap *v1.ConfigMap

	// Initialize the tests controllers
	f, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())
	Describe("the component with git source (GitHub) is created", Ordered, Label("github-webhook"), func() {
		BeforeAll(func() {
			applicationName = fmt.Sprintf("integ-app-%s", util.GenerateRandomString(4))
			appStudioE2EApplicationsNamespace = utils.GetGeneratedNamespace("integ-e2e")

			_, err := f.CommonController.CreateTestNamespace(appStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", appStudioE2EApplicationsNamespace, err)

			app, err := f.HasController.CreateHasApplication(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)
			DeferCleanup(f.HasController.DeleteHasApplication, applicationName, appStudioE2EApplicationsNamespace, false)

			componentName = fmt.Sprintf("integration-suite-test-component-git-source-%s", util.GenerateRandomString(4))
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			timeout = time.Minute * 4
			interval = time.Second * 1
			// Create a component with Git Source URL being defined
			_, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", "", outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace, false)

			defaultBundleConfigMap, err = f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace)
			if err != nil {
				if errors.IsForbidden(err) {
					GinkgoWriter.Printf("don't have enough permissions to get a configmap with default pipeline in %s namespace\n", constants.BuildPipelinesConfigMapDefaultNamespace)
				} else {
					Fail(fmt.Sprintf("error occurred when trying to get configmap %s in %s namespace: %v", constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace, err))
				}
			}
			_ = defaultBundleConfigMap.Data["default_build_bundle"]
			_, err = f.IntegrationController.CreateIntegrationTestScenario(applicationName, appStudioE2EApplicationsNamespace, bundleURL, inPipelineName)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			Expect(f.HasController.DeleteHasApplication(applicationName, appStudioE2EApplicationsNamespace, false)).To(Succeed())
			Expect(f.HasController.DeleteHasComponent(componentName, appStudioE2EApplicationsNamespace, false)).To(Succeed())
			err = f.IntegrationController.DeleteApplicationSnapshot(applicationSnapshot_push, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			integrationTestScenarios, err := f.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())

			for _, testScenario := range *integrationTestScenarios {
				err = f.IntegrationController.DeleteIntegrationTestScenario(&testScenario, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
			}
		})

		It("triggers a build PipelineRun", Label("integration-service"), func() {
			timeout = time.Second * 60
			interval = time.Second * 2
			Eventually(func() bool {
				pipelineRun, err := f.IntegrationController.GetBuildPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			timeout = time.Second * 600
			interval = time.Second * 10
			Eventually(func() bool {
				pipelineRun, err := f.IntegrationController.GetBuildPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false, "")
				Expect(err).ShouldNot(HaveOccurred())

				for _, condition := range pipelineRun.Status.Conditions {
					GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

					if condition.Reason == "Failed" {
						Fail(fmt.Sprintf("Pipelinerun %s has failed", pipelineRun.Name))
					}
				}
				return pipelineRun.IsDone()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")

		})

		When("the build pipelineRun run succeeded", func() {
			It("check if the ApplicationSnapshot is created", func() {
				// snapshotName is sent as empty since it is unknown at this stage
				applicationSnapshot, err = f.IntegrationController.GetApplicationSnapshot("", applicationName, appStudioE2EApplicationsNamespace, componentName)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("applicationSnapshot %s is found\n", applicationSnapshot.Name)
			})
			It("check if all of the integrationPipelineRuns passed", Label("slow"), func() {
				integrationTestScenarios, err := f.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				for _, testScenario := range *integrationTestScenarios {
					timeout = time.Second * 60
					interval = time.Second * 2
					Eventually(func() bool {
						pipelineRun, err := f.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot.Name, appStudioE2EApplicationsNamespace)
						if err != nil {
							GinkgoWriter.Printf("cannot get the Integration PipelineRun: %v\n", err)
							return false
						}
						return pipelineRun.HasStarted()
					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
					timeout = time.Second * 800
					interval = time.Second * 10
					Eventually(func() bool {
						Expect(f.IntegrationController.WaitForIntegrationPipelineToBeFinished(&testScenario, applicationSnapshot, applicationName, appStudioE2EApplicationsNamespace)).To(Succeed(), "Error when waiting for a integration pipeline to finish")
						return true
					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
				}
			})
		})

		It("create a ReleasePlan and an environment", func() {
			_, err = f.IntegrationController.CreateReleasePlan(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			env, err = f.IntegrationController.CreateEnvironment(appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			testScenarios, err := f.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			for _, testScenario := range *testScenarios {
				GinkgoWriter.Printf("IntegrationTestScenario %s is found\n", testScenario.Name)
			}
		})

		It("create an applicationSnapshot of push event", func() {
			applicationSnapshot_push, err = f.IntegrationController.CreateApplicationSnapshot(applicationName, appStudioE2EApplicationsNamespace, componentName)
			Expect(err).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("applicationSnapshot %s is found\n", applicationSnapshot_push.Name)
		})

		When("An applicationSnapshot of push event is created", func() {
			It("check if all of the integrationPipelineRuns created by push event passed", Label("slow"), func() {
				integrationTestScenarios, err := f.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				for _, testScenario := range *integrationTestScenarios {
					timeout = time.Second * 60
					interval = time.Second * 2
					Eventually(func() bool {
						pipelineRun, err := f.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot_push.Name, appStudioE2EApplicationsNamespace)
						if err != nil {
							GinkgoWriter.Printf("cannot get the Integration PipelineRun: %v\n", err)
							return false
						}
						return pipelineRun.HasStarted()

					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
					timeout = time.Second * 600
					interval = time.Second * 10
					Eventually(func() bool {
						pipelineRun, err := f.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot_push.Name, appStudioE2EApplicationsNamespace)
						Expect(err).ShouldNot(HaveOccurred())

						for _, condition := range pipelineRun.Status.Conditions {
							GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)
							if condition.Reason == "Failed" {
								Fail(fmt.Sprintf("Pipelinerun %s has failed", pipelineRun.Name))
							}
						}
						return pipelineRun.IsDone()
					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
				}
			})

			It("check if the global candidate is updated after push event", func() {
				timeout = time.Second * 600
				interval = time.Second * 10
				Eventually(func() bool {
					if f.IntegrationController.HaveHACBSTestsSucceeded(applicationSnapshot_push) {
						component, _ := f.IntegrationController.GetComponent(applicationName, appStudioE2EApplicationsNamespace)
						Expect(component.Spec.ContainerImage != "").To(BeTrue())
						GinkgoWriter.Printf("Global candidate is updated\n")
						return true
					}
					applicationSnapshot_push, err = f.IntegrationController.GetApplicationSnapshot(applicationSnapshot_push.Name, "", appStudioE2EApplicationsNamespace, "")
					return false
				}, timeout, interval).Should(BeTrue(), "time out when waiting for updating the global candidate")
			})

			It("check if a Release is created successfully", func() {
				timeout = time.Second * 800
				interval = time.Second * 10
				Eventually(func() bool {
					if f.IntegrationController.HaveHACBSTestsSucceeded(applicationSnapshot_push) {
						releases, err := f.IntegrationController.GetReleasesWithApplicationSnapshot(applicationSnapshot_push, appStudioE2EApplicationsNamespace)
						Expect(err).ShouldNot(HaveOccurred())
						if len(*releases) != 0 {
							for _, release := range *releases {
								GinkgoWriter.Printf("Release %s is found\n", release.Name)
							}
						} else {
							Fail("No Release found")
						}
						return true
					}
					applicationSnapshot_push, err = f.IntegrationController.GetApplicationSnapshot(applicationSnapshot_push.Name, "", appStudioE2EApplicationsNamespace, "")
					return false
				}, timeout, interval).Should(BeTrue(), "time out when waiting for release created")
			})

			It("check if an EnvironmentBinding is created successfully", func() {
				timeout = time.Second * 600
				interval = time.Second * 2
				Eventually(func() bool {
					if f.IntegrationController.HaveHACBSTestsSucceeded(applicationSnapshot_push) {
						envbinding, err := f.IntegrationController.GetSnapshotEnvironmentBinding(applicationName, appStudioE2EApplicationsNamespace, env)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(envbinding != nil).To(BeTrue())
						GinkgoWriter.Printf("The EnvironmentBinding is created\n")
						return true
					}
					applicationSnapshot_push, err = f.IntegrationController.GetApplicationSnapshot(applicationSnapshot_push.Name, "", appStudioE2EApplicationsNamespace, "")
					return false
				}, timeout, interval).Should(BeTrue(), "time out when waiting for release created")
			})
		})
	})
})
