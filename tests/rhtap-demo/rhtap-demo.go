package rhtap_demo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/google/go-github/v44/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	r "github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/yaml"
)

const (
	// This app might be replaced with service-registry in a future
	sampleRepoName             = "hacbs-test-project"
	componentDefaultBranchName = "main"

	// Kubernetes resource names
	testNamespacePrefix = "rhtap-demo-dev"
	managedNamespace    = "rhtap-demo-managed"

	appName                = "mvp-test-app"
	testScenarioGitURL     = "https://github.com/redhat-appstudio/integration-examples.git"
	testScenarioRevision   = "main"
	testScenarioPathInRepo = "pipelines/integration_resolver_pipeline_pass.yaml"

	// Timeouts
	appDeployTimeout            = time.Minute * 20
	appRouteAvailableTimeout    = time.Minute * 5
	customResourceUpdateTimeout = time.Minute * 2
	jvmRebuildTimeout           = time.Minute * 20
	mergePRTimeout              = time.Minute * 1
	pipelineRunStartedTimeout   = time.Minute * 5
	pullRequestCreationTimeout  = time.Minute * 5
	releasePipelineTimeout      = time.Minute * 15
	snapshotTimeout             = time.Minute * 4
	releaseTimeout              = time.Minute * 4
	testPipelineTimeout         = time.Minute * 15

	// Intervals
	defaultPollingInterval    = time.Second * 2
	jvmRebuildPollingInterval = time.Second * 10
	snapshotPollingInterval   = time.Second * 1
	releasePollingInterval    = time.Second * 1
)

var sampleRepoURL = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), sampleRepoName)

