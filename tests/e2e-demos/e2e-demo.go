package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var AppStudioE2EApplicationsNamespace = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, "appstudio-e2e-test")

var _ = framework.E2ESuiteDescribe(Label("e2e-demo"), func() {
	defer GinkgoRecover()

	// Initialize the application struct
	application := &appservice.Application{}
	component := &appservice.Component{}

	// Initialize the e2e demo configuration
	configTestFile := viper.GetString("config-suites")
	klog.Infof("Starting e2e-demo test suites from config: %s", configTestFile)

	// Initialize the tests controllers
	fw, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())
	configTest, err := LoadTestGeneratorConfig(configTestFile)
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
		// Check if 'has-github-token' is present, unless SKIP_HAS_SECRET_CHECK env var is set
		if !utils.CheckIfEnvironmentExists(constants.SKIP_HAS_SECRET_CHECK_ENV) {
			_, err := fw.HasController.KubeInterface().CoreV1().Secrets(RedHatAppStudioApplicationNamespace).Get(context.TODO(), ApplicationServiceGHTokenSecrName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Error checking 'has-github-token' secret %s", err)
		}

		_, err := fw.CommonController.CreateTestNamespace(AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", AppStudioE2EApplicationsNamespace, err)
	})

	// Remove all resources created by the tests
	AfterAll(func() {
		Expect(fw.HasController.DeleteAllComponentsInASpecificNamespace(AppStudioE2EApplicationsNamespace)).To(Succeed())
		Expect(fw.HasController.DeleteAllApplicationsInASpecificNamespace(AppStudioE2EApplicationsNamespace)).To(Succeed())
		Expect(fw.GitOpsController.DeleteAllGitOpDeploymentsCRsInASpecificNamespace(AppStudioE2EApplicationsNamespace)).To(Succeed())
	})

	for _, appTest := range configTest.Tests {
		appTest := appTest

		When(appTest.Name, func() {
			// Create an application in a specific namespace
			It("application is created", func() {
				createdApplication, err := fw.HasController.CreateHasApplication(appTest.ApplicationName, AppStudioE2EApplicationsNamespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdApplication.Spec.DisplayName).To(Equal(appTest.ApplicationName))
				Expect(createdApplication.Namespace).To(Equal(AppStudioE2EApplicationsNamespace))
			})

			// Check the application health and check if a devfile was generated in the status
			It("application is healthy", func() {
				Eventually(func() string {
					appstudioApp, err := fw.HasController.GetHasApplication(appTest.ApplicationName, AppStudioE2EApplicationsNamespace)
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
				componentTest := componentTest
				var containerIMG = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
				// Fail all private tests
				if componentTest.Type == "private" {
					defer GinkgoRecover()
					Fail("Component creation from private repo is not supported. Jira issue: https://issues.redhat.com/browse/SVPI-135")
				}

				// Components for now can be imported from gitUrl, container image or a devfile
				if componentTest.ContainerSource != "" {
					It(fmt.Sprintf("create component %s from container source", componentTest.Name), func() {
						var outputContainerImage = ""
						_, err := fw.HasController.CreateComponent(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace, "", "", componentTest.ContainerSource, outputContainerImage, "")
						Expect(err).NotTo(HaveOccurred())
					})

				} else if componentTest.GitSourceUrl != "" && componentTest.Devfilesource != "" {
					It(fmt.Sprintf("create component %s from git source %s and devfile %s", componentTest.Name, componentTest.GitSourceUrl, componentTest.Devfilesource), func() {
						component, err = fw.HasController.CreateComponentFromDevfile(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace,
							componentTest.GitSourceUrl, componentTest.Devfilesource, "", containerIMG, "")
						Expect(err).NotTo(HaveOccurred())
					})

				} else if componentTest.GitSourceUrl != "" {
					It(fmt.Sprintf("create component %s from git source %s", componentTest.Name, componentTest.GitSourceUrl), func() {
						component, err = fw.HasController.CreateComponent(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace,
							componentTest.GitSourceUrl, "", "", containerIMG, "")
						Expect(err).NotTo(HaveOccurred())
					})

				} else {
					defer GinkgoRecover()
					Fail("Please Provide a valid test configuration")
				}

				It(fmt.Sprintf("wait component %s pipeline to be finished", componentTest.Name), func() {
					if componentTest.ContainerSource != "" {
						Skip(fmt.Sprintf("component %s was imported from quay.io/docker.io source. Skiping pipelinerun check.", componentTest.Name))
					}
					Expect(fw.HasController.WaitForComponentPipelineToBeFinished(component.Name, application.Name, AppStudioE2EApplicationsNamespace)).To(Succeed(), "Failed component pipeline %v", err)
				})

				// Deploy the component using gitops and check for the health
				It(fmt.Sprintf("deploy component %s using gitops", componentTest.Name), func() {

					Eventually(func() bool {
						deployment, err := fw.CommonController.GetAppDeploymentByName(componentTest.Name, AppStudioE2EApplicationsNamespace)
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

				It(fmt.Sprintf("check component %s health", componentTest.Name), func() {
					Eventually(func() bool {
						gitOpsRoute, err := fw.CommonController.GetOpenshiftRoute(componentTest.Name, AppStudioE2EApplicationsNamespace)
						Expect(err).NotTo(HaveOccurred())
						err = fw.GitOpsController.CheckGitOpsEndpoint(gitOpsRoute, componentTest.HealthEndpoint)
						if err != nil {
							klog.Info("Failed to request component endpoint. retrying...")
						}
						return true
					}, 5*time.Minute, 10*time.Second).Should(BeTrue())
				})

				if componentTest.K8sSpec != (config.K8sSpec{}) && *componentTest.K8sSpec.Replicas > 1 {
					It(fmt.Sprintf("scale component %s replicas", componentTest.Name), func() {
						component, err := fw.HasController.GetHasComponent(componentTest.Name, AppStudioE2EApplicationsNamespace)
						Expect(err).NotTo(HaveOccurred())
						_, err = fw.HasController.ScaleComponentReplicas(component, int(*componentTest.K8sSpec.Replicas))
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() bool {
							deployment, _ := fw.CommonController.GetAppDeploymentByName(componentTest.Name, AppStudioE2EApplicationsNamespace)
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

func LoadTestGeneratorConfig(configPath string) (config.WorkflowSpec, error) {
	c := config.WorkflowSpec{}
	// Open config file
	file, err := os.Open(filepath.Clean(configPath))
	if err != nil {
		return c, err
	}

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	return c, nil
}
