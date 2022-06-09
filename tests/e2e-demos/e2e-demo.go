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
	"k8s.io/klog"
)

const (
	// The name of the SPIAccessTokenBinding resource that the HAS e2e tests will create
	SPIAccessTokenBindingName string = "has-private-git-repo-binding" // #nosec

	// The name of the secret to be created by the SPIAccessTokenBinding resource
	SPIAccessTokenSecretName string = "has-private-git-repo-secret" // #nosec

	// Valid container with a quarkus image to import in appstudio.Using to test a component imported from quay.io
	containerImageSource = "quay.io/redhat-appstudio-qe/test-images:7ac98d2c0ff64671baa54d4a94675601"
)

var _ = framework.E2ESuiteDescribe("test-generator", func() {
	defer GinkgoRecover()

	// Initialize the application struct
	application := &appservice.Application{}
	component := &appservice.Component{}

	// Initialize the e2e demo configuration
	configTestFile := viper.GetString("config-suites")
	klog.Infof("Starting e2e-demo test suites from config: %s", configTestFile)

	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())
	configTest, err := LoadTestGeneratorConfig(configTestFile)
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
		// Check if 'has-github-token' is present, unless SKIP_HAS_SECRET_CHECK env var is set
		if !utils.CheckIfEnvironmentExists(constants.SKIP_HAS_SECRET_CHECK_ENV) {
			_, err := framework.HasController.KubeInterface().CoreV1().Secrets(RedHatAppStudioApplicationNamespace).Get(context.TODO(), ApplicationServiceGHTokenSecrName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Error checking 'has-github-token' secret %s", err)
		}

		_, err := framework.CommonController.CreateTestNamespace(AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", AppStudioE2EApplicationsNamespace, err)
	})

	// Remove all resources created by the tests
	AfterAll(func() {
		Expect(framework.HasController.DeleteAllComponentsInASpecificNamespace(AppStudioE2EApplicationsNamespace)).NotTo(HaveOccurred())
		Expect(framework.HasController.DeleteAllApplicationsInASpecificNamespace(AppStudioE2EApplicationsNamespace)).NotTo(HaveOccurred())
	})

	for _, appTest := range configTest.Tests {
		appTest := appTest

		When(fmt.Sprintf(appTest.Name), func() {
			// Create an application in a specific namespace
			It(fmt.Sprintf("%s is created", appTest.Name), func() {
				createdApplication, err := framework.HasController.CreateHasApplication(appTest.ApplicationName, AppStudioE2EApplicationsNamespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdApplication.Spec.DisplayName).To(Equal(appTest.ApplicationName))
				Expect(createdApplication.Namespace).To(Equal(AppStudioE2EApplicationsNamespace))
			})

			// Check the application health and check if a devfile was generated in the status
			It(fmt.Sprintf("%s is healthy", appTest.Name), func() {
				Eventually(func() string {
					appstudioApp, err := framework.HasController.GetHasApplication(appTest.ApplicationName, AppStudioE2EApplicationsNamespace)
					Expect(err).NotTo(HaveOccurred())
					application = appstudioApp

					return application.Status.Devfile
				}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

				Eventually(func() bool {
					// application info should be stored even after deleting the application in application variable
					gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

					return framework.HasController.Github.CheckIfRepositoryExist(gitOpsRepository)
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
						_, err := framework.HasController.CreateComponent(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace, "", containerImageSource, outputContainerImage, "")
						Expect(err).NotTo(HaveOccurred())
					})

				} else if componentTest.GitSourceUrl != "" && componentTest.Devfilesource != "" {
					It(fmt.Sprintf("create component %s from git source %s and devfile %s", componentTest.Name, componentTest.GitSourceUrl, componentTest.Devfilesource), func() {
						component, err = framework.HasController.CreateComponentFromDevfile(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace,
							componentTest.GitSourceUrl, componentTest.Devfilesource, "", containerIMG, "")
						Expect(err).NotTo(HaveOccurred())
					})

				} else if componentTest.GitSourceUrl != "" {
					It(fmt.Sprintf("create component %s from git source %s", componentTest.Name, componentTest.GitSourceUrl), func() {
						component, err = framework.HasController.CreateComponent(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace,
							componentTest.GitSourceUrl, "", containerIMG, "")
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
					Expect(framework.HasController.WaitForComponentPipelineToBeFinished(component.Name, application.Name, AppStudioE2EApplicationsNamespace)).NotTo(HaveOccurred(), "Failed component pipeline %v", err)
				})

				// Deploy the component using gitops and check for the health
				It(fmt.Sprintf("deploy component %s using gitops", componentTest.Name), func() {
					gitOpsRepository := utils.ObtainGitOpsRepositoryUrl(application.Status.Devfile)
					gitOpsRepositoryPath := fmt.Sprintf("components/%s/base", componentTest.Name)

					_, err := framework.GitOpsController.CreateGitOpsCR(GitOpsDeploymentName, AppStudioE2EApplicationsNamespace, gitOpsRepository, gitOpsRepositoryPath, GitOpsRepositoryRevision)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() bool {
						deployment, _ := framework.CommonController.GetAppDeploymentByName(componentTest.Name, AppStudioE2EApplicationsNamespace)
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
					gitOpsRoute, err := framework.CommonController.GetOpenshiftRoute(componentTest.Name, AppStudioE2EApplicationsNamespace)
					Expect(err).NotTo(HaveOccurred())
					err = framework.GitOpsController.CheckGitOpsEndpoint(gitOpsRoute, componentTest.HealthEndpoint)
					Expect(err).NotTo(HaveOccurred())
				})
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
