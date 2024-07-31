package konflux_demo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	buildcontrollers "github.com/konflux-ci/build-service/controllers"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	"github.com/redhat-appstudio/jvm-build-service/openshift-with-appstudio-test/e2e"
	jvmclientSet "github.com/redhat-appstudio/jvm-build-service/pkg/client/clientset/versioned"
	pipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"

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
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	e2eConfig "github.com/konflux-ci/e2e-tests/tests/konflux-demo/config"
)

var _ = framework.KonfluxDemoSuiteDescribe(Label(devEnvTestLabel), func() {
	defer GinkgoRecover()

	var timeout, interval time.Duration
	var namespace string
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

	var componentNewBaseBranch, gitRevision, componentRepositoryName, componentName string

	for _, appSpec := range e2eConfig.ApplicationSpecs {
		appSpec := appSpec
		if appSpec.Skip {
			continue
		}

		Describe(appSpec.Name, Ordered, func() {
			BeforeAll(func() {
				if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
					Skip("Skipping this test due to configuration issue with Spray proxy")
				}
				// Namespace config
				fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devEnvTestLabel))
				Expect(err).NotTo(HaveOccurred())
				namespace = fw.UserNamespace
				Expect(err).NotTo(HaveOccurred())
				managedNamespace = fw.UserNamespace + "-managed"

				// Component config
				componentName = fmt.Sprintf("%s-%s", appSpec.ComponentSpec.Name, util.GenerateRandomString(4))
				pacBranchName = fmt.Sprintf("appstudio-%s", componentName)
				componentRepositoryName = utils.ExtractGitRepositoryNameFromURL(appSpec.ComponentSpec.GitSourceUrl)

				// Secrets config
				// https://issues.redhat.com/browse/KFLUXBUGS-1462 - creating SCM secret alongside with PaC
				// leads to PLRs being duplicated
				// secretDefinition := build.GetSecretDefForGitHub(namespace)
				// secret, err = fw.AsKubeAdmin.CommonController.CreateSecret(namespace, secretDefinition)
				sharedSecret, err := fw.AsKubeAdmin.CommonController.GetSecret(constants.QuayRepositorySecretNamespace, constants.QuayRepositorySecretName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error when getting shared secret - make sure the secret %s in %s userNamespace is created", constants.QuayRepositorySecretName, constants.QuayRepositorySecretNamespace))

				createReleaseConfig(*fw, managedNamespace, appSpec.ComponentSpec.Name, appSpec.ApplicationName, sharedSecret.Data[".dockerconfigjson"])

				// JBS related config
				_, err = fw.AsKubeAdmin.JvmbuildserviceController.CreateJBSConfig(constants.JBSConfigName, fw.UserNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(fw.AsKubeAdmin.JvmbuildserviceController.WaitForCache(fw.AsKubeAdmin.CommonController, fw.UserNamespace)).Should(Succeed())
			})

			// Remove all resources created by the tests
			AfterAll(func() {
				if !(strings.EqualFold(os.Getenv("E2E_SKIP_CLEANUP"), "true")) && !CurrentSpecReport().Failed() { // RHTAPBUGS-978: temporary timeout to 15min
					Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
					Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(Succeed())

					// Delete new branch created by PaC and a testing branch used as a component's base branch
					err = fw.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepositoryName, pacBranchName)
					if err != nil {
						Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
					}
					err = fw.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepositoryName, componentNewBaseBranch)
					if err != nil {
						Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
					}
				}
			})

			// Create an application in a specific namespace
			It("creates an application", Label(devEnvTestLabel), func() {
				createdApplication, err := fw.AsKubeDeveloper.HasController.CreateApplication(appSpec.ApplicationName, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdApplication.Spec.DisplayName).To(Equal(appSpec.ApplicationName))
				Expect(createdApplication.Namespace).To(Equal(namespace))
			})

			// Create an Integration test scenario for the app
			It("creates an IntegrationTestScenario for the app", Label(devEnvTestLabel), func() {
				its := appSpec.ComponentSpec.IntegrationTestScenario
				integrationTestScenario, err = fw.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario("", appSpec.ApplicationName, fw.UserNamespace, its.GitURL, its.GitRevision, its.TestPath)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("creates new branch for the build", Label(devEnvTestLabel), func() {
				// We need to create a new branch that we will target
				// and that will contain the PaC configuration, so we
				// can avoid polluting the default (main) branch
				componentNewBaseBranch = fmt.Sprintf("base-%s", util.GenerateRandomString(6))
				gitRevision = componentNewBaseBranch
				Expect(fw.AsKubeAdmin.CommonController.Github.CreateRef(componentRepositoryName, appSpec.ComponentSpec.GitSourceDefaultBranchName, appSpec.ComponentSpec.GitSourceRevision, componentNewBaseBranch)).To(Succeed())
			})

			// Component are imported from gitUrl
			It(fmt.Sprintf("creates component %s (private: %t) from git source %s", appSpec.ComponentSpec.Name, appSpec.ComponentSpec.Private, appSpec.ComponentSpec.GitSourceUrl), Label(devEnvTestLabel), func() {
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

				component, err = fw.AsKubeAdmin.HasController.CreateComponent(componentObj, namespace, "", "", appSpec.ApplicationName, false, utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.DefaultDockerBuildPipelineBundle))
				Expect(err).ShouldNot(HaveOccurred())
			})

			When("Component is created", func() {

				It("triggers creation of a PR in the sample repo", func() {

					var prSHA string
					Eventually(func() error {
						prs, err := fw.AsKubeAdmin.CommonController.Github.ListPullRequests(componentRepositoryName)
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

					// We actually don't need the "on-pull-request" PipelineRun to complete, so we can delete it
					Eventually(func() error {
						pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, fw.UserNamespace, prSHA)
						if err == nil {
							Expect(fw.AsKubeAdmin.TektonController.DeletePipelineRun(pipelineRun.Name, pipelineRun.Namespace)).To(Succeed())
							return nil
						}
						return err
					}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for init PaC PipelineRun to be present in the user namespace %q for component %q with a label pointing to %q", fw.UserNamespace, component.GetName(), appSpec.ApplicationName))

				})

				It("component build status is set correctly", func() {
					var buildStatus *buildcontrollers.BuildStatus
					Eventually(func() (bool, error) {
						component, err := fw.AsKubeAdmin.HasController.GetComponent(component.GetName(), fw.UserNamespace)
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
				It("should eventually lead to triggering another PipelineRun after merging the PaC init branch ", func() {
					Eventually(func() error {
						mergeResult, err = fw.AsKubeAdmin.CommonController.Github.MergePullRequest(componentRepositoryName, prNumber)
						return err
					}, mergePRTimeout).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v\n", err))

					headSHA = mergeResult.GetSHA()

					Eventually(func() error {
						pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, fw.UserNamespace, headSHA)
						if err != nil {
							GinkgoWriter.Printf("PipelineRun has not been created yet for component %s/%s\n", fw.UserNamespace, component.GetName())
							return err
						}
						if !pipelineRun.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for a PipelineRun in namespace %q with label component label %q and application label %q and sha label %q to start", fw.UserNamespace, component.GetName(), appSpec.ApplicationName, headSHA))
				})
			})

			When("Build PipelineRun is created", func() {
				It("does not contain an annotation with a Snapshot Name", func() {
					Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(""))
				})
				It("should eventually complete successfully", func() {
					Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, headSHA,
						fw.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 5, Always: true}, pipelineRun)).To(Succeed())

					// in case the first pipelineRun attempt has failed and was retried, we need to update the git branch head ref
					headSHA = pipelineRun.Labels["pipelinesascode.tekton.dev/sha"]
				})
			})

			When("Build PipelineRun completes successfully", func() {
				It("should be possible to download the SBOM file", func() {
					var outputImage string
					for _, p := range pipelineRun.Spec.Params {
						if p.Name == "output-image" {
							outputImage = p.Value.StringVal
						}
					}
					Expect(outputImage).ToNot(BeEmpty(), "output image of a component could not be found")

					_, _, err = build.GetParsedSbomFilesContentFromImage(outputImage)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should validate Tekton TaskRun test results successfully", func() {
					pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, fw.UserNamespace, headSHA)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(build.ValidateBuildPipelineTestResults(pipelineRun, fw.AsKubeAdmin.CommonController.KubeRest())).To(Succeed())
				})

				It("should validate that the build pipelineRun is signed", func() {
					pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, fw.UserNamespace, headSHA)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(pipelineRun.Annotations["chains.tekton.dev/signed"]).To(Equal("true"), fmt.Sprintf("pipelinerun %s/%s does not have the expected value of annotation 'chains.tekton.dev/signed'", pipelineRun.GetNamespace(), pipelineRun.GetName()))
				})

				It("should find the related Snapshot CR", func() {
					Eventually(func() error {
						snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", fw.UserNamespace)
						return err
					}, snapshotTimeout, snapshotPollingInterval).Should(Succeed(), "timed out when trying to check if the Snapshot exists for PipelineRun %s/%s", fw.UserNamespace, pipelineRun.GetName())
				})

				It("should validate that the build pipelineRun is annotated with the name of the Snapshot", func() {
					pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appSpec.ApplicationName, fw.UserNamespace, headSHA)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(snapshot.GetName()))
				})

				It("should find the related Integration Test PipelineRun", func() {
					Eventually(func() error {
						testPipelinerun, err = fw.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot.Name, fw.UserNamespace)
						if err != nil {
							GinkgoWriter.Printf("failed to get Integration test PipelineRun for a snapshot '%s' in '%s' namespace: %+v\n", snapshot.Name, fw.UserNamespace, err)
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

			When("Integration Test PipelineRun is created", func() {
				It("should eventually complete successfully", func() {
					Expect(fw.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenario, snapshot, fw.UserNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a integration pipeline for snapshot %s/%s to finish", fw.UserNamespace, snapshot.GetName()))
				})
			})

			When("Integration Test PipelineRun completes successfully", func() {

				It("should lead to Snapshot CR being marked as passed", func() {
					snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", fw.UserNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					Eventually(func() bool {
						return fw.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot)
					}, time.Minute*5, defaultPollingInterval).Should(BeTrue(), fmt.Sprintf("tests have not succeeded for snapshot %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
				})

				It("should trigger creation of Release CR", func() {
					Eventually(func() error {
						release, err = fw.AsKubeAdmin.ReleaseController.GetRelease("", snapshot.Name, fw.UserNamespace)
						return err
					}, releaseTimeout, releasePollingInterval).Should(Succeed(), fmt.Sprintf("timed out when trying to check if the release exists for snapshot %s/%s", fw.UserNamespace, snapshot.GetName()))
				})
			})

			When("Release CR is created", func() {
				It("triggers creation of Release PipelineRun", func() {
					Eventually(func() error {
						pipelineRun, err = fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
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

			When("Release PipelineRun is triggered", func() {
				It("should eventually succeed", func() {
					Eventually(func() error {
						pr, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
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
			When("Release PipelineRun is completed", func() {
				It("should lead to Release CR being marked as succeeded", func() {
					Eventually(func() error {
						release, err = fw.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", fw.UserNamespace)
						Expect(err).ShouldNot(HaveOccurred())
						if !release.IsReleased() {
							return fmt.Errorf("release CR %s/%s is not marked as finished yet", release.GetNamespace(), release.GetName())
						}
						return nil
					}, customResourceUpdateTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("failed to see release %q in namespace %q get marked as released", release.Name, fw.UserNamespace))
				})
			})

			When("JVM Build Service is used for rebuilding dependencies", func() {
				It("should eventually rebuild of all artifacts and dependencies successfully", func() {
					jvmClient := jvmclientSet.New(fw.AsKubeAdmin.JvmbuildserviceController.JvmbuildserviceClient().JvmbuildserviceV1alpha1().RESTClient())
					tektonClient := pipelineclientset.New(fw.AsKubeAdmin.TektonController.PipelineClient().TektonV1beta1().RESTClient())
					kubeClient := kubernetes.New(fw.AsKubeAdmin.CommonController.KubeInterface().CoreV1().RESTClient())
					//status report ends up in artifacts/redhat-appstudio-e2e/redhat-appstudio-e2e/artifacts/rp_preproc/attachments/xunit
					defer e2e.GenerateStatusReport(fw.UserNamespace, jvmClient, kubeClient, tektonClient)
					Eventually(func() error {
						abList, err := fw.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(fw.UserNamespace)
						Expect(err).ShouldNot(HaveOccurred())
						for _, ab := range abList.Items {
							if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
								return fmt.Errorf("artifactbuild %s not complete", ab.Spec.GAV)
							}
						}
						dbList, err := fw.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(fw.UserNamespace)
						Expect(err).ShouldNot(HaveOccurred())
						for _, db := range dbList.Items {
							if db.Status.State != v1alpha1.DependencyBuildStateComplete {
								return fmt.Errorf("dependencybuild %s not complete", db.Spec.ScmInfo.SCMURL)
							}
						}
						return nil
					}, jvmRebuildTimeout, jvmRebuildPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for all artifactbuilds and dependencybuilds to complete in namespace %q", fw.UserNamespace))
				})
			})
		})
	}

})

func createReleaseConfig(fw framework.Framework, managedNamespace, componentName, appName string, secretData []byte) {
	var err error
	_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
	Expect(err).ShouldNot(HaveOccurred())

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "release-pull-secret", Namespace: managedNamespace},
		Data: map[string][]byte{".dockerconfigjson": secretData},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
	Expect(err).ShouldNot(HaveOccurred())

	managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount("release-service-account", managedNamespace, []corev1.ObjectReference{{Name: secret.Name}}, nil)
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(fw.UserNamespace, managedServiceAccount)
	Expect(err).NotTo(HaveOccurred())
	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
	Expect(err).NotTo(HaveOccurred())

	publicKey, err := fw.AsKubeAdmin.TektonController.GetTektonChainsPublicKey()
	Expect(err).ToNot(HaveOccurred())

	Expect(fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(publicKey, "cosign-public-key", managedNamespace)).To(Succeed())

	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan("source-releaseplan", fw.UserNamespace, appName, managedNamespace, "", nil, nil)
	Expect(err).NotTo(HaveOccurred())

	defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
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
	_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(ecPolicyName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission("demo", managedNamespace, "", fw.UserNamespace, ecPolicyName, "release-service-account", []string{appName}, true, &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasecommon.RelSvcCatalogURL},
			{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
			{Name: "pathInRepo", Value: "pipelines/e2e/e2e.yaml"},
		},
	}, nil)
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode("release-pvc", managedNamespace, corev1.ReadWriteOnce)
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
		"apiGroupsList": {""},
		"roleResources": {"secrets"},
		"roleVerbs":     {"get", "list", "watch"},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", "release-service-account", managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
	Expect(err).NotTo(HaveOccurred())
}
