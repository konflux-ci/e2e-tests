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
	"github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	v1 "k8s.io/api/core/v1"
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
		suite := suite
		if !suite.Skip {
			Describe(suite.Name, Ordered, func() {
				BeforeAll(func() {
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
						Expect(fw.AsKubeAdmin.IntegrationController.DeleteAllSnapshotsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(namespace)).To(Succeed())
						Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllGitOpsDeploymentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
					}
				})

				// Create an application in a specific namespace
				It(fmt.Sprintf("create application %s", suite.ApplicationName), func() {
					GinkgoWriter.Printf("Parallel process %d\n", GinkgoParallelProcess())
					application, err := fw.AsKubeDeveloper.HasController.CreateApplication(suite.ApplicationName, namespace)
					Expect(err).NotTo(HaveOccurred())
					Expect(application.Spec.DisplayName).To(Equal(suite.ApplicationName))
					Expect(application.Namespace).To(Equal(namespace))
				})

				// Check the application health and check if a devfile was generated in the status
				It(fmt.Sprintf("checks if application %s is healthy", suite.ApplicationName), func() {
					Eventually(func() string {
						appstudioApp, err := fw.AsKubeDeveloper.HasController.GetApplication(suite.ApplicationName, namespace)
						Expect(err).NotTo(HaveOccurred())
						application = appstudioApp

						return application.Status.Devfile
					}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), fmt.Sprintf("timed out waiting for gitOps repository to be created for the %s application in %s namespace", suite.ApplicationName, namespace))

					Eventually(func() bool {
						gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

						return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
					}, 5*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("timed out waiting for HAS controller to create gitops repository for the %s application in %s namespace", suite.ApplicationName, namespace))
				})

				for _, testComponent := range suite.Components {
					testComponent := testComponent
					cdq := &appservice.ComponentDetectionQuery{}
					componentList := []*appservice.Component{}
					env := &appservice.Environment{}

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
							Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "cdq also detect unsupported component")
						}
						for _, component := range cdq.Status.ComponentDetected {
							Expect(runtimeSupported).To(ContainElement(component.ProjectType), "unsupported runtime used for multi component tests")
						}
					})

					// Create an environment in a specific namespace
					It(fmt.Sprintf("creates environment %s", EnvironmentName), func() {
						env, err = fw.AsKubeDeveloper.GitOpsController.CreatePocEnvironment(EnvironmentName, namespace)
						Expect(err).NotTo(HaveOccurred())
					})

					It(fmt.Sprintf("creates multiple components in application %s", suite.ApplicationName), func() {
						for _, component := range cdq.Status.ComponentDetected {
							c, err := fw.AsKubeDeveloper.HasController.CreateComponent(component.ComponentStub, namespace, "", SPIGithubSecretName, application.Name, true, map[string]string{})
							Expect(err).NotTo(HaveOccurred())
							Expect(c.Name).To(Equal(component.ComponentStub.ComponentName))
							Expect(runtimeSupported).To(ContainElement(component.ProjectType), "unsupported runtime used for multi component tests")

							componentList = append(componentList, c)
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

							Eventually(func() error {
								componentSnapshot, err = fw.AsKubeDeveloper.IntegrationController.GetSnapshot("", "", component.Name, namespace)
								if err != nil {
									GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
									return err
								}
								if !fw.AsKubeAdmin.CommonController.HaveTestsSucceeded(componentSnapshot) {
									return fmt.Errorf("tests haven't succeeded for snapshot %s/%s. snapshot status: %+v", componentSnapshot.GetNamespace(), componentSnapshot.GetName(), componentSnapshot.Status)
								}
								return nil
							}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out waiting for the snapshot for the component %s/%s to be marked as successful", component.GetNamespace(), component.GetName()))

							Eventually(func() error {
								_, err := fw.AsKubeAdmin.HasController.GetSnapshotEnvironmentBinding(application.Name, namespace, env)
								if err != nil {
									GinkgoWriter.Println("SnapshotEnvironmentBinding has not been found yet")
									return err
								}
								return nil
							}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out waiting for the SnapshotEnvironmentBinding to be created (snapshot: %s, env: %s, namespace: %s)", componentSnapshot.GetName(), env.GetName(), componentSnapshot.GetNamespace()))
						}
					})

					if !testComponent.SkipDeploymentCheck {
						It("checks if multiple components are deployed", func() {
							var expectedReplicas int32 = 1
							for _, component := range componentList {
								Eventually(func() error {
									componentDeployment, err := fw.AsKubeDeveloper.CommonController.GetDeployment(component.Name, namespace)
									if err != nil {
										return err
									}
									if componentDeployment.Status.AvailableReplicas != expectedReplicas {
										return fmt.Errorf("the deployment %s/%s does not have the expected amount of replicas (expected: %d, got: %d)", componentDeployment.GetNamespace(), componentDeployment.GetName(), expectedReplicas, componentDeployment.Status.AvailableReplicas)
									}
									return nil
								}, 10*time.Minute, 10*time.Second).Should(Succeed(), fmt.Sprintf("timed out waiting for deployment of a component %s/%s to have expected amount of replicas", component.GetNamespace(), component.GetName()))

							}
						})

						It("checks if multicomponents routes exists", func() {
							for _, component := range componentList {
								Eventually(func() error {
									if _, err := fw.AsKubeDeveloper.CommonController.GetOpenshiftRouteByComponentName(component.Name, namespace); err != nil {
										return err
									}
									return nil
								}, 10*time.Minute, 10*time.Second).Should(Succeed(), fmt.Sprintf("timed out waiting for a route to be created for a component %s/%s", component.GetNamespace(), component.GetName()))
							}
						})
					}
				}
			})
		}
	}
})
