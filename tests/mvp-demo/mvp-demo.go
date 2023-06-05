package mvp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-github/v44/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	"knative.dev/pkg/apis"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	integrationv1alpha1 "github.com/redhat-appstudio/integration-service/api/v1alpha1"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/yaml"
)

const (
	// This app might be replaced with service-registry in a future
	sampleRepoName             = "hacbs-test-project"
	componentDefaultBranchName = "main"

	// Kubernetes resource names
	testNamespacePrefix = "mvp-dev"
	managedNamespace    = "mvp-managed"

	appName        = "mvp-test-app"
	BundleURL      = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass"
	InPipelineName = "integration-pipeline-pass"

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
	defaultPollingInterval     = time.Second * 2
	jvmRebuildPollingInterval  = time.Second * 10
	pipelineRunPollingInterval = time.Second * 10
	snapshotPollingInterval    = time.Second * 1
	releasePollingInterval     = time.Second * 1
)

var sampleRepoURL = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), sampleRepoName)

var _ = framework.MvpDemoSuiteDescribe("MVP Demo tests", Label("mvp-demo"), func() {

	defer GinkgoRecover()
	var err error
	var f *framework.Framework
	var sharedSecret *corev1.Secret
	var pacControllerRoute *routev1.Route
	var componentName string

	var untrustedPipelineBundle, componentNewBaseBranch, userNamespace string

	var kc tekton.KubeController

	// set vs. simply declare these pointers so we can use them in debug, where an empty name is indicative of Get's failing
	pipelineRun := &tektonapi.PipelineRun{}
	release := &releaseApi.Release{}
	snapshot := &appstudioApi.Snapshot{}
	testPipelinerun := &tektonapi.PipelineRun{}
	integrationTestScenario := &integrationv1alpha1.IntegrationTestScenario{}

	BeforeAll(func() {
		// This pipeline contains an image that comes from "not allowed" container image registry repo
		// https://github.com/hacbs-contract/ec-policies/blob/de8afa912e7a80d02abb82358ce7b23cf9a286c8/data/rule_data.yml#L9-L12
		// It is required in order to test that the release of the image failed based on a failed check in EC
		untrustedPipelineBundle, err = createUntrustedPipelineBundle()
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("generated untrusted pipeline bundle: %s", untrustedPipelineBundle)

		f, err = framework.NewFramework(utils.GetGeneratedNamespace(testNamespacePrefix))
		Expect(err).NotTo(HaveOccurred())
		userNamespace = f.UserNamespace
		Expect(userNamespace).NotTo(BeEmpty())

		componentName = fmt.Sprintf("test-mvp-component-%s", util.GenerateRandomString(4))
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
		_, err = f.AsKubeAdmin.CommonController.CreateServiceAccount("release-service-account", managedNamespace, []corev1.ObjectReference{{Name: secret.Name}})
		Expect(err).NotTo(HaveOccurred())

		publicKey, err := kc.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())

		Expect(kc.CreateOrUpdateSigningSecret(publicKey, "cosign-public-key", managedNamespace)).To(Succeed())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlan("source-releaseplan", userNamespace, appName, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		sc := f.AsKubeAdmin.ReleaseController.GenerateReleaseStrategyConfig(componentName, constants.DefaultReleasedImagePushRepo)
		scYaml, err := yaml.Marshal(sc)
		Expect(err).ShouldNot(HaveOccurred())

		scPath := "mvp-demo.yaml"
		Expect(f.AsKubeAdmin.CommonController.Github.CreateRef("strategy-configs", "main", componentName)).To(Succeed())
		_, err = f.AsKubeAdmin.CommonController.Github.CreateFile("strategy-configs", scPath, string(scYaml), componentName)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-strategy", managedNamespace, "release", constants.ReleasePipelineImageRef, "mvp-policy", "release-service-account", []releaseApi.Params{
			{Name: "extraConfigGitUrl", Value: fmt.Sprintf("https://github.com/%s/strategy-configs.git", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))},
			{Name: "extraConfigPath", Value: scPath},
			{Name: "extraConfigGitRevision", Value: componentName},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission("demo", userNamespace, appName, managedNamespace, "", "", "mvp-strategy")
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
		_, err = f.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy("mvp-policy", managedNamespace, defaultEcPolicySpec)
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

	Describe("MVP Demo Chapter 1 - basic build & deploy, failed release", Label("mvp-demo-chapter-1"), Ordered, func() {

		BeforeAll(func() {
			Expect(f.AsKubeAdmin.CommonController.Github.CreateRef(sampleRepoName, componentDefaultBranchName, componentNewBaseBranch)).To(Succeed())
			_, err = f.AsKubeAdmin.HasController.CreateHasApplication(appName, userNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = f.AsKubeAdmin.IntegrationController.CreateEnvironment(userNamespace, "mvp-test")
			Expect(err).ShouldNot(HaveOccurred())
			ps := &buildservice.BuildPipelineSelector{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "build-pipeline-selector",
					Namespace: userNamespace,
				},
				Spec: buildservice.BuildPipelineSelectorSpec{Selectors: []buildservice.PipelineSelector{
					{
						Name: "custom-selector",
						PipelineRef: tektonapi.PipelineRef{
							Name:   "docker-build",
							Bundle: untrustedPipelineBundle,
						},
						WhenConditions: buildservice.WhenCondition{
							DockerfileRequired: pointer.Bool(true),
							ComponentName:      componentName,
							Annotations:        map[string]string{"skip-initial-checks": "true"},
							Labels:             constants.ComponentDefaultLabel,
						},
					},
				}},
			}

			Expect(f.AsKubeAdmin.CommonController.KubeRest().Create(context.TODO(), ps)).To(Succeed())
			integrationTestScenario, err = f.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario(appName, userNamespace, BundleURL, InPipelineName)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("sample app can be built successfully", func() {
			_, err = f.AsKubeAdmin.HasController.CreateComponent(appName, componentName, userNamespace, sampleRepoURL, componentNewBaseBranch, "", "", "", true)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(f.AsKubeAdmin.CommonController, componentName, appName, userNamespace, "")).To(Succeed())
		})

		It("sample app is successfully deployed to dev environment", func() {
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.DeploymentIsCompleted(componentName, userNamespace, 1), appDeployTimeout)).To(Succeed())
		})

		It("sample app's route can be accessed", func() {
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.RouteHostnameIsAccessible(componentName, userNamespace), appRouteAvailableTimeout)).To(Succeed())
		})

		It("Snapshot is created", func() {
			Eventually(func() (bool, error) {
				snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", "", componentName, userNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
					return false, err
				}
				return true, nil
			}, snapshotTimeout, snapshotPollingInterval).Should(BeTrue())
		})

		It("Integration Test PipelineRun is created", func() {
			Eventually(func() (bool, error) {
				testPipelinerun, err = f.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot.Name, userNamespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get Integration test PipelineRun for a snapshot '%s' in '%s' namespace: %+v\n", snapshot.Name, userNamespace, err)
					return false, err
				}
				return testPipelinerun.HasStarted(), nil
			}, pipelineRunStartedTimeout, defaultPollingInterval).Should(BeTrue())
			Expect(testPipelinerun.Spec.PipelineRef.Bundle).To(ContainSubstring(integrationTestScenario.Spec.Bundle))
			Expect(testPipelinerun.Labels["appstudio.openshift.io/snapshot"]).To(ContainSubstring(snapshot.Name))
			Expect(testPipelinerun.Labels["test.appstudio.openshift.io/scenario"]).To(ContainSubstring(integrationTestScenario.Name))
		})

		It("Integration Test PipelineRun should eventually succeed", func() {
			Expect(f.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(f.AsKubeAdmin.CommonController, integrationTestScenario, snapshot, appName, userNamespace)).To(Succeed(), "Error when waiting for a integration pipeline to finish")
		})

		It("Snapshot is marked as passed", func() {
			Expect(f.AsKubeAdmin.IntegrationController.HaveTestsSucceeded(snapshot)).To(BeTrue())
		})

		It("Release is created", func() {
			release, err = f.AsKubeAdmin.ReleaseController.GetRelease("", snapshot.Name, userNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(release.Name).ToNot(BeEmpty())
		})

		It("Release PipelineRun is triggered", func() {
			Eventually(func() (bool, error) {
				pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
				if err != nil {
					GinkgoWriter.Printf("pipelineRun for component '%s' in namespace '%s' not created yet: %+v\n", componentName, managedNamespace, err)
					return false, err
				}
				return pipelineRun.HasStarted(), nil
			}, pipelineRunStartedTimeout, defaultPollingInterval).Should(BeTrue(), fmt.Sprintf("component %q did not start a pipelinerun named %q in namespace %q from release %q in namespace %q in time", componentName, pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
		})

		It("Release PipelineRun should eventually fail", func() {
			Eventually(func() (bool, error) {
				pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get PipelineRun for a release '%s' in '%s' namespace: %+v\n", release.Name, managedNamespace, err)
					return false, err
				}
				return pipelineRun.IsDone(), nil
			}, releasePipelineTimeout, pipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("the pipelinerun %q in namespace %q for release %q in namespace %q did not fail in time", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
		})

		It("associated Release should be marked as failed", func() {
			Eventually(func() (bool, error) {
				release, err = f.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get Release CR in '%s' namespace: %+v\n", managedNamespace, err)
					return false, err
				}
				return release.HasReleaseFinished() && !release.IsReleased(), nil
			}, customResourceUpdateTimeout, defaultPollingInterval).Should(BeTrue(), fmt.Sprintf("the release %q in namespace %q was not marked as released but failed", release.Name, release.Namespace))
		})

	})

	Describe("MVP Demo Chapter 2 - advanced pipeline, JVM rebuild, successful release", Label("mvp-demo-chapter-2"), Ordered, func() {

		var pacControllerHost, pacBranchName string
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

			_, err = f.AsKubeAdmin.JvmbuildserviceController.CreateJBSConfig(constants.JBSConfigName, userNamespace, utils.GetQuayIOOrganization())
			Expect(err).ShouldNot(HaveOccurred())
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: constants.JVMBuildImageSecretName, Namespace: userNamespace},
				Data: map[string][]byte{".dockerconfigjson": sharedSecret.Data[".dockerconfigjson"]},
				Type: corev1.SecretTypeDockerConfigJson,
			}
			_, err = f.AsKubeAdmin.CommonController.CreateSecret(userNamespace, secret)
			Expect(err).ShouldNot(HaveOccurred())

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
		})

		It("upgrading to SLSA level 3 customizable pipeline triggers creation of a PR in the sample repo", func() {
			comp, err := f.AsKubeAdmin.HasController.GetHasComponent(componentName, userNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			comp.Annotations["skip-initial-checks"] = "false"
			for k, v := range constants.ComponentPaCRequestAnnotation {
				comp.Annotations[k] = v
			}
			Expect(f.AsKubeAdmin.CommonController.KubeRest().Update(context.TODO(), comp)).To(Succeed())

			pacBranchName := fmt.Sprintf("appstudio-%s", componentName)

			var prSHA string
			Eventually(func() (bool, error) {
				prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(sampleRepoName)
				if err != nil {
					return false, err
				}
				for _, pr := range prs {
					if pr.Head.GetRef() == pacBranchName {
						prNumber = pr.GetNumber()
						prSHA = pr.GetHead().GetSHA()
						return true, nil
					}
				}
				return false, fmt.Errorf("could not get the expected PaC branch name %s", pacBranchName)
			}, pullRequestCreationTimeout, defaultPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR to be created against the %q repo", sampleRepoName))

			// We actually don't need the "on-pull-request" PipelineRun to complete, so we can delete it
			Eventually(func() (bool, error) {
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, prSHA)
				if err == nil {
					Expect(kc.Tektonctrl.DeletePipelineRun(pipelineRun.Name, pipelineRun.Namespace)).To(Succeed())
					return true, nil
				}
				return false, err
			}, pipelineRunStartedTimeout, pipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PipelineRun to be present in the user namespace %q for component %q with a label pointing to %q", userNamespace, componentName, appName))

		})

		It("merging the PaC init branch eventually leads to triggering another PipelineRun", func() {
			Eventually(func() error {
				mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(sampleRepoName, prNumber)
				return err
			}, mergePRTimeout).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v\n", err))

			mergeResultSha = mergeResult.GetSHA()

			Eventually(func() (bool, error) {
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, mergeResultSha)
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false, err
				}
				return pipelineRun.HasStarted(), nil
			}, pipelineRunStartedTimeout, pipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for a PipelineRun in namespace %q with label component label %q and application label %q and sha label %q to start", userNamespace, componentName, appName, mergeResultSha))
		})

		It("SLSA level 3 customizable pipeline completes successfully", func() {
			Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(f.AsKubeAdmin.CommonController, componentName, appName, userNamespace, mergeResultSha)).To(Succeed())
		})

		It("resulting SBOM file can be downloaded", func() {
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

		It("validation of Tekton TaskRun test results completes successfully", func() {
			pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, mergeResultSha)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(build.ValidateBuildPipelineTestResults(pipelineRun, f.AsKubeAdmin.CommonController.KubeRest())).To(Succeed())
		})

		It("Snapshot is created", func() {
			Eventually(func() (bool, error) {
				snapshot, err = f.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", userNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Snapshot: %v\n", err)
					return false, err
				}
				return true, nil
			}, snapshotTimeout, snapshotPollingInterval).Should(BeTrue(), "timed out when trying to check if the Snapshot exists")
		})

		It("Release is created and Release PipelineRun is triggered and Release status is updated", func() {
			Eventually(func() (bool, error) {
				release, err = f.AsKubeAdmin.ReleaseController.GetRelease("", snapshot.Name, userNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the release: %v\n", err)
					return false, err
				}
				return true, nil
			}, releaseTimeout, releasePollingInterval).Should(BeTrue(), "timed out when trying to check if the release exists")

			Eventually(func() (bool, error) {
				pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
				if err != nil {
					GinkgoWriter.Printf("pipelineRun for component '%s' in namespace '%s' not created yet: %+v\n", componentName, managedNamespace, err)
					return false, err
				}
				return pipelineRun.HasStarted(), nil
			}, pipelineRunStartedTimeout, defaultPollingInterval).Should(BeTrue(), fmt.Sprintf("failed to get pipelinerun named %q in namespace %q with label to release %q in namespace %q to start", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))

			Eventually(func() (bool, error) {
				release, err = f.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get Release CR in '%s' namespace: %+v\n", managedNamespace, err)
					return false, err
				}
				return release.IsReleasing(), nil
			}, customResourceUpdateTimeout, defaultPollingInterval).Should(BeTrue(), fmt.Sprintf("failed to get release %q in namespace %q to releasing state", release.Name, userNamespace))
		})

		It("Release PipelineRun should eventually succeed and associated Release should be marked as succeeded", func() {
			Skip("Skip until bug is fixed: https://issues.redhat.com/browse/RHTAPBUGS-356")
			Eventually(func() (bool, error) {
				pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get PipelineRun for a release '%s' in '%s' namespace: %+v\n", release.Name, managedNamespace, err)
					return false, err
				}
				Expect(utils.PipelineRunFailed(pipelineRun)).NotTo(BeTrue(), fmt.Sprintf("did not expect PipelineRun %s:%s to fail", pipelineRun.GetNamespace(), pipelineRun.GetName()))
				return pipelineRun.IsDone() && pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue(), nil
			}, releasePipelineTimeout, pipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("failed to see pipelinerun %q in namespace %q with a label pointing to release %q in namespace %q to complete successfully", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))

			Eventually(func() (bool, error) {
				release, err = f.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get Release CR in '%s' namespace: %+v\n", managedNamespace, err)
					return false, err
				}
				return release.IsReleased(), nil
			}, customResourceUpdateTimeout, defaultPollingInterval).Should(BeTrue(), fmt.Sprintf("failed to see release %q in namespace %q get marked as released", release.Name, userNamespace))
		})

		It("JVM Build Service is used for rebuilding dependencies and completes rebuild of all artifacts and dependencies", func() {
			Eventually(func() (bool, error) {
				abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(userNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing artifactbuilds: %s\n", err.Error())
					return false, err
				}
				for _, ab := range abList.Items {
					if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
						GinkgoWriter.Printf("artifactbuild %s not complete\n", ab.Spec.GAV)
						return false, err
					}
				}
				dbList, err := f.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(userNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing dependencybuilds: %s\n", err.Error())
					return false, err
				}
				for _, db := range dbList.Items {
					if db.Status.State != v1alpha1.DependencyBuildStateComplete {
						GinkgoWriter.Printf("dependencybuild %s not complete\n", db.Spec.ScmInfo.SCMURL)
						return false, err
					}
				}
				return true, nil
			}, jvmRebuildTimeout, jvmRebuildPollingInterval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for all artifactbuilds and dependencybuilds to complete in namespace %q", userNamespace))
		})

	})

})

