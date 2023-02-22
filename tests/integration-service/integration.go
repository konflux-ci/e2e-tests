package integration

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	containerImageSource   = "quay.io/redhat-appstudio-qe/busybox-loop:latest"
	gitSourceRepoName      = "devfile-sample-python-basic"
	gitSourceURL           = "https://github.com/redhat-appstudio-qe/" + gitSourceRepoName
	BundleURL              = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass"
	InPipelineName         = "integration-pipeline-pass"
	EnvironmentName        = "development"
	IntegrationServiceUser = "integration-e2e"
)

var _ = framework.IntegrationServiceSuiteDescribe("Integration Service E2E tests", Label("integration-service", "HACBS"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var applicationName, componentName, appStudioE2EApplicationsNamespace, outputContainerImage string
	var timeout, interval time.Duration
	var applicationSnapshot *appstudioApi.Snapshot
	var applicationSnapshot_push *appstudioApi.Snapshot
	var env *appstudioApi.Environment

	Describe("the component with git source (GitHub) is created", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			f, err = framework.NewFramework(IntegrationServiceUser)
			Expect(err).NotTo(HaveOccurred())

			applicationName = fmt.Sprintf("integ-app-%s", util.GenerateRandomString(4))
			appStudioE2EApplicationsNamespace = f.UserNamespace

			_, err := f.AsKubeAdmin.CommonController.CreateTestNamespace(appStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", appStudioE2EApplicationsNamespace, err)

			app, err := f.AsKubeAdmin.HasController.CreateHasApplication(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			componentName = fmt.Sprintf("integration-suite-test-component-git-source-%s", util.GenerateRandomString(4))
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			timeout = time.Minute * 4
			interval = time.Second * 1
			// Create a component with Git Source URL being defined
			_, err = f.AsKubeAdmin.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", "", outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, appStudioE2EApplicationsNamespace, BundleURL, InPipelineName)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.HasController.DeleteHasApplication(applicationName, appStudioE2EApplicationsNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.HasController.DeleteHasComponent(componentName, appStudioE2EApplicationsNamespace, false)).To(Succeed())
				err = f.AsKubeAdmin.IntegrationController.DeleteApplicationSnapshot(applicationSnapshot_push, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				for _, testScenario := range *integrationTestScenarios {
					err = f.AsKubeAdmin.IntegrationController.DeleteIntegrationTestScenario(&testScenario, appStudioE2EApplicationsNamespace)
					Expect(err).ShouldNot(HaveOccurred())

					for _, testScenario := range *integrationTestScenarios {
						err = f.AsKubeAdmin.IntegrationController.DeleteIntegrationTestScenario(&testScenario, appStudioE2EApplicationsNamespace)
						Expect(err).ShouldNot(HaveOccurred())
					}
				}
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}
		})

		It("triggers a build PipelineRun", Label("integration-service"), func() {
			timeout = time.Minute * 10
			interval = time.Second * 2
			Eventually(func() bool {
				pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetBuildPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			timeout = time.Second * 2000
			interval = time.Second * 10
			Eventually(func() bool {
				pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetBuildPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false, "")
				Expect(err).ShouldNot(HaveOccurred())

				for _, condition := range pipelineRun.Status.Conditions {
					GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

					if condition.Reason == "Failed" {
						Fail(tekton.GetFailedPipelineRunLogs(f.AsKubeAdmin.CommonController, pipelineRun))
					}
				}
				return pipelineRun.IsDone()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")

		})

		When("the build pipelineRun run succeeded", func() {
			It("checks if the ApplicationSnapshot is created", func() {
				// snapshotName is sent as empty since it is unknown at this stage
				applicationSnapshot, err = f.AsKubeAdmin.IntegrationController.GetApplicationSnapshot("", applicationName, appStudioE2EApplicationsNamespace, componentName)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("applicationSnapshot %s is found\n", applicationSnapshot.Name)
			})
			It("checks if all of the integrationPipelineRuns passed", Label("slow"), func() {
				integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				for _, testScenario := range *integrationTestScenarios {
					timeout = time.Minute * 5
					interval = time.Second * 2
					Eventually(func() bool {
						pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot.Name, appStudioE2EApplicationsNamespace)
						if err != nil {
							GinkgoWriter.Printf("cannot get the Integration PipelineRun: %v\n", err)
							return false
						}
						return pipelineRun.HasStarted()
					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
					timeout = time.Second * 1000
					interval = time.Second * 10
					Eventually(func() bool {
						Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(&testScenario, applicationSnapshot, applicationName, appStudioE2EApplicationsNamespace)).To(Succeed(), "Error when waiting for a integration pipeline to finish")
						return true
					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
				}
			})
		})

		It("creates a ReleasePlan and an environment", func() {
			_, err = f.AsKubeAdmin.IntegrationController.CreateReleasePlan(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			env, err = f.AsKubeAdmin.IntegrationController.CreateEnvironment(appStudioE2EApplicationsNamespace, EnvironmentName)
			Expect(err).ShouldNot(HaveOccurred())
			testScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			for _, testScenario := range *testScenarios {
				GinkgoWriter.Printf("IntegrationTestScenario %s is found\n", testScenario.Name)
			}
		})

		It("creates an applicationSnapshot of push event", func() {
			sample_image := "quay.io/redhat-appstudio/sample-image"
			applicationSnapshot_push, err = f.AsKubeAdmin.IntegrationController.CreateApplicationSnapshot(applicationName, appStudioE2EApplicationsNamespace, componentName, sample_image)
			Expect(err).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("applicationSnapshot %s is found\n", applicationSnapshot_push.Name)
		})

		When("An applicationSnapshot of push event is created", func() {
			It("checks if all of the integrationPipelineRuns created by push event passed", Label("slow"), func() {
				integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				for _, testScenario := range *integrationTestScenarios {
					timeout = time.Minute * 5
					interval = time.Second * 2
					Eventually(func() bool {
						pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot_push.Name, appStudioE2EApplicationsNamespace)
						if err != nil {
							GinkgoWriter.Printf("cannot get the Integration PipelineRun: %v\n", err)
							return false
						}
						return pipelineRun.HasStarted()

					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
					timeout = time.Second * 600
					interval = time.Second * 10
					Eventually(func() bool {
						pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot_push.Name, appStudioE2EApplicationsNamespace)
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

			It("checks if the global candidate is updated after push event", func() {
				timeout = time.Second * 600
				interval = time.Second * 10
				Eventually(func() bool {
					if f.AsKubeAdmin.IntegrationController.HaveHACBSTestsSucceeded(applicationSnapshot_push) {
						component, err := f.AsKubeAdmin.IntegrationController.GetComponent(applicationName, appStudioE2EApplicationsNamespace)
						if err != nil {
							GinkgoWriter.Println("component has not been found yet")
							return false
						}
						Expect(component.Spec.ContainerImage != "").To(BeTrue())
						GinkgoWriter.Printf("Global candidate is updated\n")
						return true
					}
					applicationSnapshot_push, err = f.AsKubeAdmin.IntegrationController.GetApplicationSnapshot(applicationSnapshot_push.Name, "", appStudioE2EApplicationsNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("snapshot %s has not been found yet\n", applicationSnapshot_push.Name)
					}
					return false
				}, timeout, interval).Should(BeTrue(), "time out when waiting for updating the global candidate")
			})

			It("checks if a Release is created successfully", func() {
				timeout = time.Second * 800
				interval = time.Second * 10
				Eventually(func() bool {
					if f.AsKubeAdmin.IntegrationController.HaveHACBSTestsSucceeded(applicationSnapshot_push) {
						releases, err := f.AsKubeAdmin.IntegrationController.GetReleasesWithApplicationSnapshot(applicationSnapshot_push, appStudioE2EApplicationsNamespace)
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
					applicationSnapshot_push, err = f.AsKubeAdmin.IntegrationController.GetApplicationSnapshot(applicationSnapshot_push.Name, "", appStudioE2EApplicationsNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("snapshot %s has not been found yet\n", applicationSnapshot_push.Name)
					}
					return false
				}, timeout, interval).Should(BeTrue(), "time out when waiting for release created")
			})

			It("checks if an EnvironmentBinding is created successfully", func() {
				timeout = time.Second * 600
				interval = time.Second * 2
				Eventually(func() bool {
					if f.AsKubeAdmin.IntegrationController.HaveHACBSTestsSucceeded(applicationSnapshot_push) {
						envbinding, err := f.AsKubeAdmin.IntegrationController.GetSnapshotEnvironmentBinding(applicationName, appStudioE2EApplicationsNamespace, env)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(envbinding != nil).To(BeTrue())
						GinkgoWriter.Printf("The EnvironmentBinding is created\n")
						return true
					}
					applicationSnapshot_push, err = f.AsKubeAdmin.IntegrationController.GetApplicationSnapshot(applicationSnapshot_push.Name, "", appStudioE2EApplicationsNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("snapshot %s has not been found yet\n", applicationSnapshot_push.Name)
					}
					return false
				}, timeout, interval).Should(BeTrue(), "time out when waiting for release created")
			})
		})
	})
})
