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
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/google/go-github/v44/github"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	kubeapi "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
	e2eConfig "github.com/konflux-ci/e2e-tests/tests/konflux-demo/config"
)

var _ = framework.KonfluxDemoSuiteDescribe(Label(devEnvTestLabel), func() {
	defer GinkgoRecover()

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
	var kubeadminClient *framework.ControllerHub

	var buildPipelineAnnotation map[string]string

	var componentNewBaseBranch, gitRevision, componentRepositoryName, componentName string

	var appSpecs []e2eConfig.ApplicationSpec
	if strings.Contains(GinkgoLabelFilter(), upstreamKonfluxTestLabel) {
		appSpecs = e2eConfig.UpstreamAppSpecs
	} else {
		appSpecs = e2eConfig.ApplicationSpecs
	}

	for _, appSpec := range appSpecs {
		appSpec := appSpec
		if appSpec.Skip {
			continue
		}

		Describe(appSpec.Name, Ordered, func() {
			BeforeAll(func() {
				if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
					Skip("Skipping this test due to configuration issue with Spray proxy")
				}
				if !strings.Contains(GinkgoLabelFilter(), upstreamKonfluxTestLabel) {
					// Namespace config
					fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devEnvTestLabel))
					Expect(err).NotTo(HaveOccurred())
					userNamespace = fw.UserNamespace
					kubeadminClient = fw.AsKubeAdmin
					Expect(err).ShouldNot(HaveOccurred())
				} else {
					var asAdminClient *kubeapi.CustomClient
					userNamespace = os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV)
					asAdminClient, err = kubeapi.NewAdminKubernetesClient()
					Expect(err).ShouldNot(HaveOccurred())
					kubeadminClient, err = framework.InitControllerHub(asAdminClient)
					Expect(err).ShouldNot(HaveOccurred())
					_, err = kubeadminClient.CommonController.CreateTestNamespace(userNamespace)
				}
				managedNamespace = userNamespace + "-managed"

				// Component config
				componentName = fmt.Sprintf("%s-%s", appSpec.ComponentSpec.Name, util.GenerateRandomString(4))
				pacBranchName = fmt.Sprintf("appstudio-%s", componentName)
				componentRepositoryName = utils.ExtractGitRepositoryNameFromURL(appSpec.ComponentSpec.GitSourceUrl)

				// Secrets config
				// https://issues.redhat.com/browse/KFLUXBUGS-1462 - creating SCM secret alongside with PaC
				// leads to PLRs being duplicated
				// secretDefinition := build.GetSecretDefForGitHub(namespace)
				// secret, err = kubeadminClient.CommonController.CreateSecret(namespace, secretDefinition)
				sharedSecret, err := kubeadminClient.CommonController.GetSecret(constants.QuayRepositorySecretNamespace, constants.QuayRepositorySecretName)
				if err != nil && k8sErrors.IsNotFound(err) {
					sharedSecret, err = CreateE2EQuaySecret(kubeadminClient.CommonController.CustomClient)
					Expect(err).ShouldNot(HaveOccurred())
				}
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error when getting shared secret - make sure the secret %s in %s userNamespace is created", constants.QuayRepositorySecretName, constants.QuayRepositorySecretNamespace))

				createReleaseConfig(kubeadminClient, managedNamespace, userNamespace, appSpec.ComponentSpec.Name, appSpec.ApplicationName, sharedSecret.Data[".dockerconfigjson"])

				// get the build pipeline bundle annotation
				buildPipelineAnnotation = build.GetDockerBuildPipelineBundle()

			})

			// Remove all resources created by the tests
			AfterAll(func() {
				if !(strings.EqualFold(os.Getenv("E2E_SKIP_CLEANUP"), "true")) && !CurrentSpecReport().Failed() && !strings.Contains(GinkgoLabelFilter(), upstreamKonfluxTestLabel) {
					Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
					Expect(kubeadminClient.CommonController.DeleteNamespace(managedNamespace)).To(Succeed())

					// Delete new branch created by PaC and a testing branch used as a component's base branch
					err = kubeadminClient.CommonController.Github.DeleteRef(componentRepositoryName, pacBranchName)
					if err != nil {
						Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
					}
					err = kubeadminClient.CommonController.Github.DeleteRef(componentRepositoryName, componentNewBaseBranch)
					if err != nil {
						Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
					}
					Expect(build.CleanupWebhooks(fw, componentRepositoryName)).ShouldNot(HaveOccurred())
				}
			})

			// Create an application in a specific namespace
			It("creates an application", Label(devEnvTestLabel, upstreamKonfluxTestLabel), func() {
				createdApplication, err := kubeadminClient.HasController.CreateApplication(appSpec.ApplicationName, userNamespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdApplication.Spec.DisplayName).To(Equal(appSpec.ApplicationName))
				Expect(createdApplication.Namespace).To(Equal(userNamespace))
			})

			// Create an IntegrationTestScenario for the App
			It("creates an IntegrationTestScenario for the app", Label(devEnvTestLabel, upstreamKonfluxTestLabel), func() {
				its := appSpec.ComponentSpec.IntegrationTestScenario
				integrationTestScenario, err = kubeadminClient.IntegrationController.CreateIntegrationTestScenario("", appSpec.ApplicationName, userNamespace, its.GitURL, its.GitRevision, its.TestPath, []string{})
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("creates new branch for the build", Label(devEnvTestLabel, upstreamKonfluxTestLabel), func() {
				// We need to create a new branch that we will target
				// and that will contain the PaC configuration, so we
				// can avoid polluting the default (main) branch
				componentNewBaseBranch = fmt.Sprintf("base-%s", util.GenerateRandomString(6))
				gitRevision = componentNewBaseBranch
				Expect(kubeadminClient.CommonController.Github.CreateRef(componentRepositoryName, appSpec.ComponentSpec.GitSourceDefaultBranchName, appSpec.ComponentSpec.GitSourceRevision, componentNewBaseBranch)).To(Succeed())
			})

			// Component are imported from gitUrl
			It(fmt.Sprintf("creates component %s (private: %t) from git source %s", appSpec.ComponentSpec.Name, appSpec.ComponentSpec.Private, appSpec.ComponentSpec.GitSourceUrl), Label(devEnvTestLabel, upstreamKonfluxTestLabel), func() {
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

				component, err = kubeadminClient.HasController.CreateComponent(componentObj, userNamespace, "", "", appSpec.ApplicationName, false, utils.MergeMaps(constants.ComponentPaCRequestAnnotation, buildPipelineAnnotation))
				Expect(err).ShouldNot(HaveOccurred())
			})

			When("Component is created", Label(upstreamKonfluxTestLabel), func() {
				It("triggers creation of a PR in the sample repo", func() {
					var prSHA string
					Eventually(func() error {
						prs, err := kubeadminClient.CommonController.Github.ListPullRequests(componentRepositoryName)
						Expect(err).ShouldNot(HaveOccurred())
						for _, pr := range prs {
							if pr.Head.GetRef() == pacBranchName {
								prNumber = pr.GetNumber()
								prSHA = pr.GetHead().GetSHA()
								return nil
							}
						}
						return fmt.Errorf("could not get the expected PaC branch name %s", pacBranchName)
					}, pullRequestCreationTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for init PaC PR (branch %q) to be created against the %q repo", pacBranchName, componentRepositoryName))

					// We don't need the PipelineRun from a PaC 'pull-request' event to finish, so we can delete it
					Eventually(func() error {
						pipelineRun, err = kubeadminClient.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, prSHA)
						if err == nil {
							Expect(kubeadminClient.TektonController.DeletePipelineRun(pipelineRun.Name, pipelineRun.Namespace)).To(Succeed())
							return nil
						}
						return err
					}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for `pull-request` event type PaC PipelineRun to be present in the user namespace %q for component %q with a label pointing to %q", userNamespace, component.GetName(), appSpec.ApplicationName))
				})

				It("verifies component build status", func() {
					var buildStatus *buildcontrollers.BuildStatus
					Eventually(func() (bool, error) {
						component, err := kubeadminClient.HasController.GetComponent(component.GetName(), userNamespace)
						if err != nil {
							return false, err
						}

						statusBytes := []byte(component.Annotations[buildcontrollers.BuildStatusAnnotationName])

						err = json.Unmarshal(statusBytes, &buildStatus)
						if err != nil {
							return false, err
						}

						if buildStatus.PaC != nil {
							GinkgoWriter.Printf("state: %s\n", buildStatus.PaC.State)
							GinkgoWriter.Printf("mergeUrl: %s\n", buildStatus.PaC.MergeUrl)
							GinkgoWriter.Printf("errId: %d\n", buildStatus.PaC.ErrId)
							GinkgoWriter.Printf("errMessage: %s\n", buildStatus.PaC.ErrMessage)
							GinkgoWriter.Printf("configurationTime: %s\n", buildStatus.PaC.ConfigurationTime)
						} else {
							GinkgoWriter.Println("build status does not have PaC field")
						}

						return buildStatus.PaC != nil && buildStatus.PaC.State == "enabled" && buildStatus.PaC.MergeUrl != "" && buildStatus.PaC.ErrId == 0 && buildStatus.PaC.ConfigurationTime != "", nil
					}, timeout, interval).Should(BeTrue(), "component build status has unexpected content")
				})

				It("should eventually lead to triggering a 'push' event type PipelineRun after merging the PaC init branch ", func() {
					Eventually(func() error {
						mergeResult, err = kubeadminClient.CommonController.Github.MergePullRequest(componentRepositoryName, prNumber)
						return err
					}, mergePRTimeout).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v\n", err))

					headSHA = mergeResult.GetSHA()

					Eventually(func() error {
						pipelineRun, err = kubeadminClient.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, headSHA)
						if err != nil {
							GinkgoWriter.Printf("PipelineRun has not been created yet for component %s/%s\n", userNamespace, component.GetName())
							return err
						}
						if !pipelineRun.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for a PipelineRun in namespace %q with label component label %q and application label %q and sha label %q to start", userNamespace, component.GetName(), appSpec.ApplicationName, headSHA))
				})
			})

			When("Build PipelineRun is created", Label(upstreamKonfluxTestLabel), func() {
				It("does not contain an annotation with a Snapshot Name", func() {
					Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(""))
				})
				It("should eventually complete successfully", func() {
					Expect(kubeadminClient.HasController.WaitForComponentPipelineToBeFinished(component, headSHA,
						kubeadminClient.TektonController, &has.RetryOptions{Retries: 5, Always: true}, pipelineRun)).To(Succeed())

					// in case the first pipelineRun attempt has failed and was retried, we need to update the git branch head ref
					headSHA = pipelineRun.Labels["pipelinesascode.tekton.dev/sha"]
				})
			})

			When("Build PipelineRun completes successfully", func() {

				It("should validate Tekton TaskRun test results successfully", func() {
					pipelineRun, err = kubeadminClient.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, headSHA)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(build.ValidateBuildPipelineTestResults(pipelineRun, kubeadminClient.CommonController.KubeRest(), false)).To(Succeed())
				})

				It("should validate that the build pipelineRun is signed", Label(upstreamKonfluxTestLabel), func() {
					Eventually(func() error {
						pipelineRun, err = kubeadminClient.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, headSHA)
						if err != nil {
							return err
						}
						if pipelineRun.Annotations["chains.tekton.dev/signed"] != "true" {
							return fmt.Errorf("pipelinerun %s/%s does not have the expected value of annotation 'chains.tekton.dev/signed'", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, time.Minute*5, time.Second*5).Should(Succeed(), "failed while validating build pipelineRun is signed")

				})

				It("should find the related Snapshot CR", Label(upstreamKonfluxTestLabel), func() {
					Eventually(func() error {
						snapshot, err = kubeadminClient.IntegrationController.GetSnapshot("", pipelineRun.Name, "", userNamespace)
						return err
					}, snapshotTimeout, snapshotPollingInterval).Should(Succeed(), "timed out when trying to check if the Snapshot exists for PipelineRun %s/%s", userNamespace, pipelineRun.GetName())
				})

				It("should validate that the build pipelineRun is annotated with the name of the Snapshot", Label(upstreamKonfluxTestLabel), func() {
					pipelineRun, err = kubeadminClient.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, userNamespace, headSHA)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(snapshot.GetName()))
				})

				It("should find the related Integration Test PipelineRun", Label(upstreamKonfluxTestLabel), func() {
					Eventually(func() error {
						testPipelinerun, err = kubeadminClient.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot.Name, userNamespace)
						if err != nil {
							GinkgoWriter.Printf("failed to get Integration test PipelineRun for a snapshot '%s' in '%s' namespace: %+v\n", snapshot.Name, userNamespace, err)
							return err
						}
						if !testPipelinerun.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", testPipelinerun.GetNamespace(), testPipelinerun.GetName())
						}
						return nil
					}, pipelineRunStartedTimeout, defaultPollingInterval).Should(Succeed())
					Expect(testPipelinerun.Labels["appstudio.openshift.io/snapshot"]).To(ContainSubstring(snapshot.Name))
					Expect(testPipelinerun.Labels["test.appstudio.openshift.io/scenario"]).To(ContainSubstring(integrationTestScenario.Name))
				})
			})

			When("Integration Test PipelineRun is created", Label(upstreamKonfluxTestLabel), func() {
				It("should eventually complete successfully", func() {
					Expect(kubeadminClient.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenario, snapshot, userNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a integration pipeline for snapshot %s/%s to finish", userNamespace, snapshot.GetName()))
				})
			})

			When("Integration Test PipelineRun completes successfully", Label(upstreamKonfluxTestLabel), func() {
				It("should lead to Snapshot CR being marked as passed", func() {
					Eventually(func() bool {
						snapshot, err = kubeadminClient.IntegrationController.GetSnapshot("", pipelineRun.Name, "", userNamespace)
						Expect(err).ShouldNot(HaveOccurred())
						return kubeadminClient.CommonController.HaveTestsSucceeded(snapshot)
					}, time.Minute*5, defaultPollingInterval).Should(BeTrue(), fmt.Sprintf("tests have not succeeded for snapshot %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
				})

				It("should trigger creation of Release CR", func() {
					Eventually(func() error {
						release, err = kubeadminClient.ReleaseController.GetRelease("", snapshot.Name, userNamespace)
						return err
					}, releaseTimeout, releasePollingInterval).Should(Succeed(), fmt.Sprintf("timed out when trying to check if the release exists for snapshot %s/%s", userNamespace, snapshot.GetName()))
				})
			})

			When("Release CR is created", Label(upstreamKonfluxTestLabel), func() {
				It("triggers creation of Release PipelineRun", func() {
					Eventually(func() error {
						pipelineRun, err = kubeadminClient.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
						if err != nil {
							GinkgoWriter.Printf("pipelineRun for component '%s' in namespace '%s' not created yet: %+v\n", component.GetName(), managedNamespace, err)
							return err
						}
						if !pipelineRun.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, pipelineRunStartedTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("failed to get pipelinerun named %q in namespace %q with label to release %q in namespace %q to start", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
				})
			})

			When("Release PipelineRun is triggered", Label(upstreamKonfluxTestLabel), func() {
				It("should eventually succeed", func() {
					Eventually(func() error {
						pr, err := kubeadminClient.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(tekton.HasPipelineRunFailed(pr)).NotTo(BeTrue(), fmt.Sprintf("did not expect PipelineRun %s/%s to fail", pr.GetNamespace(), pr.GetName()))
						if !pr.IsDone() {
							return fmt.Errorf("release pipelinerun %s/%s has not finished yet", pr.GetNamespace(), pr.GetName())
						}
						Expect(tekton.HasPipelineRunSucceeded(pr)).To(BeTrue(), fmt.Sprintf("PipelineRun %s/%s did not succeed", pr.GetNamespace(), pr.GetName()))
						return nil
					}, releasePipelineTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("failed to see pipelinerun %q in namespace %q with a label pointing to release %q in namespace %q to complete successfully", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
				})
			})

			When("Release PipelineRun is completed", Label(upstreamKonfluxTestLabel), func() {
				It("should lead to Release CR being marked as succeeded", func() {
					Eventually(func() error {
						release, err = kubeadminClient.ReleaseController.GetRelease(release.Name, "", userNamespace)
						Expect(err).ShouldNot(HaveOccurred())
						if !release.IsReleased() {
							return fmt.Errorf("release CR %s/%s is not marked as finished yet", release.GetNamespace(), release.GetName())
						}
						return nil
					}, customResourceUpdateTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("failed to see release %q in namespace %q get marked as released", release.Name, userNamespace))
				})
			})
		})
	}
})

func createReleaseConfig(kubeadminClient *framework.ControllerHub, managedNamespace, userNamespace, componentName, appName string, secretData []byte) {
	var err error
	_, err = kubeadminClient.CommonController.CreateTestNamespace(managedNamespace)
	Expect(err).ShouldNot(HaveOccurred())

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "release-pull-secret", Namespace: managedNamespace},
		Data: map[string][]byte{".dockerconfigjson": secretData},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	_, err = kubeadminClient.CommonController.CreateSecret(managedNamespace, secret)
	Expect(err).ShouldNot(HaveOccurred())

	managedServiceAccount, err := kubeadminClient.CommonController.CreateServiceAccount("release-service-account", managedNamespace, []corev1.ObjectReference{{Name: secret.Name}}, nil)
	Expect(err).NotTo(HaveOccurred())

	_, err = kubeadminClient.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(userNamespace, managedServiceAccount)
	Expect(err).NotTo(HaveOccurred())
	_, err = kubeadminClient.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
	Expect(err).NotTo(HaveOccurred())

	publicKey, err := kubeadminClient.TektonController.GetTektonChainsPublicKey()
	Expect(err).ToNot(HaveOccurred())

	Expect(kubeadminClient.TektonController.CreateOrUpdateSigningSecret(publicKey, "cosign-public-key", managedNamespace)).To(Succeed())

	_, err = kubeadminClient.ReleaseController.CreateReleasePlan("source-releaseplan", userNamespace, appName, managedNamespace, "", nil, nil, nil)
	Expect(err).NotTo(HaveOccurred())

	defaultEcPolicy, err := kubeadminClient.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
	Expect(err).NotTo(HaveOccurred())
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
	Expect(err).NotTo(HaveOccurred())

	_, err = kubeadminClient.ReleaseController.CreateReleasePlanAdmission("demo", managedNamespace, "", userNamespace, ecPolicyName, "release-service-account", []string{appName}, true, &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasecommon.RelSvcCatalogURL},
			{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
			{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
		},
	}, nil)
	Expect(err).NotTo(HaveOccurred())

	_, err = kubeadminClient.TektonController.CreatePVCInAccessMode("release-pvc", managedNamespace, corev1.ReadWriteOnce)
	Expect(err).NotTo(HaveOccurred())

	_, err = kubeadminClient.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
		"apiGroupsList": {""},
		"roleResources": {"secrets"},
		"roleVerbs":     {"get", "list", "watch"},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = kubeadminClient.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", "release-service-account", managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
	Expect(err).NotTo(HaveOccurred())
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