func createUntrustedPipelineBundle() (string, error) {
	var err error
	var defaultBundleRef string
	var tektonObj runtime.Object

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.DEFAULT_QUAY_ORG_ENV, constants.DefaultQuayOrg)
	newBuildahTaskRefImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newBuildahTaskRef, _ = name.ParseReference(fmt.Sprintf("%s:task-bundle-%s", newBuildahTaskRefImg, tag))
	newDockerBuildPipelineRefImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newDockerBuildPipelineRef, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newDockerBuildPipelineRefImg, tag))
	var newBuildImage = "quay.io/containers/buildah:latest"
	var newTaskYaml, newPipelineYaml []byte

	if err = utils.CreateDockerConfigFile(os.Getenv("QUAY_TOKEN")); err != nil {
		return "", fmt.Errorf("failed to create docker config file: %+v", err)
	}
	if defaultBundleRef, err = utils.GetDefaultPipelineBundleRef(constants.BuildPipelineSelectorYamlURL, "Docker build"); err != nil {
		return "", fmt.Errorf("failed to get the pipeline bundle ref: %+v", err)
	}
	if tektonObj, err = utils.ExtractTektonObjectFromBundle(defaultBundleRef, "pipeline", "docker-build"); err != nil {
		return "", fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
	}
	dockerPipelineObj := tektonObj.(tektonapi.PipelineObject)

	var currentBuildahTaskRef string
	for _, t := range dockerPipelineObj.PipelineSpec().Tasks {
		if t.TaskRef.Name == "buildah" {
			currentBuildahTaskRef = t.TaskRef.Bundle
			t.TaskRef.Bundle = newBuildahTaskRef.String()
		}
	}
	if tektonObj, err = utils.ExtractTektonObjectFromBundle(currentBuildahTaskRef, "task", "buildah"); err != nil {
		return "", fmt.Errorf("failed to extract the Tekton Task from bundle: %+v", err)
	}
	taskObj := tektonObj.(tektonapi.TaskObject)

	for i, s := range taskObj.TaskSpec().Steps {
		if s.Name == "build" {
			taskObj.TaskSpec().Steps[i].Image = newBuildImage
		}
	}

	if newTaskYaml, err = yaml.Marshal(taskObj); err != nil {
		return "", fmt.Errorf("error when marshalling a new task to YAML: %v", err)
	}
	if newPipelineYaml, err = yaml.Marshal(dockerPipelineObj); err != nil {
		return "", fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
	}

	keychain := authn.NewMultiKeychain(authn.DefaultKeychain)
	authOption := remoteimg.WithAuthFromKeychain(keychain)

	if err = utils.BuildAndPushTektonBundle(newTaskYaml, newBuildahTaskRef, authOption); err != nil {
		return "", fmt.Errorf("error when building/pushing a tekton task bundle: %v", err)
	}
	if err = utils.BuildAndPushTektonBundle(newPipelineYaml, newDockerBuildPipelineRef, authOption); err != nil {
		return "", fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
	}

	return newDockerBuildPipelineRef.Name(), nil
}
