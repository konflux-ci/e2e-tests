package e2e

import (
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
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	MultiComponentDemoNamespace string = "multi-comp-e2e"
)

var _ = framework.E2ESuiteDescribe(Label("e2e-demo"), func() {
	// TODO investigate failing of Component detection
	var timeout, interval time.Duration

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	// Initialize the application struct
	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	componentGo := &appservice.Component{}
	componentNode := &appservice.Component{}
	snapshotGo := &appservice.Snapshot{}
	snapshotNode := &appservice.Snapshot{}
	env := &appservice.Environment{}

	var testSpecification = config.WorkflowSpec{
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

	var compNameGo string
	var compNameNode string

	var removeApplication = true

	var namespace string

	Describe(testSpecification.Tests[0].ApplicationName, Ordered, func() {
		BeforeAll(func() {
			Skip("skip tests due a issue with dockerfile detections. See jira: https://issues.redhat.com/browse/DEVHAS-266")
			// Initialize the tests controllers
			fw, err = framework.NewFramework(MultiComponentDemoNamespace)
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			// Check to see if the github token was provided
			Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
			// Check test specification has at least one test defined
			Expect(len(testSpecification.Tests)).To(BeNumerically(">", 0))
		})

		// Remove all resources created by the tests
		AfterAll(func() {
			if removeApplication {
				Expect(fw.AsKubeDeveloper.HasController.DeleteAllComponentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
				Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
				Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
			}
		})

		It("creates Red Hat AppStudio Application", func() {
			createdApplication, err := fw.AsKubeDeveloper.HasController.CreateHasApplication(testSpecification.Tests[0].ApplicationName, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdApplication.Spec.DisplayName).To(Equal(testSpecification.Tests[0].ApplicationName))
			Expect(createdApplication.Namespace).To(Equal(namespace))
		})

		It("checks Red Hat AppStudio Application health", func() {
			Eventually(func() string {
				application, err = fw.AsKubeDeveloper.HasController.GetHasApplication(testSpecification.Tests[0].ApplicationName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return application.Status.Devfile
			}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
		})

		// Create an environment in a specific namespace
		It("creates an environment", func() {
			env, err = fw.AsKubeDeveloper.IntegrationController.CreateEnvironment(namespace, EnvironmentName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates Red Hat AppStudio ComponentDetectionQuery for Component repository", func() {
			cdq, err := fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(
				testSpecification.Tests[0].Components[0].Name,
				namespace,
				testSpecification.Tests[0].Components[0].GitSourceUrl,
				testSpecification.Tests[0].Components[0].GitSourceRevision,
				testSpecification.Tests[0].Components[0].GitSourceContext,
				"",
				false)
			Expect(err).NotTo(HaveOccurred())
			Expect(cdq.Name).To(Equal(testSpecification.Tests[0].Components[0].Name))
		})

		It("checks Red Hat AppStudio ComponentDetectionQuery status", func() {
			// Validate that the CDQ completes successfully
			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				cdq, err = fw.AsKubeDeveloper.HasController.GetComponentDetectionQuery(testSpecification.Tests[0].Components[0].Name, namespace)
				return err == nil && len(cdq.Status.ComponentDetected) > 0
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "ComponentDetectionQuery did not complete successfully")

			// Validate that the completed CDQ only has detected the two components (nodejs and go)
			Expect(len(cdq.Status.ComponentDetected)).To(Equal(2), "Expected length of the detected Components was not 2")

			// get the name of the components for future use and validate they are go and nodejs
			for key, element := range cdq.Status.ComponentDetected {
				if element.Language == "go" {
					compNameGo = key
				}
				if element.Language == "nodejs" {
					compNameNode = key
				}
			}

			_, golang := cdq.Status.ComponentDetected[compNameGo]
			Expect(golang).To(BeTrue(), "Expect Golang component to be detected")
			_, nodejs := cdq.Status.ComponentDetected[compNameNode]
			Expect(nodejs).To(BeTrue(), "Expect NodeJS component to be detected")
		})

		It("creates multiple components", func() {
			// Create Golang component from CDQ result
			Expect(cdq.Status.ComponentDetected[compNameGo].DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
			componentDescription := cdq.Status.ComponentDetected[compNameGo]
			componentDescription.ComponentStub.ContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			componentGo, err = fw.AsKubeDeveloper.HasController.CreateComponentFromStub(componentDescription, compNameGo, namespace, "", testSpecification.Tests[0].ApplicationName, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(componentGo.Name).To(Equal(compNameGo))

			// Create NodeJS component from CDQ result
			Expect(cdq.Status.ComponentDetected[compNameNode].DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
			componentDescription = cdq.Status.ComponentDetected[compNameNode]
			componentDescription.ComponentStub.ContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			componentNode, err = fw.AsKubeDeveloper.HasController.CreateComponentFromStub(componentDescription, compNameNode, namespace, "", testSpecification.Tests[0].ApplicationName, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(componentNode.Name).To(Equal(compNameNode))
		})

		// Start to watch the pipeline until is finished
		It("waits for all pipelines to be finished", func() {
			err := fw.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(fw.AsKubeAdmin.CommonController, compNameGo, testSpecification.Tests[0].ApplicationName, namespace, "")
			if err != nil {
				removeApplication = false
				Fail(fmt.Sprint(err))
			}

			err = fw.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(fw.AsKubeAdmin.CommonController, compNameNode, testSpecification.Tests[0].ApplicationName, namespace, "")
			if err != nil {
				removeApplication = false
				Fail(fmt.Sprint(err))
			}
		})

		It("finds the snapshot and checks if it is marked as successful for golang component", func() {
			timeout = time.Second * 600
			interval = time.Second * 10

			snapshotGo, err = fw.AsKubeDeveloper.IntegrationController.GetApplicationSnapshot("", application.Name, namespace, componentGo.Name)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				return fw.AsKubeDeveloper.IntegrationController.HaveHACBSTestsSucceeded(snapshotGo)

			}, timeout, interval).Should(BeTrue(), "time out when trying to check if the snapshot is marked as successful")
		})

		It("finds the snapshot and checks if it is marked as successful for NodeJS component", func() {
			timeout = time.Second * 600
			interval = time.Second * 10

			snapshotNode, err = fw.AsKubeDeveloper.IntegrationController.GetApplicationSnapshot("", application.Name, namespace, componentNode.Name)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				return fw.AsKubeDeveloper.IntegrationController.HaveHACBSTestsSucceeded(snapshotNode)

			}, timeout, interval).Should(BeTrue(), "time out when trying to check if the snapshot is marked as successful")
		})

		It("checks if a golang snapshot environment binding is created successfully", func() {
			Eventually(func() bool {
				if fw.AsKubeDeveloper.IntegrationController.HaveHACBSTestsSucceeded(snapshotGo) {
					envbinding, err := fw.AsKubeDeveloper.IntegrationController.GetSnapshotEnvironmentBinding(application.Name, namespace, env)
					Expect(err).ShouldNot(HaveOccurred())
					GinkgoWriter.Printf("The EnvironmentBinding %s is created\n", envbinding.Name)
					return true
				}

				snapshotGo, err = fw.AsKubeDeveloper.IntegrationController.GetApplicationSnapshot("", application.Name, namespace, compNameGo)
				Expect(err).ShouldNot(HaveOccurred())
				return false
			}, timeout, interval).Should(BeTrue(), "time out when waiting for release created")
		})

		It("checks if a Nodejs snapshot environment binding is created successfully", func() {
			Eventually(func() bool {
				if fw.AsKubeDeveloper.IntegrationController.HaveHACBSTestsSucceeded(snapshotGo) {
					envbinding, err := fw.AsKubeDeveloper.IntegrationController.GetSnapshotEnvironmentBinding(application.Name, namespace, env)
					Expect(err).ShouldNot(HaveOccurred())
					GinkgoWriter.Printf("The EnvironmentBinding %s is created\n", envbinding.Name)
					return true
				}

				snapshotGo, err = fw.AsKubeDeveloper.IntegrationController.GetApplicationSnapshot("", application.Name, namespace, componentNode.Name)
				Expect(err).ShouldNot(HaveOccurred())
				return false
			}, timeout, interval).Should(BeTrue(), "time out when waiting for release created")
		})

		// Check components are deployed
		It("checks if multiple components are deployed", Pending, func() {
			Eventually(func() bool {
				deploymentGo, err := fw.AsKubeDeveloper.CommonController.GetAppDeploymentByName(compNameGo, namespace)
				if err != nil && !errors.IsNotFound(err) {
					return false
				}

				deploymentNode, err := fw.AsKubeDeveloper.CommonController.GetAppDeploymentByName(compNameNode, namespace)
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
})
