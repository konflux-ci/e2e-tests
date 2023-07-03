package e2e

import (
	"fmt"
	"time"

	"github.com/devfile/library/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	e2eConfig "github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

var _ = framework.E2ESuiteDescribe(Label("e2e-demo"), func() {
	defer GinkgoRecover()

	var timeout, interval time.Duration
	var namespace string

	// Initialize the application struct
	application := &appservice.Application{}
	component := &appservice.Component{}
	snapshot := &appservice.Snapshot{}
	env := &appservice.Environment{}
	fw := &framework.Framework{}

	// Initialize the e2e demo configuration
	configTestFile := viper.GetString("config-suites")
	GinkgoWriter.Printf("Starting e2e-demo test suites from config: %s\n", configTestFile)

	configTest, err := e2eConfig.LoadTestGeneratorConfig(configTestFile)
	Expect(err).NotTo(HaveOccurred())

	for _, appTest := range configTest.Tests {
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
						Expect(fw.AsKubeAdmin.HasController.DeleteAllSnapshotEnvBindingsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.ReleaseController.DeleteAllSnapshotsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(namespace)).To(Succeed())
						Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllGitOpsDeploymentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
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

				for _, componentTest := range appTest.Components {
					componentTest := componentTest
					cdq := &appservice.ComponentDetectionQuery{}
					var secret string

					if componentTest.Type == "private" {
						secret = SPIGithubSecretName
						It(fmt.Sprintf("injects manually SPI token for component %s", componentTest.Name), func() {
							// Inject spi tokens to work with private components
							if componentTest.ContainerSource != "" {
								// More info about manual token upload for quay.io here: https://github.com/redhat-appstudio/service-provider-integration-operator/pull/115
								oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "") + `", "username":"` + utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "") + `"}`

								_ = fw.AsKubeAdmin.SPIController.InjectManualSPIToken(namespace, componentTest.ContainerSource, oauthCredentials, v1.SecretTypeDockerConfigJson, SPIQuaySecretName)
							}
						})
					}

					It(fmt.Sprintf("creates componentdetectionquery for component %s", componentTest.Name), func() {
						cdq, err = fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(componentTest.Name, namespace, componentTest.GitSourceUrl, componentTest.GitSourceRevision, componentTest.GitSourceContext, secret, false)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")
					})

					// Components for now can be imported from gitUrl, container image or a devfile
					if componentTest.GitSourceUrl != "" {
						It(fmt.Sprintf("creates component %s from %s git source %s", componentTest.Name, componentTest.Type, componentTest.GitSourceUrl), func() {
							for _, compDetected := range cdq.Status.ComponentDetected {
								component, err = fw.AsKubeDeveloper.HasController.CreateComponent(compDetected.ComponentStub, namespace, "", secret, appTest.ApplicationName, true, map[string]string{})
								// Workaround until https://issues.redhat.com/browse/RHTAPBUGS-441 is resolved
								if err != nil && errors.IsAlreadyExists(err) {
									compDetected.ComponentStub.ComponentName = fmt.Sprintf("%s-%s", compDetected.ComponentStub.ComponentName, util.GenerateRandomString(4))
									component, err = fw.AsKubeDeveloper.HasController.CreateComponent(compDetected.ComponentStub, namespace, "", secret, appTest.ApplicationName, true, map[string]string{})
								}
								Expect(err).NotTo(HaveOccurred())

							}
						})
					} else {
						defer GinkgoRecover()
						Fail("Please Provide a valid test sample")
					}

					// Start to watch the pipeline until is finished
					It(fmt.Sprintf("waits %s component %s pipeline to be finished", componentTest.Type, componentTest.Name), func() {
						if componentTest.ContainerSource != "" {
							Skip(fmt.Sprintf("component %s was imported from quay.io/docker.io source. Skipping pipelinerun check.", componentTest.Name))
						}
						component, err = fw.AsKubeAdmin.HasController.GetComponent(component.Name, namespace)
						Expect(err).ShouldNot(HaveOccurred(), "failed to get component: %v", err)

						Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", 2)).To(Succeed())
					})

					It("finds the snapshot and checks if it is marked as successful", func() {
						timeout = time.Second * 600
						interval = time.Second * 10

						Eventually(func() error {
							snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", "", component.Name, namespace)
							if err != nil {
								GinkgoWriter.Println("snapshot has not been found yet")
								return err
							}
							if !fw.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot) {
								return fmt.Errorf("tests haven't succeeded for snapshot %s/%s. snapshot status: %+v", snapshot.GetNamespace(), snapshot.GetName(), snapshot.Status)
							}
							return nil
						}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out waiting for the snapshot for the component %s/%s to be marked as successful", component.GetNamespace(), component.GetName()))
					})

					It("checks if a SnapshotEnvironmentBinding is created successfully", func() {
						Eventually(func() error {
							_, err := fw.AsKubeAdmin.IntegrationController.GetSnapshotEnvironmentBinding(application.Name, namespace, env)
							if err != nil {
								GinkgoWriter.Println("SnapshotEnvironmentBinding has not been found yet")
								return err
							}
							return nil
						}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out waiting for the SnapshotEnvironmentBinding to be created (snapshot: %s, env: %s, namespace: %s)", snapshot.GetName(), env.GetName(), snapshot.GetNamespace()))
					})

					// Deploy the component using gitops and check for the health
					if !componentTest.SkipDeploymentCheck {
						var expectedReplicas int32 = 1
						It(fmt.Sprintf("deploys component %s successfully using gitops", componentTest.Name), func() {
							var deployment *appsv1.Deployment
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
							Expect(err).NotTo(HaveOccurred())
						})

						It(fmt.Sprintf("checks if component %s endpoint is healthy", componentTest.Name), func() {
							Eventually(func() error {
								gitOpsRoute, err := fw.AsKubeDeveloper.CommonController.GetOpenshiftRouteByComponentName(component.Name, namespace)
								Expect(err).NotTo(HaveOccurred())
								err = fw.AsKubeDeveloper.CommonController.RouteEndpointIsAccessible(gitOpsRoute, componentTest.HealthEndpoint)
								if err != nil {
									GinkgoWriter.Printf("Failed to request component endpoint: %+v\n retrying...\n", err)
									return err
								}
								return nil
							}, 5*time.Minute, 10*time.Second).Should(Succeed())
						})
					}

					if componentTest.K8sSpec != (e2eConfig.K8sSpec{}) && componentTest.K8sSpec.Replicas > 1 {
						It(fmt.Sprintf("scales component %s replicas", componentTest.Name), Pending, func() {
							component, err := fw.AsKubeDeveloper.HasController.GetComponent(component.Name, namespace)
							Expect(err).NotTo(HaveOccurred())
							_, err = fw.AsKubeDeveloper.HasController.ScaleComponentReplicas(component, pointer.Int(int(componentTest.K8sSpec.Replicas)))
							Expect(err).NotTo(HaveOccurred())
							var deployment *appsv1.Deployment

							Eventually(func() error {
								deployment, err = fw.AsKubeDeveloper.CommonController.GetDeployment(component.Name, namespace)
								Expect(err).NotTo(HaveOccurred())
								if deployment.Status.AvailableReplicas != componentTest.K8sSpec.Replicas {
									return fmt.Errorf("the deployment %s/%s does not have the expected amount of replicas (expected: %d, got: %d)", deployment.GetNamespace(), deployment.GetName(), componentTest.K8sSpec.Replicas, deployment.Status.AvailableReplicas)
								}
								return nil
							}, 5*time.Minute, 10*time.Second).Should(Succeed(), "Component deployment %s/%s didn't get scaled to desired replicas", deployment.GetNamespace(), deployment.GetName())
							Expect(err).NotTo(HaveOccurred())
						})
					}
				}
			})
		}
	}
})
