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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
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
)

var _ = framework.E2ESuiteDescribe(Label("e2e-demo"), func() {
	defer GinkgoRecover()
	var outputContainerImage = ""

	// Initialize the application struct
	application := &appservice.Application{}
	component := &appservice.Component{}

	// Initialize the e2e demo configuration
	configTestFile := viper.GetString("config-suites")
	klog.Infof("Starting e2e-demo test suites from config: %s", configTestFile)

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
				fmt.Printf("Parallel processes: %d", suiteConfig.ParallelTotal)
				fmt.Printf("Running on namespace: %s", namespace)
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
				Expect(fw.GitOpsController.DeleteAllGitOpsDeploymentInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
			})

			// Create an application in a specific namespace
			It("application is created", func() {
				fmt.Printf("Parallel process %d", GinkgoParallelProcess())
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
						_, err := fw.HasController.CreateComponent(application.Name, componentTest.Name, namespace, "", "", componentTest.ContainerSource, outputContainerImage, oauthSecretName)
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
							componentTest.GitSourceUrl, "", "", containerIMG, oauthSecretName)
						Expect(err).NotTo(HaveOccurred())
					})

				} else {
					defer GinkgoRecover()
					Fail("Please Provide a valid test configuration")
				}

				// Start to watch the pipeline until is finished
				It(fmt.Sprintf("wait %s component %s pipeline to be finished", componentTest.Type, componentTest.Name), func() {
					if componentTest.ContainerSource != "" {
						Skip(fmt.Sprintf("component %s was imported from quay.io/docker.io source. Skiping pipelinerun check.", componentTest.Name))
					}
					Expect(fw.HasController.WaitForComponentPipelineToBeFinished(component.Name, application.Name, namespace)).To(Succeed(), "Failed component pipeline %v", err)
				})

				// Deploy the component using gitops and check for the health
				// TODO re-enable once the issue with GitopsDeployment creation is resolved
				It(fmt.Sprintf("deploy component %s using gitops", componentTest.Name), Pending, func() {

					Eventually(func() bool {
						deployment, err := fw.CommonController.GetAppDeploymentByName(componentTest.Name, namespace)
						if err != nil && !errors.IsNotFound(err) {
							return false
						}
						if deployment.Status.AvailableReplicas == 1 {
							klog.Infof("Deployment %s is ready", deployment.Name)
							return true
						}

						return false
					}, 15*time.Minute, 10*time.Second).Should(BeTrue(), "Component deployment didn't become ready")
					Expect(err).NotTo(HaveOccurred())
				})

				It(fmt.Sprintf("check component %s health", componentTest.Name), Pending, func() {
					Eventually(func() bool {
						gitOpsRoute, err := fw.CommonController.GetOpenshiftRoute(componentTest.Name, namespace)
						Expect(err).NotTo(HaveOccurred())
						err = fw.GitOpsController.CheckGitOpsEndpoint(gitOpsRoute, componentTest.HealthEndpoint)
						if err != nil {
							klog.Info("Failed to request component endpoint. retrying...")
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
								klog.Infof("Replicas scaled to %s ", componentTest.K8sSpec.Replicas)
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
