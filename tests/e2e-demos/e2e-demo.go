package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	e2eConfig "github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

const (
	// Environment name used for e2e-tests demos
	EnvironmentName string = "development"

	// Secret Name created by spi to interact with github
	SPIGithubSecretName string = "e2e-github-secret"

	// Environment name used for e2e-tests demos
	SPIQuaySecretName string = "e2e-quay-secret"
)

var supportedRuntimes = []string{"Dockerfile", "Node.js", "Go", "Quarkus", "Python", "JavaScript", "springboot", "dotnet"}

var _ = framework.E2ESuiteDescribe(Label("e2e-demo"), func() {
	defer GinkgoRecover()

	var timeout, interval time.Duration
	var namespace string
	var err error

	// Initialize the application struct
	application := &appservice.Application{}
	snapshot := &appservice.Snapshot{}
	env := &appservice.Environment{}
	fw := &framework.Framework{}

	for _, appTest := range e2eConfig.TestScenarios {
		appTest := appTest
		if !appTest.Skip {

			Describe(appTest.Name, Ordered, func() {
				BeforeAll(func() {

					// Initialize the tests controllers
					fw, err = framework.NewFramework(utils.GetGeneratedNamespace("e2e-demos"))
					Expect(err).NotTo(HaveOccurred())
					namespace = fw.UserNamespace
					Expect(namespace).NotTo(BeEmpty())

					// collect SPI ResourceQuota metrics (temporary)
					err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("e2e-demo", namespace, "appstudio-crds-spi")
					Expect(err).NotTo(HaveOccurred())

					suiteConfig, _ := GinkgoConfiguration()
					GinkgoWriter.Printf("Parallel processes: %d\n", suiteConfig.ParallelTotal)
					GinkgoWriter.Printf("Running on namespace: %s\n", namespace)
					GinkgoWriter.Printf("User: %s\n", fw.UserName)

					githubCredentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`

					_ = fw.AsKubeDeveloper.SPIController.InjectManualSPIToken(namespace, fmt.Sprintf("https://github.com/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")), githubCredentials, v1.SecretTypeBasicAuth, SPIGithubSecretName)
				})

				// Remove all resources created by the tests
				AfterAll(func() {
					// collect SPI ResourceQuota metrics (temporary)
					err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("e2e-demo", namespace, "appstudio-crds-spi")
					Expect(err).NotTo(HaveOccurred())

					if !CurrentSpecReport().Failed() {
						Expect(fw.AsKubeDeveloper.HasController.DeleteAllComponentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.CommonController.DeleteAllSnapshotEnvBindingsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.IntegrationController.DeleteAllSnapshotsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(namespace)).To(Succeed())
						Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllGitOpsDeploymentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
					}
				})

				// Create an application in a specific namespace
				It("creates an application", func() {
					GinkgoWriter.Printf("Parallel process %d\n", GinkgoParallelProcess())
					createdApplication, err := fw.AsKubeDeveloper.HasController.CreateApplication(appTest.ApplicationName, namespace)
					Expect(err).NotTo(HaveOccurred())
					Expect(createdApplication.Spec.DisplayName).To(Equal(appTest.ApplicationName))
					Expect(createdApplication.Namespace).To(Equal(namespace))
				})

				// Check the application health and check if a devfile was generated in the status
				It("checks if application is healthy", func() {
					Eventually(func() string {
						appstudioApp, err := fw.AsKubeDeveloper.HasController.GetApplication(appTest.ApplicationName, namespace)
						Expect(err).NotTo(HaveOccurred())
						application = appstudioApp

						return application.Status.Devfile
					}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), fmt.Sprintf("timed out waiting for gitOps repository to be created for the %s application in %s namespace", appTest.ApplicationName, fw.UserNamespace))

					Eventually(func() bool {
						gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

						return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
					}, 1*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("timed out waiting for HAS controller to create gitops repository for the %s application in %s namespace", appTest.ApplicationName, fw.UserNamespace))
				})

				// Create an environment in a specific namespace
				It("creates an environment", func() {
					env, err = fw.AsKubeDeveloper.GitOpsController.CreatePocEnvironment(EnvironmentName, namespace)
					Expect(err).NotTo(HaveOccurred())
				})

				for _, componentSpec := range appTest.Components {
					componentSpec := componentSpec
					cdq := &appservice.ComponentDetectionQuery{}
					componentList := []*appservice.Component{}
					var secret string

					if componentSpec.Private {
						secret = SPIGithubSecretName
						It(fmt.Sprintf("injects manually SPI token for component %s", componentSpec.Name), func() {
							// Inject spi tokens to work with private components
							if componentSpec.ContainerSource != "" {
								// More info about manual token upload for quay.io here: https://github.com/redhat-appstudio/service-provider-integration-operator/pull/115
								oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "") + `", "username":"` + utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "") + `"}`

								_ = fw.AsKubeAdmin.SPIController.InjectManualSPIToken(namespace, componentSpec.ContainerSource, oauthCredentials, v1.SecretTypeDockerConfigJson, SPIQuaySecretName)
							}
						})
					}

					It(fmt.Sprintf("creates componentdetectionquery for component %s", componentSpec.Name), func() {
						cdq, err = fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(componentSpec.Name, namespace, componentSpec.GitSourceUrl, componentSpec.GitSourceRevision, componentSpec.GitSourceContext, secret, false)
						Expect(err).NotTo(HaveOccurred())
					})

					It("check if components have supported languages by AppStudio", func() {
						if appTest.Name == e2eConfig.MultiComponentWithUnsupportedRuntime {
							// Validate that the completed CDQ only has detected 1 component and not also the unsupported component
							Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "cdq also detect unsupported component")
						}
						for _, component := range cdq.Status.ComponentDetected {
							Expect(supportedRuntimes).To(ContainElement(component.ProjectType), "unsupported runtime used for multi component tests")
						}
					})

					// Components for now can be imported from gitUrl, container image or a devfile
					if componentSpec.GitSourceUrl != "" {
						It(fmt.Sprintf("creates component %s (private: %t) from git source %s", componentSpec.Name, componentSpec.Private, componentSpec.GitSourceUrl), func() {
							for _, compDetected := range cdq.Status.ComponentDetected {
								c, err := fw.AsKubeDeveloper.HasController.CreateComponent(compDetected.ComponentStub, namespace, "", secret, appTest.ApplicationName, true, map[string]string{})
								Expect(err).NotTo(HaveOccurred())
								Expect(c.Name).To(Equal(compDetected.ComponentStub.ComponentName))
								Expect(supportedRuntimes).To(ContainElement(compDetected.ProjectType), "unsupported runtime used for multi component tests")

								componentList = append(componentList, c)
							}
						})
					} else {
						defer GinkgoRecover()
						Fail("Please Provide a valid test sample")
					}

					// Start to watch the pipeline until is finished
					It(fmt.Sprintf("waits for %s component (private: %t) pipeline to be finished", componentSpec.Name, componentSpec.Private), func() {
						if componentSpec.ContainerSource != "" {
							Skip(fmt.Sprintf("component %s was imported from quay.io/docker.io source. Skipping pipelinerun check.", componentSpec.Name))
						}
						for _, component := range componentList {
							component, err = fw.AsKubeAdmin.HasController.GetComponent(component.GetName(), namespace)
							Expect(err).ShouldNot(HaveOccurred(), "failed to get component: %v", err)

							Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", 2)).To(Succeed())
						}
					})

					It("finds the snapshot and checks if it is marked as successful", func() {
						timeout = time.Second * 600
						interval = time.Second * 10
						for _, component := range componentList {
							Eventually(func() error {
								snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", "", component.Name, namespace)
								if err != nil {
									GinkgoWriter.Println("snapshot has not been found yet")
									return err
								}
								if !fw.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot) {
									return fmt.Errorf("tests haven't succeeded for snapshot %s/%s. snapshot status: %+v", snapshot.GetNamespace(), snapshot.GetName(), snapshot.Status)
								}
								return nil
							}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out waiting for the snapshot for the component %s/%s to be marked as successful", component.GetNamespace(), component.GetName()))
						}
					})

					It("checks if a SnapshotEnvironmentBinding is created successfully", func() {
						Eventually(func() error {
							_, err := fw.AsKubeAdmin.CommonController.GetSnapshotEnvironmentBinding(application.Name, namespace, env)
							if err != nil {
								GinkgoWriter.Println("SnapshotEnvironmentBinding has not been found yet")
								return err
							}
							return nil
						}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out waiting for the SnapshotEnvironmentBinding to be created (snapshot: %s, env: %s, namespace: %s)", snapshot.GetName(), env.GetName(), snapshot.GetNamespace()))
					})

					// Deploy the component using gitops and check for the health
					if !componentSpec.SkipDeploymentCheck {
						var expectedReplicas int32 = 1
						It(fmt.Sprintf("deploys component %s successfully using gitops", componentSpec.Name), func() {
							var deployment *appsv1.Deployment
							for _, component := range componentList {
								Eventually(func() error {
									deployment, err = fw.AsKubeDeveloper.CommonController.GetDeployment(component.Name, namespace)
									if err != nil {
										return err
									}
									if deployment.Status.AvailableReplicas != expectedReplicas {
										return fmt.Errorf("the deployment %s/%s does not have the expected amount of replicas (expected: %d, got: %d)", deployment.GetNamespace(), deployment.GetName(), expectedReplicas, deployment.Status.AvailableReplicas)
									}
									return nil
								}, 25*time.Minute, 10*time.Second).Should(Succeed(), fmt.Sprintf("timed out waiting for deployment of a component %s/%s to become ready", component.GetNamespace(), component.GetName()))
							}
						})

						It(fmt.Sprintf("checks if component %s route(s) exist and health endpoint (if defined) is reachable", componentSpec.Name), func() {
							for _, component := range componentList {
								Eventually(func() error {
									gitOpsRoute, err := fw.AsKubeDeveloper.CommonController.GetOpenshiftRouteByComponentName(component.Name, namespace)
									Expect(err).NotTo(HaveOccurred())
									if componentSpec.HealthEndpoint != "" {
										err = fw.AsKubeDeveloper.CommonController.RouteEndpointIsAccessible(gitOpsRoute, componentSpec.HealthEndpoint)
										if err != nil {
											GinkgoWriter.Printf("Failed to request component endpoint: %+v\n retrying...\n", err)
											return err
										}
									}
									return nil
								}, 5*time.Minute, 10*time.Second).Should(Succeed())
							}
						})
					}

					if componentSpec.K8sSpec != (e2eConfig.K8sSpec{}) && componentSpec.K8sSpec.Replicas > 1 {
						It(fmt.Sprintf("scales component %s replicas", componentSpec.Name), Pending, func() {
							for _, component := range componentList {
								c, err := fw.AsKubeDeveloper.HasController.GetComponent(component.Name, namespace)
								Expect(err).NotTo(HaveOccurred())
								_, err = fw.AsKubeDeveloper.HasController.ScaleComponentReplicas(c, pointer.Int(int(componentSpec.K8sSpec.Replicas)))
								Expect(err).NotTo(HaveOccurred())
								var deployment *appsv1.Deployment

								Eventually(func() error {
									deployment, err = fw.AsKubeDeveloper.CommonController.GetDeployment(c.Name, namespace)
									Expect(err).NotTo(HaveOccurred())
									if deployment.Status.AvailableReplicas != componentSpec.K8sSpec.Replicas {
										return fmt.Errorf("the deployment %s/%s does not have the expected amount of replicas (expected: %d, got: %d)", deployment.GetNamespace(), deployment.GetName(), componentSpec.K8sSpec.Replicas, deployment.Status.AvailableReplicas)
									}
									return nil
								}, 5*time.Minute, 10*time.Second).Should(Succeed(), "Component deployment %s/%s didn't get scaled to desired replicas", deployment.GetNamespace(), deployment.GetName())
								Expect(err).NotTo(HaveOccurred())
							}
						})
					}
				}
			})
		}
	}
})