var _ = framework.RhtapDemoSuiteDescribe("RHTAP Demo", Label("rhtap-demo"), func() {

	defer GinkgoRecover()
	var err error
	var f *framework.Framework
	var sharedSecret *corev1.Secret
	var pacControllerRoute *routev1.Route
	var componentName string

	var componentNewBaseBranch, userNamespace string

	var kc tekton.KubeController

	// set vs. simply declare these pointers so we can use them in debug, where an empty name is indicative of Get's failing
	component := &appstudioApi.Component{}
	pipelineRun := &tektonapi.PipelineRun{}
	release := &releaseApi.Release{}
	snapshot := &appstudioApi.Snapshot{}
	testPipelinerun := &tektonapi.PipelineRun{}
	integrationTestScenario := &integrationv1beta1.IntegrationTestScenario{}

	BeforeAll(func() {
		f, err = framework.NewFramework(utils.GetGeneratedNamespace(testNamespacePrefix))
		Expect(err).NotTo(HaveOccurred())
		userNamespace = f.UserNamespace
		Expect(userNamespace).NotTo(BeEmpty())

		componentName = fmt.Sprintf("rhtap-demo-component-%s", util.GenerateRandomString(4))
		componentNewBaseBranch = fmt.Sprintf("base-%s", util.GenerateRandomString(4))

		sharedSecret, err = f.AsKubeAdmin.CommonController.GetSecret(constants.QuayRepositorySecretNamespace, constants.QuayRepositorySecretName)
		Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error when getting shared secret - make sure the secret %s in %s userNamespace is created", constants.QuayRepositorySecretName, constants.QuayRepositorySecretNamespace))

		// Release configuration
		kc = tekton.KubeController{
			Commonctrl: *f.AsKubeAdmin.CommonController,
			Tektonctrl: *f.AsKubeAdmin.TektonController,
		}

		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "release-pull-secret", Namespace: managedNamespace},
			Data: map[string][]byte{".dockerconfigjson": sharedSecret.Data[".dockerconfigjson"]},
			Type: corev1.SecretTypeDockerConfigJson,
		}

		// release stuff
		_, err = f.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).ShouldNot(HaveOccurred())

		secret.Namespace = managedNamespace
		secret.Name = "release-pull-secret"
		secret.Type = corev1.SecretTypeDockerConfigJson
		_, err = f.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
		Expect(err).ShouldNot(HaveOccurred())
		managedServiceAccount, err := f.AsKubeAdmin.CommonController.CreateServiceAccount("release-service-account", managedNamespace, []corev1.ObjectReference{{Name: secret.Name}})
		Expect(err).NotTo(HaveOccurred())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(userNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())

		publicKey, err := kc.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())

		Expect(kc.CreateOrUpdateSigningSecret(publicKey, "cosign-public-key", managedNamespace)).To(Succeed())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlan("source-releaseplan", userNamespace, appName, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		components := []r.Component{{Name: componentName, Repository: constants.DefaultReleasedImagePushRepo}}
		sc := f.AsKubeAdmin.ReleaseController.GenerateReleaseStrategyConfig(components)
		scYaml, err := yaml.Marshal(sc)
		Expect(err).ShouldNot(HaveOccurred())

		scPath := "rhtap-demo.yaml"
		Expect(f.AsKubeAdmin.CommonController.Github.CreateRef("strategy-configs", "main", componentName)).To(Succeed())
		_, err = f.AsKubeAdmin.CommonController.Github.CreateFile("strategy-configs", scPath, string(scYaml), componentName)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("rhtap-demo-strategy", managedNamespace, "release", constants.ReleasePipelineImageRef, "rhtap-demo-policy", "release-service-account", []releaseApi.Params{
			{Name: "extraConfigGitUrl", Value: fmt.Sprintf("https://github.com/%s/strategy-configs.git", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))},
			{Name: "extraConfigPath", Value: scPath},
			{Name: "extraConfigGitRevision", Value: componentName},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission("demo", userNamespace, appName, managedNamespace, "", "", "rhtap-demo-strategy")
		Expect(err).NotTo(HaveOccurred())

		defaultEcPolicy, err := kc.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred())

		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   string(publicKey),
			Sources:     defaultEcPolicy.Spec.Sources,
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"minimal"},
				Exclude:     []string{"cve"},
			},
		}
		_, err = f.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy("rhtap-demo-policy", managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred())

		_, err = f.AsKubeAdmin.TektonController.CreatePVCInAccessMode("release-pvc", managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = f.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = f.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", "release-service-account", "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		Expect(err).NotTo(HaveOccurred())

		Expect(f.AsKubeAdmin.CommonController.Github.CreateRef(sampleRepoName, componentDefaultBranchName, componentNewBaseBranch)).To(Succeed())
		_, err = f.AsKubeAdmin.HasController.CreateApplication(appName, userNamespace)
		Expect(err).ShouldNot(HaveOccurred())
		_, err = f.AsKubeAdmin.GitOpsController.CreatePocEnvironment("rhtap-demo-test", userNamespace)
		Expect(err).ShouldNot(HaveOccurred())
		integrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario_beta1(appName, userNamespace, testScenarioGitURL, testScenarioRevision, testScenarioPathInRepo)
		Expect(err).ShouldNot(HaveOccurred())
	})
	AfterAll(func() {
		err = f.AsKubeAdmin.CommonController.Github.DeleteRef(sampleRepoName, componentNewBaseBranch)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
		}
		if !CurrentSpecReport().Failed() {
			Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			Expect(f.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(Succeed())
			Expect(f.AsKubeAdmin.CommonController.Github.DeleteRef("strategy-configs", componentName)).To(Succeed())
		}
	})

	Describe("RHTAP advanced pipeline, JVM rebuild, successful release, switch to simple build", Label("rhtap-demo"), Ordered, func() {

		var pacControllerHost, pacBranchName, pacPurgeBranchName string
		var prNumber int
		var mergeResult *github.PullRequestMergeResult
		var mergeResultSha string

		BeforeAll(func() {
			// Used for identifying related webhook on GitHub - in order to delete it
			// TODO: Remove when https://github.com/redhat-appstudio/infra-deployments/pull/1725 it is merged
			pacControllerRoute, err = f.AsKubeAdmin.CommonController.GetOpenshiftRoute("pipelines-as-code-controller", "pipelines-as-code")
			if err != nil {
				if k8sErrors.IsNotFound(err) {
					pacControllerRoute, err = f.AsKubeAdmin.CommonController.GetOpenshiftRoute("pipelines-as-code-controller", "openshift-pipelines")
				}
			}

			Expect(err).ShouldNot(HaveOccurred())
			pacControllerHost = pacControllerRoute.Spec.Host

			pacBranchName = fmt.Sprintf("appstudio-%s", componentName)
			pacPurgeBranchName = fmt.Sprintf("appstudio-purge-%s", componentName)

			_, err = f.AsKubeAdmin.JvmbuildserviceController.CreateJBSConfig(constants.JBSConfigName, userNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(f.AsKubeAdmin.JvmbuildserviceController.WaitForCache(f.AsKubeAdmin.CommonController, userNamespace)).Should(Succeed())

		})
		AfterAll(func() {

			// Delete new branch created by PaC and a testing branch used as a component's base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(sampleRepoName, pacBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}

			// Delete created webhook from GitHub
			hooks, err := f.AsKubeAdmin.CommonController.Github.ListRepoWebhooks(sampleRepoName)
			Expect(err).NotTo(HaveOccurred())

			for _, h := range hooks {
				hookUrl := h.Config["url"].(string)
				if strings.Contains(hookUrl, pacControllerHost) {
					Expect(f.AsKubeAdmin.CommonController.Github.DeleteWebhook(sampleRepoName, h.GetID())).To(Succeed())
					break
				}
			}
			Expect(f.AsKubeAdmin.JvmbuildserviceController.DeleteJbsConfig(constants.JBSConfigName, userNamespace)).To(Succeed())
		})

		When("Component with PaC is created", func() {

			It("triggers creation of a PR in the sample repo", func() {
				componentObj := appstudioApi.ComponentSpec{
					ComponentName: componentName,
					Source: appstudioApi.ComponentSource{
						ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
							GitSource: &appstudioApi.GitSource{
								URL:      sampleRepoURL,
								Revision: componentNewBaseBranch,
							},
						},
					},
				}
				component, err = f.AsKubeAdmin.HasController.CreateComponent(componentObj, userNamespace, "", "", appName, false, utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo))
				Expect(err).ShouldNot(HaveOccurred())

				pacBranchName := fmt.Sprintf("appstudio-%s", component.GetName())

				var prSHA string
				Eventually(func() error {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(sampleRepoName)
					Expect(err).ShouldNot(HaveOccurred())
					for _, pr := range prs {
						if pr.Head.GetRef() == pacBranchName {
							prNumber = pr.GetNumber()
							prSHA = pr.GetHead().GetSHA()
							return nil
						}
					}
					return fmt.Errorf("could not get the expected PaC branch name %s", pacBranchName)
				}, pullRequestCreationTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for init PaC PR (branch %q) to be created against the %q repo", pacBranchName, sampleRepoName))

				// We actually don't need the "on-pull-request" PipelineRun to complete, so we can delete it
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, prSHA)
					if err == nil {
						Expect(kc.Tektonctrl.DeletePipelineRun(pipelineRun.Name, pipelineRun.Namespace)).To(Succeed())
						return nil
					}
					return err
				}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for init PaC PipelineRun to be present in the user namespace %q for component %q with a label pointing to %q", userNamespace, componentName, appName))

			})

			It("should eventually lead to triggering another PipelineRun after merging the PaC init branch ", func() {
				Eventually(func() error {
					mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(sampleRepoName, prNumber)
					return err
				}, mergePRTimeout).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v\n", err))

				mergeResultSha = mergeResult.GetSHA()

				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, mergeResultSha)
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for component %s/%s\n", userNamespace, componentName)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for a PipelineRun in namespace %q with label component label %q and application label %q and sha label %q to start", userNamespace, componentName, appName, mergeResultSha))
			})
		})

		When("SLSA level 3 customizable PipelineRun is created", func() {
			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, mergeResultSha, 2)).To(Succeed())
			})

			It("does not contain an annotation with a Snapshot Name", func() {
				Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(""))
			})
		})

		When("SLSA level 3 customizable PipelineRun completes successfully", func() {
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
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, mergeResultSha)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(build.ValidateBuildPipelineTestResults(pipelineRun, f.AsKubeAdmin.CommonController.KubeRest())).To(Succeed())
			})

			It("should validate pipelineRun is signed", func() {
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, mergeResultSha)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pipelineRun.Annotations["chains.tekton.dev/signed"]).To(Equal("true"))
			})

			It("should find the related Snapshot CR", func() {
				Eventually(func() error {
					snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", userNamespace)
					return err
				}, snapshotTimeout, snapshotPollingInterval).Should(Succeed(), "timed out when trying to check if the Snapshot exists for PipelineRun %s/%s", userNamespace, pipelineRun.GetName())
			})

			It("should validate the pipelineRun is annotated with the name of the Snapshot", func() {
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, mergeResultSha)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(snapshot.GetName()))
			})

			It("should find the related Integration Test PipelineRun", func() {
				Eventually(func() error {
					testPipelinerun, err = f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot.Name, userNamespace)
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

		When("Integration Test PipelineRun is created", func() {
			It("should eventually complete successfully", func() {
				Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenario, snapshot, userNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a integration pipeline for snapshot %s/%s to finish", userNamespace, snapshot.GetName()))
			})
		})

		When("Integration Test PipelineRun completes successfully", func() {

			It("should lead to Snapshot CR being marked as passed", func() {
				snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", userNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot)).To(BeTrue(), fmt.Sprintf("tests have not succeeded for snapshot %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
			})

			It("should trigger creation of Release CR", func() {
				Eventually(func() error {
					release, err = f.AsKubeAdmin.ReleaseController.GetRelease("", snapshot.Name, userNamespace)
					return err
				}, releaseTimeout, releasePollingInterval).Should(Succeed(), fmt.Sprintf("timed out when trying to check if the release exists for snapshot %s/%s", userNamespace, snapshot.GetName()))
			})
		})

		When("Release CR is created", func() {
			It("triggers creation of Release PipelineRun", func() {
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
					if err != nil {
						GinkgoWriter.Printf("pipelineRun for component '%s' in namespace '%s' not created yet: %+v\n", componentName, managedNamespace, err)
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
				Skip("Skip until bug is fixed: https://issues.redhat.com/browse/RHTAPBUGS-356")
				Eventually(func() error {
					pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(utils.HasPipelineRunFailed(pipelineRun)).NotTo(BeTrue(), fmt.Sprintf("did not expect PipelineRun %s/%s to fail", pipelineRun.GetNamespace(), pipelineRun.GetName()))
					if pipelineRun.IsDone() {
						Expect(utils.HasPipelineRunSucceeded(pipelineRun)).To(BeTrue(), fmt.Sprintf("PipelineRun %s/%s did not succeed", pipelineRun.GetNamespace(), pipelineRun.GetName()))
					}
					return nil
				}, releasePipelineTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("failed to see pipelinerun %q in namespace %q with a label pointing to release %q in namespace %q to complete successfully", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
			})
		})
		When("Release PipelineRun is completed", func() {
			It("should lead to Release CR being marked as succeeded", func() {
				Skip("Skip until bug is fixed: https://issues.redhat.com/browse/RHTAPBUGS-356")
				Eventually(func() error {
					release, err = f.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					if !release.IsReleased() {
						return fmt.Errorf("release CR %s/%s is not marked as finished yet", release.GetNamespace(), release.GetName())
					}
					return nil
				}, customResourceUpdateTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("failed to see release %q in namespace %q get marked as released", release.Name, userNamespace))
			})
		})

		When("JVM Build Service is used for rebuilding dependencies", func() {
			It("should eventually rebuild of all artifacts and dependencies successfully", func() {
				Eventually(func() error {
					abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(userNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					for _, ab := range abList.Items {
						if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
							return fmt.Errorf("artifactbuild %s not complete", ab.Spec.GAV)
						}
					}
					dbList, err := f.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(userNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					for _, db := range dbList.Items {
						if db.Status.State != v1alpha1.DependencyBuildStateComplete {
							return fmt.Errorf("dependencybuild %s not complete", db.Spec.ScmInfo.SCMURL)
						}
					}
					return nil
				}, jvmRebuildTimeout, jvmRebuildPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for all artifactbuilds and dependencybuilds to complete in namespace %q", userNamespace))
			})
		})

		When("User switches to simple build", func() {
			BeforeAll(func() {
				comp, err := f.AsKubeAdmin.HasController.GetComponent(componentName, userNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				comp.Annotations["appstudio.openshift.io/pac-provision"] = "delete"
				Expect(f.AsKubeAdmin.CommonController.KubeRest().Update(context.TODO(), comp)).To(Succeed())
			})
			AfterAll(func() {
				// Delete the new branch created by sending purge PR while moving to simple build
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(sampleRepoName, pacPurgeBranchName)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
				}
			})
			It("creates a pull request for removing PAC configuration", func() {
				Eventually(func() error {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(sampleRepoName)
					Expect(err).ShouldNot(HaveOccurred())
					for _, pr := range prs {
						if pr.Head.GetRef() == pacPurgeBranchName {
							return nil
						}
					}
					return fmt.Errorf("could not get the expected PaC purge PR branch %s", pacPurgeBranchName)
				}, time.Minute*1, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for PaC purge PR to be created against the %q repo", sampleRepoName))
			})
		})
	})
})
