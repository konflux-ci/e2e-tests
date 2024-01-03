package integration

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/redhat-appstudio/operator-toolkit/metadata"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/codeready-toolchain/api/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
	managedgitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend-shared/apis/managed-gitops/v1alpha1"
)

var _ = framework.IntegrationServiceSuiteDescribe("Namespace-backed Environment (NBE) E2E tests", Label("integration-service", "HACBS", "namespace-backed-envs"), func() {
	defer GinkgoRecover()

	var f *framework.Framework
	var err error

	var applicationName, componentName, testNamespace string
	var pipelineRun, testPipelinerun *tektonv1.PipelineRun
	var originalComponent *appstudioApi.Component
	var snapshot, snapshot_push *appstudioApi.Snapshot
	var integrationTestScenario *integrationv1beta1.IntegrationTestScenario
	var newIntegrationTestScenario *integrationv1beta1.IntegrationTestScenario
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

			integrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenarioWithEnvironment(applicationName, testNamespace, gitURL, revisionForNBE, pathInRepoForNBE, userPickedEnvironment)
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
			Expect(f.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(originalComponent, "",
				f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true})).To(Succeed())

			Expect(pipelineRun.Annotations[snapshotAnnotation]).To(Equal(""))
		})

		When("the build pipelineRun run succeeded", func() {
			It("checks if the BuildPipelineRun have the annotation of chains signed", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, chainsSignedAnnotation)).To(Succeed())
			})

			It("checks if the Snapshot is created", func() {
				snapshot, err = f.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentName, testNamespace)
				Expect(err).ToNot(HaveOccurred())
			})

			It("checks if the Build PipelineRun got annotated with Snapshot name", func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, snapshotAnnotation)).To(Succeed())
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
				Expect(err).ToNot(HaveOccurred())
				if len(dtcl.Items) == 0 {
					return fmt.Errorf("no DeploymentTargetClaim is found")
				}
				if !reflect.ValueOf(dtcl.Items[0].Status).IsZero() && dtcl.Items[0].Status.Phase != phaseDTC {
					return fmt.Errorf("DeploymentTargetClaimPhase is not yet equal to the expected phase: " + string(phaseDTC))
				}
				if dtcl.Items[0].Spec.DeploymentTargetClassName == "" {
					return fmt.Errorf("deploymentTargetClassName field within deploymentTargetClaim is empty")
				}
				return nil
			}, time.Minute*3, time.Second*5).Should(BeNil(), fmt.Sprintf("timed out checking DeploymentTargetClaim after Ephemeral Environment %s was created ", ephemeralEnvironment.Name))

		})

		It("checks for spaceRequest after Ephemeral env has been created", func() {
			Eventually(func() error {
				sr, err = f.AsKubeAdmin.GitOpsController.GetSpaceRequests(testNamespace)
				Expect(err).ToNot(HaveOccurred())
				if len(sr.Items) == 0 {
					return fmt.Errorf("No Space request is found.")
				}
				if sr.Items[0].Status.Conditions != nil && sr.Items[0].Status.Conditions[0].Type != v1alpha1.ConditionType("Ready") {
					return fmt.Errorf("Status condition for Space request is not yet equal to the expected type: Ready")
				}
				return err
			}, time.Minute*1, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out checking GitOpsDeploymentManagedEnvironment after Ephemeral Environment was created %s", ephemeralEnvironment.Name))
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
				Expect(err).ToNot(HaveOccurred())
				if len(dtl.Items) == 0 {
					return fmt.Errorf("No DeploymentTargets found, deploymentTargetList is nil.")
				}
				if dtl.Items[0].Spec.DeploymentTargetClassName == "" {
					return fmt.Errorf("deploymentTargetClassName field within DeploymentTarget is empty.")
				}
				if &dtl.Items[0].Spec.KubernetesClusterCredentials == kcc {
					return fmt.Errorf("KubernetesClusterCredentials within DeploymentTarget are empty.")
				}
				if !reflect.ValueOf(dtl.Items[0].Status).IsZero() && dtl.Items[0].Status.Phase != phaseDT {
					return fmt.Errorf("DeploymentTargetPhase is not yet equal to the expected phase: " + string(phaseDT))
				}
				return err
			}, time.Minute*1, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out checking DeploymentTarget after Ephemeral Environment %s was created ", ephemeralEnvironment.Name))
		})

		It("checks for GitOpsDeploymentManagedEnvironment after Ephemeral env has been created", func() {
			Eventually(func() error {
				godmel, err = f.AsKubeAdmin.GitOpsController.GetGitOpsDeploymentManagedEnvironmentList(testNamespace)
				Expect(err).ToNot(HaveOccurred())
				if len(godmel.Items) == 0 {
					return fmt.Errorf("No GitOpsDeploymentManagedEnvironments found, GitOpsDeploymentManagedEnvironmentList is empty.")
				}
				if godmel.Items[0].Status.Conditions != nil && !meta.IsStatusConditionTrue(godmel.Items[0].Status.Conditions, "ConnectionInitializationSucceeded") {
					return fmt.Errorf("The GitOpsDeploymentManagedEnvironment doesn't have the ConnectionInitializationSucceeded status condition set to true.")
				}
				return err
			}, time.Minute*1, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out checking GitOpsDeploymentManagedEnvironment after Ephemeral Environment was created %s", ephemeralEnvironment.Name))
			Expect(godmel.Items[0].Spec.ClusterCredentialsSecret).ToNot(BeEmpty())
			Expect(godmel.Items[0].Spec.APIURL).ToNot(BeEmpty())
			Expect(godmel.Items[0].Spec.AllowInsecureSkipTLSVerify).ToNot(BeNil())
		})

		It("checks for GitOpsDeployments after Ephemeral env has been created", func() {
			Eventually(func() error {
				godl, err = f.AsKubeAdmin.GitOpsController.ListAllGitOpsDeployments(testNamespace)
				Expect(err).ToNot(HaveOccurred())
				if len(godl.Items) == 0 {
					return fmt.Errorf("No GitOpsDeployments found.")
				}
				if !reflect.ValueOf(godl.Items[0].Status).IsZero() && reflect.ValueOf(godl.Items[0].Status.ReconciledState).IsZero() {
					return fmt.Errorf("ReconciledState doesn't exist yet for GitOpsDeployment.")
				}
				return err
			}, time.Minute*1, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out checking GitOpsDeployments after Ephemeral Environment was created %s", ephemeralEnvironment.Name))

			Expect(godl.Items[0].Spec.Source.RepoURL).ToNot(BeEmpty())
			Expect(godl.Items[0].Spec.Source.Path).ToNot(BeEmpty())
			Expect(godl.Items[0].Spec.Source.TargetRevision).To(Equal("main"))
			Expect(godl.Items[0].Spec.Type).To(Equal("automated"))
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
			Expect(testPipelinerun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
			Expect(testPipelinerun.Labels[scenarioAnnotation]).To(ContainSubstring(integrationTestScenario.Name))
			Expect(testPipelinerun.Labels[environmentLabel]).To(ContainSubstring(ephemeralEnvironment.Name))
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
				}, time.Minute*5, time.Second*5).Should(BeTrue(), fmt.Sprintf("Timed out waiting for Snapshot to be marked as succeeded %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
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

		It("creates a new IntegrationTestScenario with ephemeral environment", func() {
			var err error
			newIntegrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenarioWithEnvironment(applicationName, testNamespace, gitURL, revisionForNBE, pathInRepoForNBE, userPickedEnvironment)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("updates the Snapshot with the re-run label for the new scenario", FlakeAttempts(3), func() {
			updatedSnapshot := snapshot.DeepCopy()
			err := metadata.AddLabels(updatedSnapshot, map[string]string{snapshotRerunLabel: newIntegrationTestScenario.Name})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(f.AsKubeAdmin.IntegrationController.PatchSnapshot(snapshot, updatedSnapshot)).Should(Succeed())
			Expect(metadata.GetLabelsWithPrefix(updatedSnapshot, snapshotRerunLabel)).NotTo(BeEmpty())
		})

		When("An snapshot is updated with a re-run label for a given scenario", func() {
			It("checks if the re-run label was removed from the Snapshot", func() {
				Eventually(func() error {
					snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", testNamespace)
					if err != nil {
						return fmt.Errorf("encountered error while getting Snapshot %s/%s: %w", snapshot.Name, snapshot.Namespace, err)
					}

					if metadata.HasLabel(snapshot, snapshotRerunLabel) {
						return fmt.Errorf("the Snapshot %s/%s shouldn't contain the %s label", snapshot.Name, snapshot.Namespace, snapshotRerunLabel)
					}
					return nil
				}, time.Minute*2, time.Second*5).Should(Succeed())
			})

			It("creates an Ephemeral Environment", func() {
				Eventually(func() error {
					ephemeralEnvironment, err = f.AsKubeAdmin.GitOpsController.GetEphemeralEnvironment(snapshot.Spec.Application, snapshot.Name, newIntegrationTestScenario.Name, testNamespace)
					return err
				}, time.Minute*3, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out when waiting for the creation of Ephemeral Environment related to snapshot %s", snapshot.Name))
				Expect(err).ToNot(HaveOccurred())
				Expect(ephemeralEnvironment.Name).ToNot(BeEmpty())
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

			It("checks if the new integration pipelineRun started", Label("slow"), func() {
				reRunPipelineRun, err := f.AsKubeDeveloper.IntegrationController.WaitForIntegrationPipelineToGetStarted(newIntegrationTestScenario.Name, snapshot.Name, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(reRunPipelineRun).ShouldNot(BeNil())

				Expect(reRunPipelineRun.Labels[snapshotAnnotation]).To(ContainSubstring(snapshot.Name))
				Expect(reRunPipelineRun.Labels[scenarioAnnotation]).To(ContainSubstring(newIntegrationTestScenario.Name))
				Expect(reRunPipelineRun.Labels[environmentLabel]).To(ContainSubstring(ephemeralEnvironment.Name))
			})

			It("checks if all integration pipelineRuns finished successfully", Label("slow"), func() {
				Expect(f.AsKubeDeveloper.IntegrationController.WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName, snapshot)).To(Succeed())
			})

			It("checks if the name of the re-triggered pipelinerun is reported in the Snapshot", FlakeAttempts(3), func() {
				Eventually(func(g Gomega) {
					snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot(snapshot.Name, "", "", testNamespace)
					g.Expect(err).ShouldNot(HaveOccurred())

					statusDetail, err := f.AsKubeDeveloper.IntegrationController.GetIntegrationTestStatusDetailFromSnapshot(snapshot, newIntegrationTestScenario.Name)
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(statusDetail).NotTo(BeNil())

					integrationPipelineRun, err := f.AsKubeDeveloper.IntegrationController.GetIntegrationPipelineRun(newIntegrationTestScenario.Name, snapshot.Name, testNamespace)
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(integrationPipelineRun).NotTo(BeNil())

					g.Expect(statusDetail.TestPipelineRunName).To(Equal(integrationPipelineRun.Name))
				}, time.Minute*2, time.Second*5).Should(Succeed())
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
					ephemeralEnvironment, err = f.AsKubeAdmin.GitOpsController.GetEphemeralEnvironment(snapshot.Spec.Application, snapshot.Name, newIntegrationTestScenario.Name, testNamespace)
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
			integrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenarioWithEnvironment(applicationName, testNamespace, gitURL, revisionForNBE, pathInRepoForNBE, env)
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
