package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var AppStudioE2EApplicationsNamespace = utils.GetGeneratedNamespace("e2e-demo")

var _ = framework.E2ESuiteDescribe(Label("e2e-demo"), func() {
	defer GinkgoRecover()

	// Initialize the application struct
	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}

	// Initialize the tests controllers
	fw, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var testSpeicification = config.WorkflowSpec{
		Tests: []config.TestSpec{
			{
				Name:            "multi-component-application",
				ApplicationName: "multi-component-application",
				Components: []config.ComponentSpec{
					{
						Name:         "multi-component",
						Type:         "public",
						GitSourceUrl: "https://github.com/redhat-appstudio-qe/multi-component-example-main",
					},
				},
			},
		},
	}

	var compNameGo = testSpeicification.Tests[0].Components[0].Name + "-go"
	var compNameNode = testSpeicification.Tests[0].Components[0].Name + "-nodejs"

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

	})

	It("Create Red Hat AppStudio Application", func() {
		createdApplication, err := fw.HasController.CreateHasApplication(testSpeicification.Tests[0].ApplicationName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdApplication.Spec.DisplayName).To(Equal(testSpeicification.Tests[0].ApplicationName))
		Expect(createdApplication.Namespace).To(Equal(AppStudioE2EApplicationsNamespace))
	})

	It("Check Red Hat AppStudio Application health", func() {
		Eventually(func() string {
			application, err = fw.HasController.GetHasApplication(testSpeicification.Tests[0].ApplicationName, AppStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred())

			return application.Status.Devfile
		}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

			return fw.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
		}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
	})

	It("Create Red Hat AppStudio ComponentDetectionQuery for Component repository", func() {
		cdq, err := fw.HasController.CreateComponentDetectionQuery(testSpeicification.Tests[0].Components[0].Name, AppStudioE2EApplicationsNamespace, testSpeicification.Tests[0].Components[0].GitSourceUrl, "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(cdq.Name).To(Equal(testSpeicification.Tests[0].Components[0].Name))
	})

	It("Check Red Hat AppStudio ComponentDetectionQuery status", func() {
		// Validate that the CDQ completes successfully
		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			cdq, err = fw.HasController.GetComponentDetectionQuery(testSpeicification.Tests[0].Components[0].Name, AppStudioE2EApplicationsNamespace)
			return err == nil && len(cdq.Status.ComponentDetected) > 0
		}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "ComponentDetectionQuery did not complete successfully")

		// Validate that the completed CDQ only has detected the two components (nodejs and go)
		Expect(len(cdq.Status.ComponentDetected)).To(Equal(2), "Expected length of the detected Components was not 2")
		_, golang := cdq.Status.ComponentDetected["go"]
		Expect(golang).To(BeTrue(), "Expect Golang component to be detected")
		_, nodejs := cdq.Status.ComponentDetected["nodejs"]
		Expect(nodejs).To(BeTrue(), "Expect NodeJS component to be detected")

	})

	It("Create multiple components", func() {

		// Create Golang component from CDQ result
		Expect(cdq.Status.ComponentDetected["go"].DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
		componentGo, err := fw.HasController.CreateComponentFromStub(cdq.Status.ComponentDetected["go"], compNameGo, AppStudioE2EApplicationsNamespace, "", testSpeicification.Tests[0].ApplicationName)
		Expect(err).NotTo(HaveOccurred())
		Expect(componentGo.Name).To(Equal(compNameGo))

		// Create NodeJS component from CDQ result
		Expect(cdq.Status.ComponentDetected["nodejs"].DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
		componentNode, err := fw.HasController.CreateComponentFromStub(cdq.Status.ComponentDetected["nodejs"], compNameNode, AppStudioE2EApplicationsNamespace, "", testSpeicification.Tests[0].ApplicationName)
		Expect(err).NotTo(HaveOccurred())
		Expect(componentNode.Name).To(Equal(compNameNode))

	})

	// Start to watch the pipeline until is finished
	It("Wait for all pipelines to be finished", func() {

		Expect(fw.HasController.WaitForComponentPipelineToBeFinished(compNameGo, testSpeicification.Tests[0].ApplicationName, AppStudioE2EApplicationsNamespace)).To(Succeed(), "Failed component pipeline %v", err)
		Expect(fw.HasController.WaitForComponentPipelineToBeFinished(compNameNode, testSpeicification.Tests[0].ApplicationName, AppStudioE2EApplicationsNamespace)).To(Succeed(), "Failed component pipeline %v", err)

	})

	// Check components are deployed
	It("Check multiple components are deployed", func() {

		Eventually(func() bool {
			deploymentGo, err := fw.CommonController.GetAppDeploymentByName(compNameGo, AppStudioE2EApplicationsNamespace)
			if err != nil && !errors.IsNotFound(err) {
				return false
			}

			deploymentNode, err := fw.CommonController.GetAppDeploymentByName(compNameNode, AppStudioE2EApplicationsNamespace)
			if err != nil && !errors.IsNotFound(err) {
				return false
			}

			if deploymentGo.Status.AvailableReplicas == 1 && deploymentNode.Status.AvailableReplicas == 1 {
				return true
			}

			return false
		}, 15*time.Minute, 10*time.Second).Should(BeTrue(), "Component deployment didn't become ready")
		Expect(err).NotTo(HaveOccurred())
	})

})
