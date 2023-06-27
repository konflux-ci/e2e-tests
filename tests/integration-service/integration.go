package integration

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"knative.dev/pkg/apis"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	integrationv1alpha1 "github.com/redhat-appstudio/integration-service/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	containerImageSource   = "quay.io/redhat-appstudio-qe/busybox-loop@sha256:f698f1f2cf641fe9176d2a277c9052d872f6b1c39e56248a1dd259b96281dda9"
	gitSourceRepoName      = "devfile-sample-python-basic"
	gitSourceURL           = "https://github.com/redhat-appstudio-qe/" + gitSourceRepoName
	BundleURL              = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass"
	BundleURLFail          = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-fail"
	InPipelineName         = "integration-pipeline-pass"
	InPipelineNameFail     = "integration-pipeline-fail"
	EnvironmentName        = "development"
	IntegrationServiceUser = "integration-e2e"
	gitURL                 = "https://github.com/redhat-appstudio/integration-examples.git"
	revision               = "843f455fe87a6d7f68c238f95a8f3eb304e65ac5"
	pathInRepo             = "pipelines/integration_resolver_pipeline_pass.yaml"
)

var _ = framework.IntegrationServiceSuiteDescribe("Integration Service E2E tests", Label("integration-service", "HACBS"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var applicationName, componentName, appStudioE2EApplicationsNamespace string
	var timeout, interval time.Duration
	var originalComponent *appstudioApi.Component
	var snapshot *appstudioApi.Snapshot
	var snapshot_push *appstudioApi.Snapshot
	var env *appstudioApi.Environment

	Describe("the component with git source (GitHub) is created", Ordered, func() {

		createApp := func() {
			applicationName = fmt.Sprintf("integ-app-%s", util.GenerateRandomString(4))
			appStudioE2EApplicationsNamespace = f.UserNamespace

			_, err := f.AsKubeAdmin.CommonController.CreateTestNamespace(appStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", appStudioE2EApplicationsNamespace, err)

			app, err := f.AsKubeAdmin.HasController.CreateApplication(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.AsKubeAdmin.HasController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)
		}

		createComponent := func() {
			componentName = fmt.Sprintf("integration-suite-test-component-git-source-%s", util.GenerateRandomString(4))
			timeout = time.Minute * 4
			interval = time.Second * 1
			// Create a component with Git Source URL being defined
			// using cdq since git ref is not known
			cdq, err := f.AsKubeAdmin.HasController.CreateComponentDetectionQuery(componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", "", "", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

			for _, compDetected := range cdq.Status.ComponentDetected {
				originalComponent, err = f.AsKubeAdmin.HasController.CreateComponent(compDetected.ComponentStub, appStudioE2EApplicationsNamespace, "", "", applicationName, true, map[string]string{})
				Expect(err).NotTo(HaveOccurred())
				Expect(originalComponent).NotTo(BeNil())
				componentName = originalComponent.Name
			}
		}

		cleanup := func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.HasController.DeleteApplication(applicationName, appStudioE2EApplicationsNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.HasController.DeleteComponent(componentName, appStudioE2EApplicationsNamespace, false)).To(Succeed())
				integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				for _, testScenario := range *integrationTestScenarios {
					Expect(f.AsKubeAdmin.IntegrationController.DeleteIntegrationTestScenario(&testScenario, appStudioE2EApplicationsNamespace)).To(Succeed())
				}
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}
		}

		assertBuildPipelineRunFinished := func() {
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
			Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(originalComponent, "", 2)).To(Succeed())
		}

		assertSnapshotCreated := func() {
			Eventually(func() bool {
				// snapshotName is sent as empty since it is unknown at this stage
				snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", "", componentName, appStudioE2EApplicationsNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
					return false
				}

				GinkgoWriter.Printf("snapshot %s is found\n", snapshot.Name)
				return true
			}, timeout, interval).Should(BeTrue(), "timed out when trying to check if the Snapshot exists")
		}

		assertIntegrationPipelineRunFinished := func() {
			integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			for _, testScenario := range *integrationTestScenarios {
				timeout = time.Minute * 5
				interval = time.Second * 2
				Eventually(func() bool {
					pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, snapshot.Name, appStudioE2EApplicationsNamespace)
					if err != nil {
						GinkgoWriter.Printf("cannot get the Integration PipelineRun: %v\n", err)
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(&testScenario, snapshot, appStudioE2EApplicationsNamespace)).To(Succeed(), "Error when waiting for a integration pipeline to finish")
			}
		}

		Describe("with happy path", Ordered, func() {
			BeforeAll(func() {
				// Initialize the tests controllers
				f, err = framework.NewFramework(IntegrationServiceUser)
				Expect(err).NotTo(HaveOccurred())

				createApp()
				createComponent()
				_, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, appStudioE2EApplicationsNamespace, BundleURL, InPipelineName)
				// create a integrationTestScenario v1beta1 version works also here
				// ex: _, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario_beta1(applicationName, appStudioE2EApplicationsNamespace, gitURL, revision, pathInRepo)
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					cleanup()

					err = f.AsKubeAdmin.IntegrationController.DeleteSnapshot(snapshot_push, appStudioE2EApplicationsNamespace)
					Expect(err).ShouldNot(HaveOccurred())
				}
			})

			It("triggers a build PipelineRun", Label("integration-service"), func() {
				assertBuildPipelineRunFinished()
			})

			When("the build pipelineRun run succeeded", func() {
				It("checks if the Snapshot is created", func() {
					assertSnapshotCreated()
				})

				It("checks if all of the integrationPipelineRuns passed", Label("slow"), func() {
					assertIntegrationPipelineRunFinished()
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

			It("creates an snapshot of push event", func() {
				sampleImage := "quay.io/redhat-appstudio/sample-image@sha256:841328df1b9f8c4087adbdcfec6cc99ac8308805dea83f6d415d6fb8d40227c1"
				snapshot_push, err = f.AsKubeAdmin.IntegrationController.CreateSnapshot(applicationName, appStudioE2EApplicationsNamespace, componentName, sampleImage)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("snapshot %s is found\n", snapshot_push.Name)
			})

			When("An snapshot of push event is created", func() {
				It("checks if all of the integrationPipelineRuns created by push event passed", Label("slow"), func() {
					integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
					Expect(err).ShouldNot(HaveOccurred())

					for _, testScenario := range *integrationTestScenarios {
						GinkgoWriter.Printf("Integration test scenario %s is found\n", testScenario.Name)
						timeout = time.Minute * 5
						interval = time.Second * 2
						Eventually(func() bool {
							pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, snapshot_push.Name, appStudioE2EApplicationsNamespace)
							if err != nil {
								GinkgoWriter.Printf("cannot get the Integration PipelineRun: %v\n", err)
								return false
							}
							return pipelineRun.HasStarted()

						}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
						timeout = time.Second * 600
						interval = time.Second * 10
						Eventually(func() bool {
							pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, snapshot_push.Name, appStudioE2EApplicationsNamespace)
							Expect(err).ShouldNot(HaveOccurred())

							for _, condition := range pipelineRun.Status.Conditions {
								GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)
								if !pipelineRun.IsDone() {
									return false
								}

								if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
									failMessage, err := tekton.GetFailedPipelineRunLogs(f.AsKubeAdmin.CommonController.KubeRest(), f.AsKubeAdmin.CommonController.KubeInterface(), pipelineRun)
									if err != nil {
										GinkgoWriter.Printf("failed to get logs for pipelinerun %s: %+v\n", pipelineRun.Name, err)
									}
									Fail(failMessage)
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
						if f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot_push) {
							component, err := f.AsKubeAdmin.IntegrationController.GetComponent(applicationName, appStudioE2EApplicationsNamespace)
							if err != nil {
								GinkgoWriter.Println("component has not been found yet")
								return false
							}
							Expect(component.Spec.ContainerImage != originalComponent.Spec.ContainerImage).To(BeTrue())
							GinkgoWriter.Printf("Global candidate is updated\n")
							return true
						}
						snapshot_push, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot_push.Name, "", "", appStudioE2EApplicationsNamespace)
						if err != nil {
							GinkgoWriter.Printf("snapshot %s has not been found yet\n", snapshot_push.Name)
						}
						return false
					}, timeout, interval).Should(BeTrue(), "time out when waiting for updating the global candidate")
				})

				It("checks if a Release is created successfully", func() {
					timeout = time.Second * 800
					interval = time.Second * 10
					Eventually(func() bool {
						if f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot_push) {
							releases, err := f.AsKubeAdmin.IntegrationController.GetReleasesWithSnapshot(snapshot_push, appStudioE2EApplicationsNamespace)
							Expect(err).ShouldNot(HaveOccurred())
							if len(releases) != 0 {
								for _, release := range releases {
									GinkgoWriter.Printf("Release %s is found\n", release.Name)
								}
							} else {
								Fail("No Release found")
							}
							return true
						}
						snapshot_push, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot_push.Name, "", "", appStudioE2EApplicationsNamespace)
						if err != nil {
							GinkgoWriter.Printf("snapshot %s has not been found yet\n", snapshot_push.Name)
						}
						return false
					}, timeout, interval).Should(BeTrue(), "time out when waiting for release created")
				})

				It("checks if an EnvironmentBinding is created successfully", func() {
					timeout = time.Second * 600
					interval = time.Second * 2
					Eventually(func() bool {
						if f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot_push) {
							envbinding, err := f.AsKubeAdmin.IntegrationController.GetSnapshotEnvironmentBinding(applicationName, appStudioE2EApplicationsNamespace, env)
							Expect(err).ShouldNot(HaveOccurred())
							Expect(envbinding != nil).To(BeTrue())
							GinkgoWriter.Printf("The EnvironmentBinding is created\n")
							return true
						}
						snapshot_push, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot_push.Name, "", "", appStudioE2EApplicationsNamespace)
						if err != nil {
							GinkgoWriter.Printf("snapshot %s has not been found yet\n", snapshot_push.Name)
						}
						return false
					}, timeout, interval).Should(BeTrue(), "time out when waiting for release created")
				})
			})
		})

		Describe("with an integration test fail", Ordered, func() {
			BeforeAll(func() {
				// Initialize the tests controllers
				f, err = framework.NewFramework(IntegrationServiceUser)
				Expect(err).NotTo(HaveOccurred())

				createApp()
				createComponent()
				_, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, appStudioE2EApplicationsNamespace, BundleURLFail, InPipelineNameFail)
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					cleanup()
				}
			})

			It("triggers a build PipelineRun", Label("integration-service"), func() {
				assertBuildPipelineRunFinished()
			})

			It("checks if the Snapshot is created", func() {
				assertSnapshotCreated()
			})

			It("checks if all of the integrationPipelineRuns finished", Label("slow"), func() {
				assertIntegrationPipelineRunFinished()
			})

			It("checks if snapshot is marked as failed", func() {
				Eventually(func() bool {
					snapshot, err := f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", appStudioE2EApplicationsNamespace)
					if err != nil {
						GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
						return false
					}

					GinkgoWriter.Printf("snapshot %s is found\n", snapshot.Name)
					return !f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot)
				}, timeout, interval).Should(BeTrue(), "time out when trying to check if either the Snapshot exists or if the Snapshot is marked as failed")
			})

			It("checks if the global candidate is not updated", func() {
				// give some time to do eventual updates in component
				time.Sleep(60 * time.Second)

				component, err := f.AsKubeAdmin.IntegrationController.GetComponent(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				// global candidate is not updated
				Expect(component.Spec.ContainerImage == originalComponent.Spec.ContainerImage).To(BeTrue())

			})
		})

		Describe("valid dtcls doesn't exist", Ordered, func() {
			var integrationTestScenario_alpha1 *integrationv1alpha1.IntegrationTestScenario
			BeforeAll(func() {
				// Initialize the tests controllers
				f, err = framework.NewFramework(IntegrationServiceUser)
				Expect(err).NotTo(HaveOccurred())

				createApp()
				createComponent()

				integrationTestScenario_alpha1, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenarioWithEnvironment(applicationName, appStudioE2EApplicationsNamespace, BundleURL, InPipelineName, EnvironmentName)
				Expect(err).ShouldNot(HaveOccurred())
				env, err = f.AsKubeAdmin.IntegrationController.CreateEnvironment(appStudioE2EApplicationsNamespace, EnvironmentName)
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					cleanup()

					Expect(f.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(appStudioE2EApplicationsNamespace, 30*time.Second)).To(Succeed())
					Expect(f.AsKubeAdmin.IntegrationController.DeleteSnapshot(snapshot_push, appStudioE2EApplicationsNamespace)).To(BeNil())
				}
			})

			It("valid deploymentTargetClass doesn't exist", func() {
				validDTCLS, err := f.AsKubeAdmin.IntegrationController.HaveAvailableDeploymentTargetClassExist()
				Expect(validDTCLS).To(BeNil())
				Expect(err).To(BeNil())
			})

			It("creates a snapshot of push event", func() {
				sampleImage := "quay.io/redhat-appstudio/sample-image@sha256:841328df1b9f8c4087adbdcfec6cc99ac8308805dea83f6d415d6fb8d40227c1"
				snapshot_push, err = f.AsKubeAdmin.IntegrationController.CreateSnapshot(applicationName, appStudioE2EApplicationsNamespace, componentName, sampleImage)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("snapshot %s is found\n", snapshot_push.Name)
			})

			When("nonexisting valid deploymentTargetClass", func() {
				It("check no GitOpsCR is created for the dtc with nonexisting deploymentTargetClass", func() {
					spaceRequestList, err := f.AsKubeAdmin.IntegrationController.GetSpaceRequests(appStudioE2EApplicationsNamespace)
					Expect(err).To(BeNil())
					Expect(len(spaceRequestList.Items) > 0).To(BeFalse())

					deploymentTargetList, err := f.AsKubeAdmin.IntegrationController.GetDeploymentTargets(appStudioE2EApplicationsNamespace)
					Expect(err).To(BeNil())
					Expect(len(deploymentTargetList.Items) > 0).To(BeFalse())

					deploymentTargetClaimList, err := f.AsKubeAdmin.IntegrationController.GetDeploymentTargetClaims(appStudioE2EApplicationsNamespace)
					Expect(err).To(BeNil())
					Expect(len(deploymentTargetClaimList.Items) > 0).To(BeFalse())

					environmentList, err := f.AsKubeAdmin.IntegrationController.GetEnvironments(appStudioE2EApplicationsNamespace)
					Expect(err).To(BeNil())
					Expect(len(environmentList.Items) > 1).To(BeFalse())

					pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario_alpha1.Name, snapshot_push.Name, appStudioE2EApplicationsNamespace)
					Expect(pipelineRun.Name == "" && strings.Contains(err.Error(), "no pipelinerun found")).To(BeTrue())
				})
				It("checks if snapshot is not marked as passed", func() {
					snapshot, err := f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot_push.Name, "", "", appStudioE2EApplicationsNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot)).To(BeFalse())
				})
			})
		})
	})
})
