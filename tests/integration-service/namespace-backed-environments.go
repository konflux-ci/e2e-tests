package integration

import (
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	"github.com/codeready-toolchain/api/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
	managedgitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend-shared/apis/managed-gitops/v1alpha1"
)

const (
	componentRepoName = "hacbs-test-project"
	componentRepoURL  = "https://github.com/redhat-appstudio-qe/" + componentRepoName
	EnvNameForNBE     = "user-picked-environment"
	gitURLForNBE      = "https://github.com/redhat-appstudio/integration-examples.git"
	revisionForNBE    = "main"
	pathInRepoForNBE  = "pipelines/integration_test_app.yaml"
)

var _ = framework.IntegrationServiceSuiteDescribe("Namespace-backed Environment (NBE) E2E tests", Label("integration-service", "HACBS", "namespace-backed-envs"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var applicationName, componentName, testNamespace string
	var pipelineRun, testPipelinerun *v1beta1.PipelineRun
	var originalComponent *appstudioApi.Component
	var snapshot, snapshot_push *appstudioApi.Snapshot
	var integrationTestScenario *integrationv1beta1.IntegrationTestScenario
	var env, ephemeralEnvironment, userPickedEnvironment *appstudioApi.Environment
	var dtcl *appstudioApi.DeploymentTargetClaimList
	var dtl *appstudioApi.DeploymentTargetList
	var godmel *managedgitopsv1alpha1.GitOpsDeploymentManagedEnvironmentList
	var godl *managedgitopsv1alpha1.GitOpsDeploymentList
	var phaseDTC appstudioApi.DeploymentTargetClaimPhase
	var phaseDT appstudioApi.DeploymentTargetPhase
	var sr *v1alpha1.SpaceRequestList
	var spc *v1alpha1.SpaceList
	var seb *appstudioApi.SnapshotEnvironmentBinding
	var kcc *appstudioApi.DeploymentTargetKubernetesClusterCredentials
	AfterEach(framework.ReportFailure(&f))

	Describe("with happy path for Namespace-backed environments", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("nbe-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			applicationName = createApp(*f, testNamespace)
			componentName, originalComponent = createComponent(*f, testNamespace, applicationName)
			Expect(originalComponent.Spec.Route).To(Equal(""))

			dtcls, err := f.AsKubeAdmin.GitOpsController.CreateDeploymentTargetClass()
			Expect(dtcls).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())

			userPickedEnvironment, err = f.AsKubeAdmin.GitOpsController.CreatePocEnvironment(EnvNameForNBE, testNamespace)
			Expect(err).ToNot(HaveOccurred())

			integrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenarioWithEnvironment(applicationName, testNamespace, gitURLForNBE, revisionForNBE, pathInRepoForNBE, userPickedEnvironment)
			Expect(err).ShouldNot(HaveOccurred())
			phaseDTC = appstudioApi.DeploymentTargetClaimPhase_Bound
			phaseDT = appstudioApi.DeploymentTargetPhase_Bound

			consoleRoute, err := f.AsKubeAdmin.CommonController.GetOpenshiftRoute("console", "openshift-console")
			Expect(err).ShouldNot(HaveOccurred())
			if utils.IsPrivateHostname(consoleRoute.Spec.Host) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentName)
			}

			Expect(f.AsKubeAdmin.GitOpsController.DeleteDeploymentTargetClass()).To(Succeed())
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
				Expect(err).ToNot(HaveOccurred())
			})

			It("checks if the Build PipelineRun got annotated with Snapshot name", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, "appstudio.openshift.io/snapshot")).To(Succeed())
			})
		})

		It("creates an Ephemeral Environment", func() {
			Eventually(func() error {
				ephemeralEnvironment, err = f.AsKubeAdmin.GitOpsController.GetEphemeralEnvironment(snapshot.Spec.Application, snapshot.Name, integrationTestScenario.Name, testNamespace)
				return err
			}, time.Minute*3, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out when waiting for the creation of Ephemeral Environment related to snapshot %s", snapshot.Name))
			Expect(err).ToNot(HaveOccurred())
			Expect(ephemeralEnvironment.Name).ToNot(BeEmpty())
		})

		It("checks for deploymentTargetClaim after Ephemeral env has been created", func() {
			Eventually(func() error {
				dtcl, err = f.AsKubeDeveloper.GitOpsController.GetDeploymentTargetClaimsList(testNamespace)
				if dtcl.Items == nil {
					return fmt.Errorf("DeploymentTargetClaim is nil.")
				}
				if dtcl.Items[0].Status.Phase != phaseDTC {
					return fmt.Errorf("DeploymentTargetClaimPhase is: " + string(dtcl.Items[0].Status.Phase) + " Expected: " + string(phaseDTC))
				}
				if dtcl.Items[0].Spec.DeploymentTargetClassName == "" {
					return fmt.Errorf("deploymentTargetClassName field within deploymentTargetClaim is empty.")
				}
				return err
			}, time.Minute*1, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out checking DeploymentTargetClaim after Ephemeral Environment %s was created ", ephemeralEnvironment.Name))
			Expect(err).ToNot(HaveOccurred())
		})

		It("checks for spaceRequest after Ephemeral env has been created", func() {
			Eventually(func() error {
				sr, err = f.AsKubeAdmin.GitOpsController.GetSpaceRequests(testNamespace)
				if sr.Items == nil {
					return fmt.Errorf("Space request is nil.")
				}
				if sr.Items[0].Status.Conditions[0].Type != v1alpha1.ConditionType("Ready") {
					return fmt.Errorf("Status condition for Space request is: " + string(sr.Items[0].Status.Conditions[0].Type) + " Expected: Ready")
				}
				return err
			}, time.Minute*1, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out checking GitOpsDeploymentManagedEnvironment after Ephemeral Environment was created %s", ephemeralEnvironment.Name))
			Expect(err).ToNot(HaveOccurred())
			Expect(sr.Items[0].Spec.TierName).ToNot(BeEmpty())
			Expect(sr.Items[0].Status.NamespaceAccess).ToNot(BeEmpty())
		})

		It("checks that space doesn't exist after Ephemeral env has been created", func() {
			spc, err = f.AsKubeAdmin.GitOpsController.GetSpaces(testNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(spc.Items).To(BeEmpty())
		})

		It("checks for deploymentTarget after Ephemeral env has been created", func() {
			Eventually(func() error {
				dtl, err = f.AsKubeAdmin.GitOpsController.GetDeploymentTargetsList(testNamespace)
				if dtl.Items == nil {
					return fmt.Errorf("No DeploymentTargets found, deploymentTargetList is nil.")
				}
				if dtl.Items[0].Spec.DeploymentTargetClassName == "" {
					return fmt.Errorf("deploymentTargetClassName field within DeploymentTarget is empty.")
				}
				if &dtl.Items[0].Spec.KubernetesClusterCredentials == kcc {
					return fmt.Errorf("KubernetesClusterCredentials within DeploymentTarget are empty.")
				}
				if dtl.Items[0].Status.Phase != phaseDT {
					return fmt.Errorf("DeploymentTargetPhase is: " + string(dtl.Items[0].Status.Phase) + " Expected: " + string(phaseDT))
				}
				return err
			}, time.Minute*1, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out checking DeploymentTarget after Ephemeral Environment %s was created ", ephemeralEnvironment.Name))
			Expect(err).ToNot(HaveOccurred())
		})

		It("checks for GitOpsDeploymentManagedEnvironment after Ephemeral env has been created", func() {
			Eventually(func() error {
				godmel, err = f.AsKubeAdmin.GitOpsController.GetGitOpsDeploymentManagedEnvironmentList(testNamespace)
				if godmel.Items == nil {
					return fmt.Errorf("No GitOpsDeploymentManagedEnvironments found, GitOpsDeploymentManagedEnvironmentList is nil.")
				}
				if godmel.Items[0].Status.Conditions[0].Type != "ConnectionInitializationSucceeded" {
					return fmt.Errorf("Status condition for GitOpsDeploymentManagedEnvironment is: " + string(godmel.Items[0].Status.Conditions[0].Type) + " Expected: ConnectionInitializationSucceeded")
				}
				return err
			}, time.Minute*1, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out checking GitOpsDeploymentManagedEnvironment after Ephemeral Environment was created %s", ephemeralEnvironment.Name))
			Expect(godmel.Items[0].Spec.ClusterCredentialsSecret).ToNot(BeEmpty())
			Expect(godmel.Items[0].Spec.APIURL).ToNot(BeEmpty())
			Expect(godmel.Items[0].Spec.AllowInsecureSkipTLSVerify).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})

		It("checks for GitOpsDeployments after Ephemeral env has been created", func() {
			godl, err = f.AsKubeAdmin.GitOpsController.ListAllGitOpsDeployments(testNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(godl.Items).ToNot(BeNil())
			Expect(godl.Items[0].Spec.Source.RepoURL).ToNot(BeEmpty())
			Expect(godl.Items[0].Spec.Source.Path).ToNot(BeEmpty())
			Expect(godl.Items[0].Spec.Source.TargetRevision).To(Equal("main"))
			Expect(godl.Items[0].Spec.Type).To(Equal("automated"))
			Expect(godl.Items[0].Status.ReconciledState).ToNot(BeNil())
		})

		It("checks for SEB after Ephemeral env has been created", func() {
			seb, err = f.AsKubeAdmin.CommonController.GetSnapshotEnvironmentBinding(applicationName, testNamespace, ephemeralEnvironment)
			Expect(err).ToNot(HaveOccurred())
			Expect(seb).ToNot(BeNil())
			Expect(seb.Spec.Snapshot).To(Equal(snapshot.Name))
			Expect(seb.Spec.Application).To(Equal(applicationName))
			Expect(seb.Spec.Environment).To(Equal(ephemeralEnvironment.Name))
			Expect(seb.Spec.Components).ToNot(BeEmpty())
		})

		It("should find the related Integration Test PipelineRun", func() {
			testPipelinerun, err = f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(integrationTestScenario.Name, snapshot.Name, testNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(testPipelinerun.Labels["appstudio.openshift.io/snapshot"]).To(ContainSubstring(snapshot.Name))
			Expect(testPipelinerun.Labels["test.appstudio.openshift.io/scenario"]).To(ContainSubstring(integrationTestScenario.Name))
			Expect(testPipelinerun.Labels["appstudio.openshift.io/environment"]).To(ContainSubstring(ephemeralEnvironment.Name))
		})

		When("Integration Test PipelineRun is created", func() {
			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenario, snapshot, testNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a integration pipeline for snapshot %s/%s to finish", testNamespace, snapshot.GetName()))
			})
		})

		When("Integration Test PipelineRun completes successfully", func() {
			It("should lead to Snapshot CR being marked as passed", FlakeAttempts(3), func() {
				Eventually(func() bool {
					snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", testNamespace)
					return err == nil && f.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot)
				}, time.Minute*3, time.Second*5).Should(BeTrue(), fmt.Sprintf("Timed out waiting for Snapshot to be marked as succeeded %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
			})

			It("should lead to SnapshotEnvironmentBinding getting deleted", func() {
				Eventually(func() error {
					_, err = f.AsKubeAdmin.CommonController.GetSnapshotEnvironmentBinding(applicationName, testNamespace, ephemeralEnvironment)
					return err
				}, time.Minute*3, time.Second*5).ShouldNot(Succeed(), fmt.Sprintf("timed out when waiting for SnapshotEnvironmentBinding to be deleted for application %s/%s", testNamespace, applicationName))
				Expect(err.Error()).To(ContainSubstring(constants.SEBAbsenceErrorString))
			})

			It("should lead to ephemeral environment getting deleted", func() {
				Eventually(func() error {
					ephemeralEnvironment, err = f.AsKubeAdmin.GitOpsController.GetEphemeralEnvironment(snapshot.Spec.Application, snapshot.Name, integrationTestScenario.Name, testNamespace)
					return err
				}, time.Minute*3, time.Second*1).ShouldNot(Succeed(), fmt.Sprintf("timed out when waiting for the Ephemeral Environment %s to be deleted", ephemeralEnvironment.Name))
				Expect(err.Error()).To(ContainSubstring(constants.EphemeralEnvAbsenceErrorString))
			})
		})
	})

	Describe("when valid DeploymentTargetClass doesn't exist", Ordered, func() {
		var integrationTestScenario *integrationv1beta1.IntegrationTestScenario
		BeforeAll(func() {
			// Initialize the tests controllers
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("nbe-neg"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			applicationName = createApp(*f, testNamespace)
			componentName, originalComponent = createComponent(*f, testNamespace, applicationName)

			env, err = f.AsKubeAdmin.GitOpsController.CreatePocEnvironment(EnvNameForNBE, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			integrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenarioWithEnvironment(applicationName, testNamespace, gitURLForNBE, revisionForNBE, pathInRepoForNBE, env)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				cleanup(*f, testNamespace, applicationName, componentName)

				Expect(f.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(testNamespace, 30*time.Second)).To(Succeed())
				Expect(f.AsKubeAdmin.IntegrationController.DeleteSnapshot(snapshot_push, testNamespace)).To(Succeed())
			}
		})

		It("valid deploymentTargetClass doesn't exist", func() {
			validDTCLS, err := f.AsKubeAdmin.GitOpsController.HaveAvailableDeploymentTargetClassExist()
			Expect(validDTCLS).To(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates a snapshot of push event", func() {
			sampleImage := "quay.io/redhat-appstudio/sample-image@sha256:841328df1b9f8c4087adbdcfec6cc99ac8308805dea83f6d415d6fb8d40227c1"
			snapshot_push, err = f.AsKubeAdmin.IntegrationController.CreateSnapshotWithImage(componentName, applicationName, testNamespace, sampleImage)
			Expect(err).ToNot(HaveOccurred())
			GinkgoWriter.Printf("snapshot %s is found\n", snapshot_push.Name)
		})

		When("nonexisting valid deploymentTargetClass", func() {
			It("check no GitOpsCR is created for the dtc with nonexisting deploymentTargetClass", func() {
				spaceRequestList, err := f.AsKubeAdmin.GitOpsController.GetSpaceRequests(testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(spaceRequestList.Items).To(BeEmpty())

				deploymentTargetList, err := f.AsKubeAdmin.GitOpsController.GetDeploymentTargetsList(testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(deploymentTargetList.Items).To(BeEmpty())

				deploymentTargetClaimList, err := f.AsKubeAdmin.GitOpsController.GetDeploymentTargetClaimsList(testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(deploymentTargetClaimList.Items).To(BeEmpty())

				environmentList, err := f.AsKubeAdmin.GitOpsController.GetEnvironmentsList(testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(environmentList.Items)).ToNot(BeNumerically(">", 2))

				pipelineRun, err := f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot_push.Name, testNamespace)
				Expect(pipelineRun.Name == "" && strings.Contains(err.Error(), "no pipelinerun found")).To(BeTrue())
			})

			It("checks if snapshot is not marked as passed", func() {
				snapshot, err := f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot_push.Name, "", "", testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(f.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot)).To(BeFalse())
			})
		})
	})
})
