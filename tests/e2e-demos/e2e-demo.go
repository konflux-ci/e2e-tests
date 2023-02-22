package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	e2eConfig "github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	// Environment name used for e2e-tests demos
	EnvironmentName string = "development"

	// Secret Name created by spi to interact with github
	SPIGithubSecretName string = "e2e-github-secret"

	// Environment name used for e2e-tests demos
	SPIQuaySecretName string = "e2e-quay-secret"

	InitialBuildAnnotationName = "appstudio.openshift.io/component-initial-build"
)

var _ = framework.E2ESuiteDescribe(Label("e2e-demo"), func() {
	defer GinkgoRecover()
	var outputContainerImage = ""
	var timeout, interval time.Duration
	var namespace string

	// Initialize the application struct
	application := &appservice.Application{}
	component := &appservice.Component{}
	snapshot := &appservice.Snapshot{}
	env := &appservice.Environment{}
	cdq := &appservice.ComponentDetectionQuery{}
	fw := &framework.Framework{}

	// Initialize the e2e demo configuration
	configTestFile := viper.GetString("config-suites")
	GinkgoWriter.Printf("Starting e2e-demo test suites from config: %s\n", configTestFile)

	configTest, err := e2eConfig.LoadTestGeneratorConfig(configTestFile)
	Expect(err).NotTo(HaveOccurred())

	for _, appTest := range configTest.Tests {
		appTest := appTest

		Describe(appTest.Name, Ordered, func() {
			BeforeAll(func() {
				if appTest.Skip {
					Skip(fmt.Sprintf("test skipped %s", appTest.Name))
				}

				// Initialize the tests controllers
				fw, err = framework.NewFramework(utils.GetGeneratedNamespace("e2e-demos"))
				Expect(err).NotTo(HaveOccurred())
				namespace = fw.UserNamespace
				Expect(namespace).NotTo(BeEmpty())

				suiteConfig, _ := GinkgoConfiguration()
				GinkgoWriter.Printf("Parallel processes: %d\n", suiteConfig.ParallelTotal)
				GinkgoWriter.Printf("Running on namespace: %s\n", namespace)
				GinkgoWriter.Printf("User: %s\n", fw.UserName)

				githubCredentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`

				_ = fw.AsKubeDeveloper.SPIController.InjectManualSPIToken(namespace, fmt.Sprintf("https://github.com/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")), githubCredentials, v1.SecretTypeBasicAuth, SPIGithubSecretName)
			})

			// Remove all resources created by the tests
			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					Expect(fw.AsKubeDeveloper.HasController.DeleteAllComponentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.HasController.DeleteAllSnapshotEnvBindingsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.ReleaseController.DeleteAllSnapshotsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(namespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllGitOpsDeploymentInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
					Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
				}
			})

			// Create an application in a specific namespace
			It("creates an application", func() {
				GinkgoWriter.Printf("Parallel process %d\n", GinkgoParallelProcess())
				createdApplication, err := fw.AsKubeDeveloper.HasController.CreateHasApplication(appTest.ApplicationName, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdApplication.Spec.DisplayName).To(Equal(appTest.ApplicationName))
				Expect(createdApplication.Namespace).To(Equal(namespace))
			})

			// Check the application health and check if a devfile was generated in the status
			It("checks if application is healthy", func() {
				Eventually(func() string {
					appstudioApp, err := fw.AsKubeDeveloper.HasController.GetHasApplication(appTest.ApplicationName, namespace)
					Expect(err).NotTo(HaveOccurred())
					application = appstudioApp

					return application.Status.Devfile
				}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

				Eventually(func() bool {
					gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

					return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
				}, 5*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
			})

			// Create an environment in a specific namespace
			It("creates an environment", func() {
				env, err = fw.AsKubeDeveloper.IntegrationController.CreateEnvironment(namespace, EnvironmentName)
				Expect(err).NotTo(HaveOccurred())
			})

			for _, componentTest := range appTest.Components {
				componentTest := componentTest
				var containerIMG = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

				if componentTest.Type == "private" {
					It("injects manually SPI token", func() {
						// Inject spi tokens to work with private components
						if componentTest.ContainerSource != "" {
							// More info about manual token upload for quay.io here: https://github.com/redhat-appstudio/service-provider-integration-operator/pull/115
							oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "") + `", "username":"` + utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "") + `"}`

							_ = fw.AsKubeAdmin.SPIController.InjectManualSPIToken(namespace, componentTest.ContainerSource, oauthCredentials, v1.SecretTypeDockerConfigJson, SPIQuaySecretName)
						}
					})
				}

				It("creates componentdetectionquery", func() {
					if componentTest.Type == "private" {
						cdq, err = fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(componentTest.Name, namespace, componentTest.GitSourceUrl, componentTest.GitSourceRevision, componentTest.GitSourceContext, SPIGithubSecretName, false)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

					} else {
						cdq, err = fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(componentTest.Name, namespace, componentTest.GitSourceUrl, componentTest.GitSourceRevision, componentTest.GitSourceContext, "", false)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")
					}
				})

				// Components for now can be imported from gitUrl, container image or a devfile
				if componentTest.ContainerSource != "" {
					It(fmt.Sprintf("creates component %s from %s container source", componentTest.Name, componentTest.Type), func() {
						component, err = fw.AsKubeDeveloper.HasController.CreateComponent(application.Name, componentTest.Name, namespace, "", "", componentTest.ContainerSource, outputContainerImage, SPIQuaySecretName, true)
						Expect(err).NotTo(HaveOccurred())
					})
				} else if componentTest.GitSourceUrl != "" {
					It(fmt.Sprintf("creates component %s from %s git source %s", componentTest.Name, componentTest.Type, componentTest.GitSourceUrl), func() {
						for _, compDetected := range cdq.Status.ComponentDetected {
							if componentTest.Type == "private" {
								component, err = fw.AsKubeDeveloper.HasController.CreateComponentFromStub(compDetected, componentTest.Name, namespace, SPIGithubSecretName, appTest.ApplicationName, containerIMG)
								Expect(err).NotTo(HaveOccurred())
							} else if componentTest.Type == "public" {
								component, err = fw.AsKubeDeveloper.HasController.CreateComponentFromStub(compDetected, componentTest.Name, namespace, "", appTest.ApplicationName, containerIMG)
								Expect(err).NotTo(HaveOccurred())
							}
						}
					})

				} else {
					defer GinkgoRecover()
					Fail("Please Provide a valid test sample")
				}

				// Start to watch the pipeline until is finished
				It(fmt.Sprintf("waits %s component %s pipeline to be finished", componentTest.Type, componentTest.Name), FlakeAttempts(3), func() {
					if componentTest.ContainerSource != "" {
						Skip(fmt.Sprintf("component %s was imported from quay.io/docker.io source. Skipping pipelinerun check.", componentTest.Name))
					}
					component, err = fw.AsKubeAdmin.HasController.GetHasComponent(component.Name, namespace)
					Expect(err).ShouldNot(HaveOccurred(), "failed to get component: %v", err)

					// If we are attempting more than 1 time lets retrigger the pipelinerun
					if CurrentSpecReport().NumAttempts > 1 {
						pipelineRun, err := fw.AsKubeDeveloper.HasController.GetComponentPipelineRun(component.Name, application.Name, namespace, "")
						Expect(err).ShouldNot(HaveOccurred(), "failed to get pipelinerun: %v", err)

						err = fw.AsKubeAdmin.TektonController.DeletePipelineRun(pipelineRun.Name, namespace)
						Expect(err).ShouldNot(HaveOccurred(), "failed to delete pipelinerun when retriger: %v", err)

						delete(component.Annotations, InitialBuildAnnotationName)
						err = fw.AsKubeDeveloper.HasController.KubeRest().Update(context.Background(), component)
						Expect(err).ShouldNot(HaveOccurred(), "failed to update component to trigger another pipeline build: %v", err)
					}

					err := fw.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(fw.AsKubeAdmin.CommonController, component.Name, application.Name, namespace, "")
					if err != nil {
						Fail(fmt.Sprint(err))
					}
				})

				It("finds the snapshot and checks if it is marked as successful", func() {
					timeout = time.Second * 600
					interval = time.Second * 10

					Eventually(func() bool {
						snapshot, err = fw.AsKubeAdmin.IntegrationController.GetApplicationSnapshot("", application.Name, namespace, component.Name)
						if err != nil {
							GinkgoWriter.Println("snapshot has not been found yet")
							return false
						}
						return fw.AsKubeAdmin.IntegrationController.HaveHACBSTestsSucceeded(snapshot)

					}, timeout, interval).Should(BeTrue(), fmt.Sprintf("time out when trying to check if the snapshot %s is marked as successful", snapshot.Name))
				})

				It("checks if a snapshot environment binding is created successfully", func() {
					Eventually(func() bool {
						if fw.AsKubeAdmin.IntegrationController.HaveHACBSTestsSucceeded(snapshot) {
							envbinding, err := fw.AsKubeAdmin.IntegrationController.GetSnapshotEnvironmentBinding(application.Name, namespace, env)
							if err != nil {
								GinkgoWriter.Println("SnapshotEnvironmentBinding has not been found yet")
								return false
							}
							GinkgoWriter.Printf("The SnapshotEnvironmentBinding %s is created\n", envbinding.Name)
							return true
						}
						return false
					}, timeout, interval).Should(BeTrue(), fmt.Sprintf("time out when trying to check if SnapshotEnvironmentBinding is created (snapshot: %s, env: %s)", snapshot.Name, env.Name))
				})

				// Deploy the component using gitops and check for the health
				It(fmt.Sprintf("deploys component %s using gitops", component.Name), func() {
					var deployment *appsv1.Deployment
					Eventually(func() bool {
						deployment, err = fw.AsKubeDeveloper.CommonController.GetAppDeploymentByName(component.Name, namespace)
						if err != nil && !errors.IsNotFound(err) {
							return false
						}
						if deployment.Status.AvailableReplicas == 1 {
							GinkgoWriter.Printf("Deployment %s is ready\n", deployment.Name)
							return true
						}

						return false
					}, 25*time.Minute, 10*time.Second).Should(BeTrue(), fmt.Sprintf("Component deployment didn't become ready: %+v", deployment))
					Expect(err).NotTo(HaveOccurred())
				})

				It(fmt.Sprintf("checks if component %s health", component.Name), func() {
					Eventually(func() bool {
						gitOpsRoute, err := fw.AsKubeDeveloper.CommonController.GetOpenshiftRoute(component.Name, namespace)
						Expect(err).NotTo(HaveOccurred())
						err = fw.AsKubeDeveloper.GitOpsController.CheckGitOpsEndpoint(gitOpsRoute, componentTest.HealthEndpoint)
						if err != nil {
							GinkgoWriter.Println("Failed to request component endpoint. retrying...")
						}
						return true
					}, 5*time.Minute, 10*time.Second).Should(BeTrue())
				})

				if componentTest.K8sSpec != (config.K8sSpec{}) && *componentTest.K8sSpec.Replicas > 1 {
					It(fmt.Sprintf("scales component %s replicas", component.Name), Pending, func() {
						component, err := fw.AsKubeDeveloper.HasController.GetHasComponent(component.Name, namespace)
						Expect(err).NotTo(HaveOccurred())
						_, err = fw.AsKubeDeveloper.HasController.ScaleComponentReplicas(component, int(*componentTest.K8sSpec.Replicas))
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() bool {
							deployment, _ := fw.AsKubeDeveloper.CommonController.GetAppDeploymentByName(component.Name, namespace)
							if err != nil && !errors.IsNotFound(err) {
								return false
							}
							if deployment.Status.AvailableReplicas == *componentTest.K8sSpec.Replicas {
								GinkgoWriter.Printf("Replicas scaled to %s\n", componentTest.K8sSpec.Replicas)
								return true
							}

							return false
						}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "Component deployment didn't get scaled to desired replicas")
						Expect(err).NotTo(HaveOccurred())
					})
				}
			}
		})
	}
})
