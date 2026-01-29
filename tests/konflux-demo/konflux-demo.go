package konflux_demo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	buildcontrollers "github.com/konflux-ci/build-service/controllers"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/conforma/crds/api/v1alpha1"
	"github.com/google/go-github/v44/github"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	kubeapi "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
	e2eConfig "github.com/konflux-ci/e2e-tests/tests/konflux-demo/config"
)

var _ = framework.KonfluxDemoSuiteDescribe(ginkgo.Label(devEnvTestLabel), func() {
	defer ginkgo.GinkgoRecover()

	var timeout, interval time.Duration
	var userNamespace string
	var err error

	var managedNamespace string

	var component *appservice.Component
	var release *releaseApi.Release
	var snapshot *appservice.Snapshot
	var pipelineRun, testPipelinerun *tektonapi.PipelineRun
	var integrationTestScenario *integrationv1beta2.IntegrationTestScenario

	// PaC related variables
	var prNumber int
	var headSHA, pacBranchName string
	var mergeResult *github.PullRequestMergeResult

	//secret := &corev1.Secret{}

	fw := &framework.Framework{}

	var buildPipelineAnnotation map[string]string

	var componentNewBaseBranch, gitRevision, componentRepositoryName, componentName string

	var appSpecs []e2eConfig.ApplicationSpec
	if strings.Contains(ginkgo.GinkgoLabelFilter(), upstreamKonfluxTestLabel) {
		appSpecs = e2eConfig.UpstreamAppSpecs
	} else {
		appSpecs = e2eConfig.ApplicationSpecs
	}

	for _, appSpec := range appSpecs {
		appSpec := appSpec
		if appSpec.Skip {
			continue
		}

		ginkgo.Describe(appSpec.Name, ginkgo.Ordered, func() {
			ginkgo.BeforeAll(func() {
				if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
					ginkgo.Skip("Skipping this test due to configuration issue with Spray proxy")
				}
				fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devEnvTestLabel))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				userNamespace = fw.UserNamespace
				managedNamespace = userNamespace + "-managed"

				// Component config
				componentName = fmt.Sprintf("%s-%s", appSpec.ComponentSpec.Name, util.GenerateRandomString(4))
				pacBranchName = fmt.Sprintf("%s%s", constants.PaCPullRequestBranchPrefix, componentName)
				componentRepositoryName = utils.ExtractGitRepositoryNameFromURL(appSpec.ComponentSpec.GitSourceUrl)

				// Secrets config
				// https://issues.redhat.com/browse/KFLUXBUGS-1462 - creating SCM secret alongside with PaC
				// leads to PLRs being duplicated
				// secretDefinition := build.GetSecretDefForGitHub(namespace)
				// secret, err = fw.AsKubeAdmin.CommonController.CreateSecret(namespace, secretDefinition)
				sharedSecret, err := fw.AsKubeAdmin.CommonController.GetSecret(constants.QuayRepositorySecretNamespace, constants.QuayRepositorySecretName)
				if err != nil && k8sErrors.IsNotFound(err) {
					sharedSecret, err = CreateE2EQuaySecret(fw.AsKubeAdmin.CommonController.CustomClient)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error when getting shared secret - make sure the secret %s in %s userNamespace is created", constants.QuayRepositorySecretName, constants.QuayRepositorySecretNamespace))

				createReleaseConfig(fw.AsKubeAdmin, managedNamespace, userNamespace, appSpec.ComponentSpec.Name, appSpec.ApplicationName, sharedSecret.Data[".dockerconfigjson"])

				// When RELEASE_CATALOG_TA_QUAY_TOKEN is set, create and link the TA Quay secret so the release
				// pipeline can push to quay.io/konflux-ci/release-service-trusted-artifacts (same as happy_path
				// and push_to_external_registry). Aligns with openshift/release and infra-deployments e2e flow.
				taToken := utils.GetEnv("RELEASE_CATALOG_TA_QUAY_TOKEN", "")
				if taToken != "" {
					_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(releasecommon.ReleaseCatalogTAQuaySecret, managedNamespace, taToken)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.ReleaseCatalogTAQuaySecret, "release-service-account", true)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					ginkgo.GinkgoWriter.Printf("created and linked release-catalog-trusted-artifacts-quay-secret in namespace %q\n", managedNamespace)
				} else {
					ginkgo.GinkgoWriter.Printf("RELEASE_CATALOG_TA_QUAY_TOKEN not set, skipping TA Quay secret (release pipeline may fail on trusted-artifact push)\n")
				}

				// get the build pipeline bundle annotation
				buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(appSpec.ComponentSpec.BuildPipelineType)

			})

			// Remove all resources created by the tests
			ginkgo.AfterAll(func() {
				if !(strings.EqualFold(os.Getenv("E2E_SKIP_CLEANUP"), "true")) && !ginkgo.CurrentSpecReport().Failed() && !strings.Contains(ginkgo.GinkgoLabelFilter(), upstreamKonfluxTestLabel) {
					gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(userNamespace)).To(gomega.Succeed())
					gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(gomega.Succeed())

					// Delete new branch created by PaC and a testing branch used as a component's base branch
					err = fw.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepositoryName, pacBranchName)
					if err != nil {
						gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
					}
					err = fw.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepositoryName, componentNewBaseBranch)
					if err != nil {
						gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
					}
					gomega.Expect(build.CleanupWebhooks(fw, componentRepositoryName)).ShouldNot(gomega.HaveOccurred())
				}
			})

			// Create an application in a specific namespace
			ginkgo.It("creates an application", ginkgo.Label(devEnvTestLabel, upstreamKonfluxTestLabel), func() {
				createdApplication, err := fw.AsKubeAdmin.HasController.CreateApplication(appSpec.ApplicationName, userNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(createdApplication.Spec.DisplayName).To(gomega.Equal(appSpec.ApplicationName))
				gomega.Expect(createdApplication.Namespace).To(gomega.Equal(userNamespace))
			})

			// Create an IntegrationTestScenario for the App
			ginkgo.It("creates an IntegrationTestScenario for the app", ginkgo.Label(devEnvTestLabel, upstreamKonfluxTestLabel), func() {
				its := appSpec.ComponentSpec.IntegrationTestScenario
				// Use Eventually to handle race condition where admission webhook hasn't indexed the application yet
				gomega.Eventually(func() error {
					var err error
					integrationTestScenario, err = fw.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", appSpec.ApplicationName, userNamespace, its.GitURL, its.GitRevision, its.TestPath, "", []string{})
					return err
				}, time.Minute*2, time.Second*5).Should(gomega.Succeed())
			})

			ginkgo.It("creates new branch for the build", ginkgo.Label(devEnvTestLabel, upstreamKonfluxTestLabel), func() {
				// We need to create a new branch that we will target
				// and that will contain the PaC configuration, so we
				// can avoid polluting the default (main) branch
				componentNewBaseBranch = fmt.Sprintf("base-%s", util.GenerateRandomString(6))
				gitRevision = componentNewBaseBranch
				gomega.Expect(fw.AsKubeAdmin.CommonController.Github.CreateRef(componentRepositoryName, appSpec.ComponentSpec.GitSourceDefaultBranchName, appSpec.ComponentSpec.GitSourceRevision, componentNewBaseBranch)).To(gomega.Succeed())
			})

			// Component are imported from gitUrl
			ginkgo.It(fmt.Sprintf("creates component %s (private: %t) from git source %s", appSpec.ComponentSpec.Name, appSpec.ComponentSpec.Private, appSpec.ComponentSpec.GitSourceUrl), ginkgo.Label(devEnvTestLabel, upstreamKonfluxTestLabel), func() {
				componentObj := appservice.ComponentSpec{
					ComponentName: componentName,
					Application:   appSpec.ApplicationName,
					Source: appservice.ComponentSource{
						ComponentSourceUnion: appservice.ComponentSourceUnion{
							GitSource: &appservice.GitSource{
								URL:           appSpec.ComponentSpec.GitSourceUrl,
								Revision:      gitRevision,
								Context:       appSpec.ComponentSpec.GitSourceContext,
								DockerfileURL: appSpec.ComponentSpec.DockerFilePath,
							},
						},
					},
				}

				component, err = fw.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, userNamespace, "", "", appSpec.ApplicationName, false, utils.MergeMaps(constants.ComponentPaCRequestAnnotation, buildPipelineAnnotation))
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.When("Component is created", ginkgo.Label(upstreamKonfluxTestLabel), func() {
				ginkgo.It("triggers creation of a PR in the sample repo", func() {
					var prSHA string
					gomega.Eventually(func() error {
						prs, err := fw.AsKubeAdmin.CommonController.Github.ListPullRequests(componentRepositoryName)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						for _, pr := range prs {
							if pr.Head.GetRef() == pacBranchName {
								prNumber = pr.GetNumber()
								prSHA = pr.GetHead().GetSHA()
								return nil
							}
						}
						return fmt.Errorf("could not get the expected PaC branch name %s", pacBranchName)
					}, pullRequestCreationTimeout, defaultPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for init PaC PR (branch %q) to be created against the %q repo", pacBranchName, componentRepositoryName))

					// We don't need the PipelineRun from a PaC 'pull-request' event to finish, so we can delete it
					gomega.Eventually(func() error {
						pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, prSHA)
						if err == nil {
							gomega.Expect(fw.AsKubeAdmin.TektonController.DeletePipelineRun(pipelineRun.Name, pipelineRun.Namespace)).To(gomega.Succeed())
							return nil
						}
						return err
					}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for `pull-request` event type PaC PipelineRun to be present in the user namespace %q for component %q with a label pointing to %q", userNamespace, component.GetName(), appSpec.ApplicationName))
				})

				ginkgo.It("verifies component build status", func() {
					var buildStatus *buildcontrollers.BuildStatus
					gomega.Eventually(func() (bool, error) {
						component, err := fw.AsKubeAdmin.HasController.GetComponent(component.GetName(), userNamespace)
						if err != nil {
							return false, err
						}

						statusBytes := []byte(component.Annotations[buildcontrollers.BuildStatusAnnotationName])

						err = json.Unmarshal(statusBytes, &buildStatus)
						if err != nil {
							return false, err
						}

						if buildStatus.PaC != nil {
							ginkgo.GinkgoWriter.Printf("state: %s\n", buildStatus.PaC.State)
							ginkgo.GinkgoWriter.Printf("mergeUrl: %s\n", buildStatus.PaC.MergeUrl)
							ginkgo.GinkgoWriter.Printf("errId: %d\n", buildStatus.PaC.ErrId)
							ginkgo.GinkgoWriter.Printf("errMessage: %s\n", buildStatus.PaC.ErrMessage)
							ginkgo.GinkgoWriter.Printf("configurationTime: %s\n", buildStatus.PaC.ConfigurationTime)
						} else {
							ginkgo.GinkgoWriter.Println("build status does not have PaC field")
						}

						return buildStatus.PaC != nil && buildStatus.PaC.State == "enabled" && buildStatus.PaC.MergeUrl != "" && buildStatus.PaC.ErrId == 0 && buildStatus.PaC.ConfigurationTime != "", nil
					}, timeout, interval).Should(gomega.BeTrue(), "component build status has unexpected content")
				})

				ginkgo.It("should eventually lead to triggering a 'push' event type PipelineRun after merging the PaC init branch ", func() {
					gomega.Eventually(func() error {
						mergeResult, err = fw.AsKubeAdmin.CommonController.Github.MergePullRequest(componentRepositoryName, prNumber)
						return err
					}, mergePRTimeout).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error when merging PaC pull request: %+v\n", err))

					headSHA = mergeResult.GetSHA()

					gomega.Eventually(func() error {
						pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, headSHA)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun has not been created yet for component %s/%s\n", userNamespace, component.GetName())
							return err
						}
						if !pipelineRun.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for a PipelineRun in namespace %q with label component label %q and application label %q and sha label %q to start", userNamespace, component.GetName(), appSpec.ApplicationName, headSHA))
				})
			})

			ginkgo.When("Build PipelineRun is created", ginkgo.Label(upstreamKonfluxTestLabel), func() {
				ginkgo.It("does not contain an annotation with a Snapshot Name", func() {
					gomega.Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(gomega.Equal(""))
				})
				ginkgo.It("should eventually complete successfully", func() {
					gomega.Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "build", headSHA, "",
						fw.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 5, Always: true}, pipelineRun)).To(gomega.Succeed())

					// in case the first pipelineRun attempt has failed and was retried, we need to update the git branch head ref
					headSHA = pipelineRun.Labels["pipelinesascode.tekton.dev/sha"]
				})
			})

			ginkgo.When("Build PipelineRun completes successfully", func() {

				ginkgo.It("should validate Tekton TaskRun test results successfully", func() {
					pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, headSHA)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(build.ValidateBuildPipelineTestResults(pipelineRun, fw.AsKubeAdmin.CommonController.KubeRest(), false)).To(gomega.Succeed())
				})

				ginkgo.It("should validate that the build pipelineRun is signed", ginkgo.Label(upstreamKonfluxTestLabel), func() {
					gomega.Eventually(func() error {
						pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, headSHA)
						if err != nil {
							return err
						}
						if pipelineRun.Annotations["chains.tekton.dev/signed"] != "true" {
							return fmt.Errorf("pipelinerun %s/%s does not have the expected value of annotation 'chains.tekton.dev/signed'", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, time.Minute*5, time.Second*5).Should(gomega.Succeed(), "failed while validating build pipelineRun is signed")

				})

				ginkgo.It("should find the related Snapshot CR", ginkgo.Label(upstreamKonfluxTestLabel), func() {
					gomega.Eventually(func() error {
						snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", userNamespace)
						return err
					}, snapshotTimeout, snapshotPollingInterval).Should(gomega.Succeed(), "timed out when trying to check if the Snapshot exists for PipelineRun %s/%s", userNamespace, pipelineRun.GetName())
				})

				ginkgo.It("should validate that the build pipelineRun is annotated with the name of the Snapshot", ginkgo.Label(upstreamKonfluxTestLabel), func() {
					pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, headSHA)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(gomega.Equal(snapshot.GetName()))
				})

				ginkgo.It("should find the related Integration Test PipelineRun", ginkgo.Label(upstreamKonfluxTestLabel), func() {
					gomega.Eventually(func() error {
						testPipelinerun, err = fw.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot.Name, userNamespace)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("failed to get Integration test PipelineRun for a snapshot '%s' in '%s' namespace: %+v\n", snapshot.Name, userNamespace, err)
							return err
						}
						if !testPipelinerun.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", testPipelinerun.GetNamespace(), testPipelinerun.GetName())
						}
						return nil
					}, pipelineRunStartedTimeout, defaultPollingInterval).Should(gomega.Succeed())
					gomega.Expect(testPipelinerun.Labels["appstudio.openshift.io/snapshot"]).To(gomega.ContainSubstring(snapshot.Name))
					gomega.Expect(testPipelinerun.Labels["test.appstudio.openshift.io/scenario"]).To(gomega.ContainSubstring(integrationTestScenario.Name))
				})
			})

			ginkgo.When("push pipelinerun is retriggered", func() {
				ginkgo.It("should eventually succeed", func() {
					gomega.Expect(fw.AsKubeAdmin.HasController.SetComponentAnnotation(component.GetName(), buildcontrollers.BuildRequestAnnotationName, buildcontrollers.BuildRequestTriggerPaCBuildAnnotationValue, userNamespace)).To(gomega.Succeed())
					// Check the pipelinerun is triggered
					gomega.Eventually(func() error {
						testPipelinerun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRunWithType(component.GetName(), appSpec.ApplicationName, userNamespace, "build", "", "incoming")
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun is not been retriggered yet for the component %s/%s\n", userNamespace, component.GetName())
							return err
						}
						if !testPipelinerun.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't been started yet", testPipelinerun.GetNamespace(), testPipelinerun.GetName())
						}
						return nil
					}, 10*time.Minute, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to retrigger for the component %s/%s", userNamespace, component.GetName()))
					// Should succeed
					gomega.Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "build", "", "incoming", fw.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, testPipelinerun)).To(gomega.Succeed())
				})
			})

			ginkgo.When("Integration Test PipelineRun is created", ginkgo.Label(upstreamKonfluxTestLabel), func() {
				ginkgo.It("should eventually complete successfully", func() {
					gomega.Expect(fw.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenario, snapshot, userNamespace)).To(gomega.Succeed(), fmt.Sprintf("Error when waiting for a integration pipeline for snapshot %s/%s to finish", userNamespace, snapshot.GetName()))
				})
			})

			ginkgo.When("Integration Test PipelineRun completes successfully", ginkgo.Label(upstreamKonfluxTestLabel), func() {
				ginkgo.It("should lead to Snapshot CR being marked as passed", func() {
					gomega.Eventually(func() bool {
						snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", userNamespace)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						return fw.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot)
					}, time.Minute*5, defaultPollingInterval).Should(gomega.BeTrue(), fmt.Sprintf("tests have not succeeded for snapshot %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
				})

				ginkgo.It("should trigger creation of Release CR", func() {
					gomega.Eventually(func() error {
						release, err = fw.AsKubeAdmin.ReleaseController.GetRelease("", snapshot.Name, userNamespace)
						return err
					}, releaseTimeout, releasePollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when trying to check if the release exists for snapshot %s/%s", userNamespace, snapshot.GetName()))
				})
			})

			ginkgo.When("Release CR is created", ginkgo.Label(upstreamKonfluxTestLabel), func() {
				ginkgo.It("triggers creation of Release PipelineRun", func() {
					gomega.Eventually(func() error {
						pipelineRun, err = fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("pipelineRun for component '%s' in namespace '%s' not created yet: %+v\n", component.GetName(), managedNamespace, err)
							return err
						}
						if !pipelineRun.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, pipelineRunStartedTimeout, defaultPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("failed to get pipelinerun named %q in namespace %q with label to release %q in namespace %q to start", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
				})
			})

			ginkgo.When("Release PipelineRun is triggered", ginkgo.Label(upstreamKonfluxTestLabel), func() {
				ginkgo.It("should eventually succeed", func() {
					gomega.Eventually(func() error {
						pr, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						gomega.Expect(tekton.HasPipelineRunFailed(pr)).NotTo(gomega.BeTrue(), fmt.Sprintf("did not expect PipelineRun %s/%s to fail", pr.GetNamespace(), pr.GetName()))
						if !pr.IsDone() {
							return fmt.Errorf("release pipelinerun %s/%s has not finished yet", pr.GetNamespace(), pr.GetName())
						}
						gomega.Expect(tekton.HasPipelineRunSucceeded(pr)).To(gomega.BeTrue(), fmt.Sprintf("PipelineRun %s/%s did not succeed", pr.GetNamespace(), pr.GetName()))
						return nil
					}, releasePipelineTimeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("failed to see pipelinerun %q in namespace %q with a label pointing to release %q in namespace %q to complete successfully", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
				})
			})

			ginkgo.When("Release PipelineRun is completed", ginkgo.Label(upstreamKonfluxTestLabel), func() {
				ginkgo.It("should lead to Release CR being marked as succeeded", func() {
					gomega.Eventually(func() error {
						release, err = fw.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						if !release.IsReleased() {
							return fmt.Errorf("release CR %s/%s is not marked as finished yet", release.GetNamespace(), release.GetName())
						}
						return nil
					}, customResourceUpdateTimeout, defaultPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("failed to see release %q in namespace %q get marked as released", release.Name, userNamespace))
				})
			})
		})
	}
})

func createReleaseConfig(kubeadminClient *framework.ControllerHub, managedNamespace, userNamespace, componentName, appName string, secretData []byte) {
	var err error
	_, err = kubeadminClient.CommonController.CreateTestNamespace(managedNamespace)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "release-pull-secret", Namespace: managedNamespace},
		Data: map[string][]byte{".dockerconfigjson": secretData},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	_, err = kubeadminClient.CommonController.CreateSecret(managedNamespace, secret)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	managedServiceAccount, err := kubeadminClient.CommonController.CreateServiceAccount("release-service-account", managedNamespace, []corev1.ObjectReference{{Name: secret.Name}}, nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = kubeadminClient.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(userNamespace, managedServiceAccount)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	_, err = kubeadminClient.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	publicKey, err := kubeadminClient.TektonController.GetTektonChainsPublicKey()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	gomega.Expect(kubeadminClient.TektonController.CreateOrUpdateSigningSecret(publicKey, "cosign-public-key", managedNamespace)).To(gomega.Succeed())

	_, err = kubeadminClient.ReleaseController.CreateReleasePlan("source-releaseplan", userNamespace, appName, managedNamespace, "", nil, nil, nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	defaultEcPolicy, err := kubeadminClient.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	ecPolicyName := componentName + "-policy"
	defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
		Description: "Red Hat's enterprise requirements",
		PublicKey:   string(publicKey),
		Sources:     defaultEcPolicy.Spec.Sources,
		Configuration: &ecp.EnterpriseContractPolicyConfiguration{
			Collections: []string{"minimal"},
			Exclude:     []string{"cve"},
		},
	}
	_, err = kubeadminClient.TektonController.CreateEnterpriseContractPolicy(ecPolicyName, managedNamespace, defaultEcPolicySpec)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = kubeadminClient.ReleaseController.CreateReleasePlanAdmission("demo", managedNamespace, "", userNamespace, ecPolicyName, "release-service-account", []string{appName}, false, &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasecommon.RelSvcCatalogURL},
			{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
			{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
		},
	}, nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = kubeadminClient.TektonController.CreatePVCInAccessMode("release-pvc", managedNamespace, corev1.ReadWriteOnce)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = kubeadminClient.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
		"apiGroupsList": {""},
		"roleResources": {"secrets"},
		"roleVerbs":     {"get", "list", "watch"},
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = kubeadminClient.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", "release-service-account", managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func CreateE2EQuaySecret(k *kubeapi.CustomClient) (*corev1.Secret, error) {
	var secret *corev1.Secret

	quayToken := os.Getenv("QUAY_TOKEN")
	if quayToken == "" {
		return nil, fmt.Errorf("failed to obtain quay token from 'QUAY_TOKEN' env; make sure the env exists")
	}

	decodedToken, err := base64.StdEncoding.DecodeString(quayToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode quay token. Make sure that QUAY_TOKEN env contain a base64 token")
	}

	namespace := constants.QuayRepositorySecretNamespace
	_, err = k.KubeInterface().CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := k.KubeInterface().CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("error when creating namespace %s : %v", namespace, err)
			}
		} else {
			return nil, fmt.Errorf("error when getting namespace %s : %v", namespace, err)
		}
	}

	secretName := constants.QuayRepositorySecretName
	secret, err = k.KubeInterface().CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			secret, err = k.KubeInterface().CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: decodedToken,
				},
			}, metav1.CreateOptions{})

			if err != nil {
				return nil, fmt.Errorf("error when creating secret %s : %v", secretName, err)
			}
		} else {
			secret.Data = map[string][]byte{
				corev1.DockerConfigJsonKey: decodedToken,
			}
			secret, err = k.KubeInterface().CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("error when updating secret '%s' namespace: %v", secretName, err)
			}
		}
	}

	return secret, nil
}
