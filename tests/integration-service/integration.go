package integration

import (
	"fmt"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	gitSourceRepoName      = "devfile-sample-python-basic"
	gitSourceURL           = "https://github.com/redhat-appstudio-qe/" + gitSourceRepoName
	BundleURL              = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass"
	BundleURLFail          = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-fail"
	InPipelineName         = "integration-pipeline-pass"
	InPipelineNameFail     = "integration-pipeline-fail"
	EnvironmentName        = "development"
	gitURL                 = "https://github.com/redhat-appstudio/integration-examples.git"
	revision               = "843f455fe87a6d7f68c238f95a8f3eb304e65ac5"
	pathInRepo             = "pipelines/integration_resolver_pipeline_pass.yaml"
	autoReleasePlan        = "auto-releaseplan"
	targetReleaseNamespace = "default"
)

var _ = framework.IntegrationServiceSuiteDescribe("Integration Service E2E tests", Label("integration-service", "HACBS"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var applicationName, componentName, testNamespace string
	var timeout, interval time.Duration
	var originalComponent *appstudioApi.Component
	var pipelineRun *v1beta1.PipelineRun
	var snapshot *appstudioApi.Snapshot
	var snapshotPush *appstudioApi.Snapshot
	var env *appstudioApi.Environment
	AfterEach(framework.ReportFailure(&f))

	Describe("with happy path for general flow of Integration service", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("integration1"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			applicationName = createApp(*f, testNamespace)
			componentName, originalComponent = createComponent(*f, testNamespace, applicationName)
			_, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, testNamespace, BundleURL, InPipelineName)
			// create a integrationTestScenario v1beta1 version works also here
			// ex: _, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario_beta1(applicationName, testNamespace, gitURL, revision, pathInRepo)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentName)

				Expect(f.AsKubeAdmin.IntegrationController.DeleteSnapshot(snapshotPush, testNamespace)).To(Succeed())
				Expect(f.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(EnvironmentName, time.Minute*5)).To(Succeed())
			}
		})

		It("triggers a build PipelineRun", Label("integration-service"), func() {
			pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
			Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(originalComponent, "", 2, f.AsKubeAdmin.TektonController)).To(Succeed())
			Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(""))
		})

		When("the build pipelineRun run succeeded", func() {
			It("checks if the BuildPipelineRun is signed", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToBeSigned(testNamespace, applicationName, componentName)).To(Succeed())
			})

			It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("checks if the Build PipelineRun got annotated with Snapshot name", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, "appstudio.openshift.io/snapshot")).To(Succeed())
			})

			It("checks if all of the integrationPipelineRuns passed", Label("slow"), func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName, snapshot)).To(Succeed())
			})
		})

		It("creates a ReleasePlan and an environment", func() {
			_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlan(autoReleasePlan, testNamespace, applicationName, targetReleaseNamespace, "")
			Expect(err).ShouldNot(HaveOccurred())
			env, err = f.AsKubeAdmin.GitOpsController.CreatePocEnvironment(EnvironmentName, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			testScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			for _, testScenario := range *testScenarios {
				GinkgoWriter.Printf("IntegrationTestScenario %s is found\n", testScenario.Name)
			}
		})

		It("creates an snapshot of push event", func() {
			sampleImage := "quay.io/redhat-appstudio/sample-image@sha256:841328df1b9f8c4087adbdcfec6cc99ac8308805dea83f6d415d6fb8d40227c1"
			snapshotPush, err = f.AsKubeAdmin.IntegrationController.CreateSnapshotWithImage(componentName, applicationName, testNamespace, sampleImage)
			Expect(err).ShouldNot(HaveOccurred())
		})

		When("An snapshot of push event is created", func() {
			It("checks if all of the integrationPipelineRuns created by push event passed", Label("slow"), func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName, snapshotPush)).To(Succeed(), "Error when waiting for one of the integration pipelines to finish in %s namespace", testNamespace)
			})

			It("checks if the global candidate is updated after push event", func() {
				timeout = time.Second * 600
				interval = time.Second * 10
				Eventually(func() error {
					snapshotPush, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshotPush.Name, "", "", testNamespace)
					Expect(err).ShouldNot(HaveOccurred())

					if f.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshotPush) {
						component, err := f.AsKubeAdmin.HasController.GetComponentByApplicationName(applicationName, testNamespace)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(component.Spec.ContainerImage).ToNot(Equal(originalComponent.Spec.ContainerImage))
						return nil
					}
					return fmt.Errorf("tests haven't succeeded yet for snapshot %s/%s", snapshotPush.GetNamespace(), snapshotPush.GetName())
				}, timeout, interval).Should(Succeed(), fmt.Sprintf("time out when waiting for updating the global candidate in %s namespace", testNamespace))
			})

			It("checks if a Release is created successfully", func() {
				timeout = time.Second * 60
				interval = time.Second * 5
				Eventually(func() error {
					_, err := f.AsKubeAdmin.ReleaseController.GetReleases(testNamespace)
					return err
				}, timeout, interval).Should(Succeed(), fmt.Sprintf("time out when waiting for release created for snapshot %s/%s", snapshotPush.GetNamespace(), snapshotPush.GetName()))
			})

			It("checks if an SnapshotEnvironmentBinding is created successfully", func() {
				timeout = time.Second * 600
				interval = time.Second * 2
				Eventually(func() error {
					_, err := f.AsKubeAdmin.CommonController.GetSnapshotEnvironmentBinding(applicationName, testNamespace, env)
					return err
				}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out when waiting for SnapshotEnvironmentBinding to be created for application %s/%s", testNamespace, applicationName))
			})
		})
	})

	Describe("with an integration test fail", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("integration2"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			applicationName = createApp(*f, testNamespace)
			componentName, originalComponent = createComponent(*f, testNamespace, applicationName)

			env, err = f.AsKubeAdmin.GitOpsController.CreatePocEnvironment(EnvironmentName, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(applicationName, testNamespace, BundleURLFail, InPipelineNameFail)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentName)

				Expect(f.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(EnvironmentName, time.Minute*5)).To(Succeed())
			}
		})

		It("triggers a build PipelineRun", Label("integration-service"), func() {
			pipelineRun, err = f.AsKubeDeveloper.IntegrationController.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
			Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(originalComponent, "", 2, f.AsKubeAdmin.TektonController)).To(Succeed())
			Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(""))
		})

		It("checks if the BuildPipelineRun is signed", func() {
			Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToBeSigned(testNamespace, applicationName, componentName)).To(Succeed())
		})

		It("checks if the Snapshot is created", func() {
			snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", pipelineRun.Name, componentName, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("checks if the Build PipelineRun got annotated with Snapshot name", func() {
			Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, "appstudio.openshift.io/snapshot")).To(Succeed())
		})

		It("checks if all of the integrationPipelineRuns finished", Label("slow"), func() {
			Expect(f.AsKubeDeveloper.IntegrationController.WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName, snapshot)).To(Succeed())
		})

		It("checks if snapshot is marked as failed", FlakeAttempts(3), func() {
			snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(f.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot)).To(BeFalse(), "expected tests to fail for snapshot %s/%s", snapshot.GetNamespace(), snapshot.GetName())
		})

		It("creates an snapshot of push event", func() {
			sampleImage := "quay.io/redhat-appstudio/sample-image@sha256:841328df1b9f8c4087adbdcfec6cc99ac8308805dea83f6d415d6fb8d40227c1"
			snapshotPush, err = f.AsKubeAdmin.IntegrationController.CreateSnapshotWithImage(componentName, applicationName, testNamespace, sampleImage)
			Expect(err).ShouldNot(HaveOccurred())
		})

		When("An snapshot of push event is created", func() {
			It("checks no Release CRs are created", func() {
				releases, err := f.AsKubeAdmin.ReleaseController.GetReleases(testNamespace)
				Expect(err).NotTo(HaveOccurred(), "Error when fetching the Releases")
				Expect(releases.Items).To(BeEmpty(), "Expected no Release CRs to be present, but found some")
			})

			It("checks no SnapshotEnvironmentBinding is created", func() {
				seb, err := f.AsKubeAdmin.CommonController.GetSnapshotEnvironmentBinding(applicationName, testNamespace, env)

				if err != nil {
					Expect(err.Error()).To(ContainSubstring("no SnapshotEnvironmentBinding found"))
				} else {
					Expect(seb).To(BeNil(), "Expected no SnapshotEnvironmentBinding to be present, but found one")
				}
			})

			It("checks if the global candidate is not updated", func() {
				// give some time to do eventual updates in component
				time.Sleep(60 * time.Second)

				component, err := f.AsKubeAdmin.HasController.GetComponentByApplicationName(applicationName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				// global candidate is not updated
				Expect(component.Spec.ContainerImage).To(Equal(originalComponent.Spec.ContainerImage))
			})
		})
	})
})

