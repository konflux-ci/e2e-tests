package integration

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	integrationv1alpha1 "github.com/redhat-appstudio/integration-service/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	containerImageSource   = "quay.io/redhat-appstudio-qe/busybox-loop:latest"
	gitSourceRepoName      = "devfile-sample-python-basic"
	gitSourceURL           = "https://github.com/redhat-appstudio-qe/" + gitSourceRepoName
	BundleURL              = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass"
	BundleURLFail          = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-fail"
	InPipelineName         = "integration-pipeline-pass"
	InPipelineNameFail     = "integration-pipeline-fail"
	EnvironmentName        = "development"
	IntegrationServiceUser = "integration-e2e"
	gitURL                 = "https://github.com/redhat-appstudio/integration-examples.git"
	revision               = "main"
	pathInRepo             = "pipelines/integration_resolver_pipeline_pass.yaml"
)

var _ = framework.IntegrationServiceSuiteDescribe("Integration Service E2E tests", Label("integration-service", "HACBS"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var applicationName, componentName, testNamespace string
	var timeout, interval time.Duration
	var originalComponent *appstudioApi.Component
	var snapshot *appstudioApi.Snapshot
	var snapshot_push *appstudioApi.Snapshot
	var env *appstudioApi.Environment

	Describe("the component with git source (GitHub) is created", Ordered, func() {

		createApp := func() {
			applicationName = fmt.Sprintf("integ-app-%s", util.GenerateRandomString(4))
			testNamespace = f.UserNamespace

			_, err := f.AsKubeAdmin.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Error when creating/updating '%s' namespace: %v", testNamespace, err))

			app, err := f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
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
			cdq, err := f.AsKubeAdmin.HasController.CreateComponentDetectionQuery(componentName, testNamespace, gitSourceURL, "", "", "", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

			for _, compDetected := range cdq.Status.ComponentDetected {
				originalComponent, err = f.AsKubeAdmin.HasController.CreateComponent(compDetected.ComponentStub, testNamespace, "", "", applicationName, true, map[string]string{})
				Expect(err).NotTo(HaveOccurred())
				Expect(originalComponent).NotTo(BeNil())
				componentName = originalComponent.Name
			}
		}

		cleanup := func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.HasController.DeleteApplication(applicationName, testNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.HasController.DeleteComponent(componentName, testNamespace, false)).To(Succeed())
				integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				for _, testScenario := range *integrationTestScenarios {
					Expect(f.AsKubeAdmin.IntegrationController.DeleteIntegrationTestScenario(&testScenario, testNamespace)).To(Succeed())
				}
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}
		}

		assertBuildPipelineRunFinished := func() {
			timeout = time.Minute * 10
			Eventually(func() error {
				pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
				if err != nil {
					GinkgoWriter.Printf("PipelineRun has not been created yet for Component %s/%s\n", testNamespace, componentName)
					return err
				}
				if !pipelineRun.HasStarted() {
					return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
				}
				return nil
			}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, componentName))
			Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(originalComponent, "", 2)).To(Succeed())
		}

		assertSnapshotCreated := func() {
			Eventually(func() error {
				snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", "", componentName, testNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
					return err
				}
				return nil
			}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out when trying to check if the Snapshot for component %s/%s exists", testNamespace, componentName))
		}

		assertIntegrationPipelineRunFinished := func() {
			integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			for _, testScenario := range *integrationTestScenarios {
				timeout = time.Minute * 5
				Eventually(func() error {
					pr, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, snapshot.Name, testNamespace)
					if err != nil {
						GinkgoWriter.Printf("cannot get the Integration PipelineRun: %v\n", err)
						return err
					}
					if !pr.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun for test scenario %s and snapshot %s in %s namespace to start", testScenario.GetName(), snapshot.GetName(), snapshot.GetNamespace()))

				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(&testScenario, snapshot, testNamespace)).To(Succeed(), "Error when waiting for a integration pipeline to finish in %s namespace", testNamespace)
			}
		}

		Describe("with happy path", Ordered, func() {
			BeforeAll(func() {
				// Initialize the tests controllers
				f, err = framework.NewFramework(IntegrationServiceUser)
				Expect(err).NotTo(HaveOccurred())

				createApp()
				createComponent()
				_, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, testNamespace, BundleURL, InPipelineName)
				// create a integrationTestScenario v1beta1 version works also here
				// ex: _, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario_beta1(applicationName, testNamespace, gitURL, revision, pathInRepo)
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					cleanup()

					Expect(f.AsKubeAdmin.IntegrationController.DeleteSnapshot(snapshot_push, testNamespace)).To(Succeed())
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
				_, err = f.AsKubeAdmin.IntegrationController.CreateReleasePlan(applicationName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				env, err = f.AsKubeAdmin.GitOpsController.CreatePocEnvironment(EnvironmentName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				testScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				for _, testScenario := range *testScenarios {
					GinkgoWriter.Printf("IntegrationTestScenario %s is found\n", testScenario.Name)
				}
			})

			It("creates an snapshot of push event", func() {
				sampleImage := "quay.io/redhat-appstudio/sample-image@sha256:841328df1b9f8c4087adbdcfec6cc99ac8308805dea83f6d415d6fb8d40227c1"
				snapshot_push, err = f.AsKubeAdmin.IntegrationController.CreateSnapshot(applicationName, testNamespace, componentName, sampleImage)
				Expect(err).ShouldNot(HaveOccurred())
			})

			When("An snapshot of push event is created", func() {
				It("checks if all of the integrationPipelineRuns created by push event passed", Label("slow"), func() {
					integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, testNamespace)
					Expect(err).ShouldNot(HaveOccurred())

					for _, testScenario := range *integrationTestScenarios {
						GinkgoWriter.Printf("Integration test scenario %s is found\n", testScenario.Name)
						timeout = time.Minute * 5
						Eventually(func() error {
							pr, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, snapshot_push.Name, testNamespace)
							if err != nil {
								GinkgoWriter.Printf("cannot get the Integration PipelineRun: %v\n", err)
								return err
							}
							if !pr.HasStarted() {
								return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
							}
							return nil

						}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun for test scenario %s and snapshot %s in %s namespace to start", testScenario.GetName(), snapshot_push.GetName(), snapshot_push.GetNamespace()))
						Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(&testScenario, snapshot_push, testNamespace)).To(Succeed(), "Error when waiting for a integration pipeline to finish in %s namespace", testNamespace)
					}
				})

				It("checks if the global candidate is updated after push event", func() {
					timeout = time.Second * 600
					interval = time.Second * 10
					Eventually(func() error {
						snapshot_push, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot_push.Name, "", "", testNamespace)
						Expect(err).ShouldNot(HaveOccurred())

						if f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot_push) {
							component, err := f.AsKubeAdmin.HasController.GetComponentByApplicationName(applicationName, testNamespace)
							Expect(err).ShouldNot(HaveOccurred())
							Expect(component.Spec.ContainerImage).ToNot(Equal(originalComponent.Spec.ContainerImage))
							return nil
						}
						return fmt.Errorf("tests haven't succeeded yet for snapshot %s/%s", snapshot_push.GetNamespace(), snapshot_push.GetName())
					}, timeout, interval).Should(Succeed(), fmt.Sprintf("time out when waiting for updating the global candidate in %s namespace", testNamespace))
				})

				It("checks if a Release is created successfully", func() {
					timeout = time.Second * 60
					interval = time.Second * 5
					Eventually(func() error {
						_, err := f.AsKubeAdmin.IntegrationController.GetReleasesWithSnapshot(snapshot_push, testNamespace)
						return err
					}, timeout, interval).Should(Succeed(), fmt.Sprintf("time out when waiting for release created for snapshot %s/%s", snapshot_push.GetNamespace(), snapshot_push.GetName()))
				})

				It("checks if an SnapshotEnvironmentBinding is created successfully", func() {
					timeout = time.Second * 600
					interval = time.Second * 2
					Eventually(func() error {
						_, err := f.AsKubeAdmin.IntegrationController.GetSnapshotEnvironmentBinding(applicationName, testNamespace, env)
						return err
					}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out when waiting for SnapshotEnvironmentBinding to be created for application %s/%s", testNamespace, applicationName))
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

				_, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, testNamespace, BundleURLFail, InPipelineNameFail)
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
				Eventually(func() error {
					snapshot, err := f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", testNamespace)
					if err != nil {
						GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
						return err
					}
					GinkgoWriter.Printf("snapshot %s is found\n", snapshot.Name)
					if !f.AsKubeAdmin.IntegrationController.HaveTestsFinished(snapshot) {
						return fmt.Errorf("tests haven't finished yet for snapshot %s/%s", snapshot.GetNamespace(), snapshot.GetName())
					}
					Expect(f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot)).To(BeFalse(), "expected tests to fail for snapshot %s/%s", snapshot.GetNamespace(), snapshot.GetName())
					return nil
				}, timeout, interval).Should(Succeed(), fmt.Sprintf("time out when trying to check if either the Snapshot %s/%s exists or if the Snapshot is marked as failed", testNamespace, snapshot.GetName()))
			})

			It("checks if the global candidate is not updated", func() {
				// give some time to do eventual updates in component
				time.Sleep(60 * time.Second)

				component, err := f.AsKubeAdmin.HasController.GetComponentByApplicationName(applicationName, testNamespace)
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

				integrationTestScenario_alpha1, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenarioWithEnvironment(applicationName, testNamespace, BundleURL, InPipelineName, EnvironmentName)
				Expect(err).ShouldNot(HaveOccurred())
				env, err = f.AsKubeAdmin.GitOpsController.CreatePocEnvironment(EnvironmentName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					cleanup()

					Expect(f.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(testNamespace, 30*time.Second)).To(Succeed())
					Expect(f.AsKubeAdmin.IntegrationController.DeleteSnapshot(snapshot_push, testNamespace)).To(BeNil())
				}
			})

			It("valid deploymentTargetClass doesn't exist", func() {
				validDTCLS, err := f.AsKubeAdmin.GitOpsController.HaveAvailableDeploymentTargetClassExist()
				Expect(validDTCLS).To(BeNil())
				Expect(err).To(BeNil())
			})

			It("creates a snapshot of push event", func() {
				sampleImage := "quay.io/redhat-appstudio/sample-image@sha256:841328df1b9f8c4087adbdcfec6cc99ac8308805dea83f6d415d6fb8d40227c1"
				snapshot_push, err = f.AsKubeAdmin.IntegrationController.CreateSnapshot(applicationName, testNamespace, componentName, sampleImage)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("snapshot %s is found\n", snapshot_push.Name)
			})

			When("nonexisting valid deploymentTargetClass", func() {
				It("check no GitOpsCR is created for the dtc with nonexisting deploymentTargetClass", func() {
					spaceRequestList, err := f.AsKubeAdmin.IntegrationController.GetSpaceRequests(testNamespace)
					Expect(err).To(BeNil())
					Expect(len(spaceRequestList.Items) > 0).To(BeFalse())

					deploymentTargetList, err := f.AsKubeAdmin.GitOpsController.GetDeploymentTargetsList(testNamespace)
					Expect(err).To(BeNil())
					Expect(len(deploymentTargetList.Items) > 0).To(BeFalse())

					deploymentTargetClaimList, err := f.AsKubeAdmin.GitOpsController.GetDeploymentTargetClaimsList(testNamespace)
					Expect(err).To(BeNil())
					Expect(len(deploymentTargetClaimList.Items) > 0).To(BeFalse())

					environmentList, err := f.AsKubeAdmin.GitOpsController.GetEnvironmentsList(testNamespace)
					Expect(err).To(BeNil())
					Expect(len(environmentList.Items) > 1).To(BeFalse())

					pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario_alpha1.Name, snapshot_push.Name, testNamespace)
					Expect(pipelineRun.Name == "" && strings.Contains(err.Error(), "no pipelinerun found")).To(BeTrue())
				})
				It("checks if snapshot is not marked as passed", func() {
					snapshot, err := f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot_push.Name, "", "", testNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot)).To(BeFalse())
				})
			})
		})
	})
})
