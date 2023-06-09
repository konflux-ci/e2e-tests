package e2e

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// All multiple components scenarios are supported in the next jira: https://issues.redhat.com/browse/DEVHAS-305
const (
	MultiComponentWithoutDockerFileAndDevfile     = "multi-component scenario with components without devfile or dockerfile"
	MultiComponentWithAllSupportedImportScenarios = "multi-component scenario with all supported import components"
	MultiComponentWithDevfileAndDockerfile        = "multi-component scenario with components with devfile or dockerfile or both"
	MultiComponentWithUnsupportedRuntime          = "multi-component scenario with a component with a supported runtime and another unsuported"
)

var runtimeSupported = []string{"Dockerfile", "Node.js", "Go", "Quarkus", "Python", "JavaScript", "springboot"}

const (
	multiComponentTestNamespace string = "multi-comp-e2e"
)

var _ = framework.E2ESuiteDescribe(Label("e2e-demo", "multi-component"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string

	// Initialize the application struct
	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	componentList := []*appservice.Component{}
	env := &appservice.Environment{}

	// https://github.com/redhat-appstudio-qe/rhtap-three-component-scenarios.git
	var testSpecification = config.WorkflowSpec{
		Tests: []config.TestSpec{
			{
				Name:            MultiComponentWithoutDockerFileAndDevfile,
				ApplicationName: "mc-quality-dashboard",
				// We need to skip for now deployment checks of quality dashboard until RHTAP support secrets
				Components: []config.ComponentSpec{
					{
						Name:                "mc-withdockerfile-withoutdevfile",
						SkipDeploymentCheck: true,
						Type:                "public",
						GitSourceUrl:        "https://github.com/redhat-appstudio/quality-dashboard.git",
					},
				},
			},
			{
				Name:            MultiComponentWithDevfileAndDockerfile,
				ApplicationName: "mc-two-scenarios",
				Components: []config.ComponentSpec{
					{
						Name:                "mc-two-scenarios",
						SkipDeploymentCheck: false,
						Type:                "public",
						GitSourceUrl:        "https://github.com/redhat-appstudio-qe/rhtap-devfile-multi-component.git",
					},
				},
			},
			{
				Name:            MultiComponentWithAllSupportedImportScenarios,
				ApplicationName: "mc-three-scenarios",
				Components: []config.ComponentSpec{
					{
						Name:                "mc-three-scenarios",
						Type:                "public",
						SkipDeploymentCheck: false,
						GitSourceUrl:        "https://github.com/redhat-appstudio-qe/rhtap-three-component-scenarios.git",
					},
				},
			},
			{
				Name:            MultiComponentWithUnsupportedRuntime,
				ApplicationName: "mc-unsupported-runtime",
				Components: []config.ComponentSpec{
					{
						Name:                "mc-unsuported-runtime",
						Type:                "public",
						SkipDeploymentCheck: false,
						GitSourceUrl:        "https://github.com/redhat-appstudio-qe/rhtap-mc-unsuported-runtime.git",
					},
				},
			},
		},
	}

	for _, suite := range testSpecification.Tests {
		Describe(suite.Name, Ordered, func() {
			suite := suite
			BeforeAll(func() {
				if suite.Skip {
					Skip(fmt.Sprintf("test skipped %s", suite.Name))
				}

				// Initialize the tests controllers
				fw, err = framework.NewFramework(utils.GetGeneratedNamespace(multiComponentTestNamespace))
				Expect(err).NotTo(HaveOccurred())
				namespace = fw.UserNamespace
				Expect(namespace).NotTo(BeEmpty())

				// collect SPI ResourceQuota metrics (temporary)
				err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("multi-component", namespace, "appstudio-crds-spi")
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
				err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("multi-component", namespace, "appstudio-crds-spi")
				Expect(err).NotTo(HaveOccurred())

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
			It(fmt.Sprintf("create application %s", suite.ApplicationName), func() {
				GinkgoWriter.Printf("Parallel process %d\n", GinkgoParallelProcess())
				application, err := fw.AsKubeDeveloper.HasController.CreateHasApplication(suite.ApplicationName, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(application.Spec.DisplayName).To(Equal(suite.ApplicationName))
				Expect(application.Namespace).To(Equal(namespace))
			})

			// Check the application health and check if a devfile was generated in the status
			It(fmt.Sprintf("checks if application %s is healthy", suite.ApplicationName), func() {
				Eventually(func() string {
					appstudioApp, err := fw.AsKubeDeveloper.HasController.GetHasApplication(suite.ApplicationName, namespace)
					Expect(err).NotTo(HaveOccurred())
					application = appstudioApp

					return application.Status.Devfile
				}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

				Eventually(func() bool {
					gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

					return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
				}, 5*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
			})

			for _, testComponent := range suite.Components {
				testComponent := testComponent

				It(fmt.Sprintf("creates ComponentDetectionQuery for application %s", suite.ApplicationName), func() {
					cdq, err = fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(
						testComponent.Name,
						namespace,
						testComponent.GitSourceUrl,
						testComponent.GitSourceRevision,
						testComponent.GitSourceContext,
						"",
						false,
					)
					Expect(err).NotTo(HaveOccurred())
					Expect(cdq.Name).To(Equal(testComponent.Name))
				})

				It("check if components have supported languages by AppStudio", func() {
					if suite.Name == MultiComponentWithUnsupportedRuntime {
						// Validate that the completed CDQ only has detected 1 component and not also the unsupported component
						Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "cdq also detect unsupported component")
					}
					for _, component := range cdq.Status.ComponentDetected {
						Expect(utils.Contains(runtimeSupported, component.ProjectType), "unsupported runtime used for multi component tests")

					}
				})

				// Create an environment in a specific namespace
				It(fmt.Sprintf("creates environment %s", EnvironmentName), func() {
					env, err = fw.AsKubeDeveloper.IntegrationController.CreateEnvironment(namespace, EnvironmentName)
					Expect(err).NotTo(HaveOccurred())
				})

				It(fmt.Sprintf("creates multiple components in application %s", suite.ApplicationName), func() {
					for _, component := range cdq.Status.ComponentDetected {
						// Skip https://github.com/redhat-appstudio/quality-dashboard/tree/main/frontend because takes to much to much ton push to quay.io and analyze sbom due huge image and is make
						// ci to be slow
						if !strings.Contains(component.ComponentStub.ComponentName, "frontend-quality") {
							c, err := fw.AsKubeDeveloper.HasController.CreateComponentFromStub(component, namespace, "", SPIGithubSecretName, application.Name)
							Expect(err).NotTo(HaveOccurred())
							Expect(c.Name).To(Equal(component.ComponentStub.ComponentName))
							Expect(utils.Contains(runtimeSupported, component.ProjectType), "unsupported runtime used for multi component tests")

							componentList = append(componentList, c)
						}
					}
				})

				It(fmt.Sprintf("waits application %s components pipelines to be finished", suite.ApplicationName), func() {
					for _, component := range componentList {
						Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", 2)).To(Succeed())
					}
				})

				It(fmt.Sprintf("finds the application %s components snapshots and checks if it is marked as successfully", suite.ApplicationName), func() {
					timeout := time.Second * 600
					interval := time.Second * 10

					for _, component := range componentList {
						var componentSnapshot *appservice.Snapshot

						Eventually(func() bool {
							componentSnapshot, err = fw.AsKubeDeveloper.IntegrationController.GetSnapshot("", "", component.Name, namespace)
							if err != nil {
								GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
								return false
							}

							return fw.AsKubeDeveloper.IntegrationController.HaveTestsSucceeded(componentSnapshot)
						}, timeout, interval).Should(BeTrue(), "time out when trying to either check if the snapshot is created or is marked as successful")

						Eventually(func() bool {
							if fw.AsKubeDeveloper.IntegrationController.HaveTestsSucceeded(componentSnapshot) {
								envbinding, err := fw.AsKubeDeveloper.IntegrationController.GetSnapshotEnvironmentBinding(application.Name, namespace, env)
								if err != nil {
									GinkgoWriter.Printf("cannot get the SnapshotEnvironmentBinding: %v\n", err)
									return false
								}
								GinkgoWriter.Printf("The SnapshotEnvironmentBinding %s is found\n", envbinding.Name)
								return true
							}

							componentSnapshot, err = fw.AsKubeDeveloper.IntegrationController.GetSnapshot("", "", component.Name, namespace)
							if err != nil {
								GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
								return false
							}
							return false
						}, timeout, interval).Should(BeTrue(), "time out when waiting for snapshoot environment binding")
					}
				})

				It("checks if multiple components are deployed", func() {
					if testComponent.SkipDeploymentCheck {
						Skip("component deployment skipped.")
					}
					for _, component := range componentList {
						Eventually(func() bool {
							componentDeployment, err := fw.AsKubeDeveloper.CommonController.GetDeployment(component.Name, namespace)
							if err != nil && !errors.IsNotFound(err) {
								return false
							}

							if componentDeployment.Status.AvailableReplicas == 1 {
								return true
							}
							return false
						})

					}
				})

				It("checks if multicomponents routes exists", func() {
					if testComponent.SkipDeploymentCheck {
						Skip("component deployment skipped.")
					}
					for _, component := range componentList {
						Eventually(func() bool {
							if _, err := fw.AsKubeDeveloper.CommonController.GetOpenshiftRoute(component.Name, namespace); err != nil {
								return false
							}
							return true
						})
					}
				})
			}
		})
	}
})