func createApp(f framework.Framework, testNamespace string) string {
	applicationName := fmt.Sprintf("integ-app-%s", util.GenerateRandomString(4))

	app, err := f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
	Expect(err).NotTo(HaveOccurred())
	Expect(utils.WaitUntil(f.AsKubeAdmin.HasController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
		Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
	)

	return applicationName
}

func createComponent(f framework.Framework, testNamespace, applicationName string) (string, *appstudioApi.Component) {
	var originalComponent *appstudioApi.Component

	componentName := fmt.Sprintf("integration-suite-test-component-git-source-%s", util.GenerateRandomString(4))
	// Create a component with Git Source URL being defined
	// using cdq since git ref is not known
	cdq, err := f.AsKubeAdmin.HasController.CreateComponentDetectionQuery(componentName, testNamespace, componentRepoURL, "", "", "", false)
	Expect(err).NotTo(HaveOccurred())
	Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "Expected length of the detected Components was not 1")

	for _, compDetected := range cdq.Status.ComponentDetected {
		originalComponent, err = f.AsKubeAdmin.HasController.CreateComponent(compDetected.ComponentStub, testNamespace, "", "", applicationName, true, map[string]string{})
		Expect(err).NotTo(HaveOccurred())
		Expect(originalComponent).NotTo(BeNil())
		componentName = originalComponent.Name
	}

	return componentName, originalComponent
}

func cleanup(f framework.Framework, testNamespace, applicationName, componentName string) {
	if !CurrentSpecReport().Failed() {
		Expect(f.AsKubeAdmin.HasController.DeleteApplication(applicationName, testNamespace, false)).To(Succeed())
		Expect(f.AsKubeAdmin.HasController.DeleteComponent(componentName, testNamespace, false)).To(Succeed())
		integrationTestScenarios, err := f.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(applicationName, testNamespace)
		Expect(err).ShouldNot(HaveOccurred())

		for _, testScenario := range *integrationTestScenarios {
			Expect(f.AsKubeAdmin.IntegrationController.DeleteIntegrationTestScenario(&testScenario, testNamespace)).To(Succeed())
		}
		Expect(f.SandboxController.DeleteUserSignup(f.UserName)).To(BeTrue())
	}
}
