package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Application Service controller is deployed the namespace: https://github.com/redhat-appstudio/infra-deployments/blob/main/argo-cd-apps/base/has.yaml#L14
	RedHatAppStudioApplicationNamespace string = "application-service"

	// See more info: https://github.com/redhat-appstudio/application-service#creating-a-github-secret-for-has
	ApplicationServiceGHTokenSecrName string = "has-github-token" // #nosec

	// Name for the GitOps Deployment resource
	GitOpsDeploymentName string = "gitops-deployment-e2e"

	// GitOps repository branch to use
	GitOpsRepositoryRevision string = "main"

	// Name for the Environment resource
	EnvironmentName string = "environment-e2e"

	// Name for the Snapshot resource
	SnapshotName string = "snapshot-e2e"

	// Name for the SnapshotEnvironmentBinding resource
	SnapshotEnvironmentBindingName string = "snapshot-environment-binding-e2e"
)

var _ = framework.E2ESuiteDescribe(Label("e2e-demo"), func() {
	defer GinkgoRecover()
	var outputContainerImage = ""

	// Initialize the application struct
	application := &appservice.Application{}
	component := &appservice.Component{}
	snapshot := &appservice.Snapshot{}

	// Initialize the e2e demo configuration
	configTestFile := viper.GetString("config-suites")
	GinkgoWriter.Printf("Starting e2e-demo test suites from config: %s\n", configTestFile)

	// Initialize the tests controllers
	fw, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())
	configTest, err := e2eConfig.LoadTestGeneratorConfig(configTestFile)
	Expect(err).NotTo(HaveOccurred())

	for _, appTest := range configTest.Tests {
		appTest := appTest
		Describe(appTest.Name, Ordered, func() {
			var namespace = utils.GetGeneratedNamespace("e2e-demo")
			BeforeAll(func() {
				suiteConfig, _ := GinkgoConfiguration()
				GinkgoWriter.Printf("Parallel processes: %d\n", suiteConfig.ParallelTotal)
				GinkgoWriter.Printf("Running on namespace: %s\n", namespace)
				// Check to see if the github token was provided
				Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
				// Check if 'has-github-token' is present, unless SKIP_HAS_SECRET_CHECK env var is set
				if !utils.CheckIfEnvironmentExists(constants.SKIP_HAS_SECRET_CHECK_ENV) {
					_, err := fw.HasController.KubeInterface().CoreV1().Secrets(RedHatAppStudioApplicationNamespace).Get(context.TODO(), ApplicationServiceGHTokenSecrName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred(), "Error checking 'has-github-token' secret %s", err)
				}
				_, err := fw.CommonController.CreateTestNamespace(namespace)
				Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", namespace, err)
			})
			// Remove all resources created by the tests
			AfterAll(func() {
				Expect(fw.HasController.DeleteAllComponentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
				Expect(fw.HasController.DeleteAllApplicationsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
				Expect(fw.HasController.DeleteAllSnapshotEnvBindingsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
				Expect(fw.ReleaseController.DeleteAllSnapshotsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
				Expect(fw.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
				Expect(fw.GitOpsController.DeleteAllGitOpsDeploymentInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
			})

			// Create an application in a specific namespace
			It("application is created", func() {
				GinkgoWriter.Printf("Parallel process %d\n", GinkgoParallelProcess())
				createdApplication, err := fw.HasController.CreateHasApplication(appTest.ApplicationName, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdApplication.Spec.DisplayName).To(Equal(appTest.ApplicationName))
				Expect(createdApplication.Namespace).To(Equal(namespace))
			})

			// Check the application health and check if a devfile was generated in the status
			It("application is healthy", func() {
				Eventually(func() string {
					appstudioApp, err := fw.HasController.GetHasApplication(appTest.ApplicationName, namespace)
					Expect(err).NotTo(HaveOccurred())
					application = appstudioApp

					return application.Status.Devfile
				}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

				Eventually(func() bool {
					gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

					return fw.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
				}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
			})

			It("environment is created", func() {
				createdEnvironment, err := fw.GitOpsController.CreateEnvironment(EnvironmentName, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdEnvironment.Spec.DisplayName).To(Equal(EnvironmentName))
				Expect(createdEnvironment.Namespace).To(Equal(namespace))
			})

			for _, componentTest := range appTest.Components {
				var oauthSecretName = ""

				componentTest := componentTest
				var containerIMG = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

				// TODO: In the future when HAS support creating private applications should push the containers from private repos to a private quay.io repo
				if componentTest.Type == "private" {
					It("Inject manually SPI token", func() {
						// Inject spi tokens to work with private components
						if componentTest.ContainerSource != "" {
							// More info about manual token upload for quay.io here: https://github.com/redhat-appstudio/service-provider-integration-operator/pull/115
							oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "") + `", "username":"` + utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "") + `"}`

							oauthSecretName = fw.SPIController.InjectManualSPIToken(namespace, componentTest.ContainerSource, oauthCredentials, v1.SecretTypeDockerConfigJson)
						} else if componentTest.GitSourceUrl != "" {
							// More info about manual token upload for github.com
							oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`

							oauthSecretName = fw.SPIController.InjectManualSPIToken(namespace, componentTest.GitSourceUrl, oauthCredentials, v1.SecretTypeBasicAuth)
						}
					})
				}

				// Components for now can be imported from gitUrl, container image or a devfile
				if componentTest.ContainerSource != "" {
					It(fmt.Sprintf("create component %s from %s container source", componentTest.Name, componentTest.Type), func() {
						component, err = fw.HasController.CreateComponent(application.Name, componentTest.Name, namespace, "", "", componentTest.ContainerSource, outputContainerImage, oauthSecretName, true)
						Expect(err).NotTo(HaveOccurred())
					})

					// User can define a git url and a devfile at the same time if multiple devfile exists into a repo
				} else if componentTest.GitSourceUrl != "" && componentTest.Devfilesource != "" {
					It(fmt.Sprintf("create component %s from %s git source %s and devfile %s", componentTest.Name, componentTest.Type, componentTest.GitSourceUrl, componentTest.Devfilesource), func() {
						component, err = fw.HasController.CreateComponentFromDevfile(application.Name, componentTest.Name, namespace,
							componentTest.GitSourceUrl, componentTest.Devfilesource, "", containerIMG, oauthSecretName)
						Expect(err).NotTo(HaveOccurred())
					})

					// If component have only a git source application-service will start to fetch the devfile from the git root directory
				} else if componentTest.GitSourceUrl != "" {
					It(fmt.Sprintf("create component %s from %s git source %s", componentTest.Name, componentTest.Type, componentTest.GitSourceUrl), func() {
						component, err = fw.HasController.CreateComponent(application.Name, componentTest.Name, namespace,
							componentTest.GitSourceUrl, "", "", containerIMG, oauthSecretName, true)
						Expect(err).NotTo(HaveOccurred())
					})

				} else {
					defer GinkgoRecover()
					Fail("Please Provide a valid test configuration")
				}

				// Start to watch the pipeline until is finished
				It(fmt.Sprintf("wait %s component %s pipeline to be finished", componentTest.Type, componentTest.Name), func() {
					if componentTest.ContainerSource != "" {
						Skip(fmt.Sprintf("component %s was imported from quay.io/docker.io source. Skipping pipelinerun check.", componentTest.Name))
					}
					Expect(fw.HasController.WaitForComponentPipelineToBeFinished(component.Name, application.Name, namespace)).To(Succeed(), "Failed component pipeline %v", err)
				})

				// Obtain a snapshot for the SnapshotEnvironmentBinding
				if componentTest.ContainerSource != "" {
					It("create snapshot for component imported from quay.io/docker.io source", func() {
						snapshotName := SnapshotName + "-" + util.GenerateRandomString(4)
						snapshotComponents := []appservice.SnapshotComponent{
							{Name: component.Name, ContainerImage: component.Spec.ContainerImage},
						}
						snapshot, err = fw.ReleaseController.CreateSnapshot(snapshotName, namespace, application.Name, snapshotComponents)
						Expect(err).NotTo(HaveOccurred())
					})
				} else {
					It("check if the component's snapshot is created when the pipelinerun is targeted", func() {
						// snapshotName is sent as empty since it is unknown at this stage
						snapshot, err = fw.IntegrationController.GetApplicationSnapshot("", application.Name, namespace, component.Name)
						Expect(err).ShouldNot(HaveOccurred())
					})
				}

				// Create snapshotEnvironmentBinding to cause an application (and its components) to be deployed
				It("snapshotEnvironmentBinding is created", func() {
					snapshotEnvBindingName := SnapshotEnvironmentBindingName + "-" + util.GenerateRandomString(4)
					_, err = fw.HasController.CreateSnapshotEnvironmentBinding(snapshotEnvBindingName, namespace, application.Name, snapshot.Name, EnvironmentName, component)
					Expect(err).NotTo(HaveOccurred())
				})

				// Deploy the component using gitops and check for the health
				It(fmt.Sprintf("deploy component %s using gitops", componentTest.Name), func() {
					var deployment *appsv1.Deployment
					Eventually(func() bool {
						deployment, err = fw.CommonController.GetAppDeploymentByName(componentTest.Name, namespace)
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

				It(fmt.Sprintf("check component %s health", componentTest.Name), func() {
					Eventually(func() bool {
						gitOpsRoute, err := fw.CommonController.GetOpenshiftRoute(componentTest.Name, namespace)
						Expect(err).NotTo(HaveOccurred())
						err = fw.GitOpsController.CheckGitOpsEndpoint(gitOpsRoute, componentTest.HealthEndpoint)
						if err != nil {
							GinkgoWriter.Println("Failed to request component endpoint. retrying...")
						}
						return true
					}, 5*time.Minute, 10*time.Second).Should(BeTrue())
				})

				if componentTest.K8sSpec != (config.K8sSpec{}) && *componentTest.K8sSpec.Replicas > 1 {
					It(fmt.Sprintf("scale component %s replicas", componentTest.Name), Pending, func() {
						component, err := fw.HasController.GetHasComponent(componentTest.Name, namespace)
						Expect(err).NotTo(HaveOccurred())
						_, err = fw.HasController.ScaleComponentReplicas(component, int(*componentTest.K8sSpec.Replicas))
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() bool {
							deployment, _ := fw.CommonController.GetAppDeploymentByName(componentTest.Name, namespace)
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
