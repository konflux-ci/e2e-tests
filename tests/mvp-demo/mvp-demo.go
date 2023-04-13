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
	"knative.dev/pkg/apis"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/yaml"
)

const (
	// This app might be replaced with service-registry in a future
	sampleRepoName             = "hacbs-test-project"
	componentDefaultBranchName = "main"

	testNamespace = "mvp-demo-dev-namespace"

	appName       = "mvp-test-app"
	componentName = "mvp-test-component"

	managedNamespace = "mvp-demo-managed-namespace"

	releasePipelineTimeout = time.Minute * 15
)

var sampleRepoURL = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), sampleRepoName)

var _ = framework.MvpDemoSuiteDescribe("MVP Demo tests", Label("mvp-demo"), func() {

	defer GinkgoRecover()
	var err error
	var f *framework.Framework
	var sharedSecret *corev1.Secret

	var untrustedPipelineBundle, componentNewBaseBranch, userNamespace string

	var kc tekton.KubeController

	var pipelineRun *tektonapi.PipelineRun
	var release *releaseApi.Release
	var snapshot *appstudioApi.Snapshot

	BeforeAll(func() {
		// This pipeline contains an image that comes from "not allowed" container image registry repo
		// https://github.com/hacbs-contract/ec-policies/blob/de8afa912e7a80d02abb82358ce7b23cf9a286c8/data/rule_data.yml#L9-L12
		// It is required in order to test that the release of the image failed based on a failed check in EC
		untrustedPipelineBundle, err = createUntrustedPipelineBundle()
		klog.Info(untrustedPipelineBundle)
		Expect(err).NotTo(HaveOccurred())
		f, err = framework.NewFramework(utils.GetGeneratedNamespace(testNamespace))
		Expect(err).NotTo(HaveOccurred())
		userNamespace = f.UserNamespace
		Expect(userNamespace).NotTo(BeEmpty())

		componentNewBaseBranch = fmt.Sprintf("base-%s", util.GenerateRandomString(4))

		sharedSecret, err = f.AsKubeAdmin.CommonController.GetSecret(constants.SharedPullSecretNamespace, constants.SharedPullSecretName)
		Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error when getting shared secret - make sure the secret %s in %s userNamespace is created", constants.SharedPullSecretName, constants.SharedPullSecretNamespace))

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

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-strategy", managedNamespace, "release", constants.ReleasePipelineImageRef, "mvp-policy", "release-service-account", []releaseApi.Params{
			{Name: "extraConfigGitUrl", Value: "https://github.com/redhat-appstudio-qe/strategy-configs.git"},
			{Name: "extraConfigPath", Value: "mvp.yaml"},
			{Name: "extraConfigRevision", Value: "main"},
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
		})

		It("sample app can be built successfully", func() {
			_, err = f.AsKubeAdmin.HasController.CreateComponent(appName, componentName, userNamespace, sampleRepoURL, componentNewBaseBranch, "", constants.DefaultImagePushRepo, "", true)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(f.AsKubeAdmin.CommonController, componentName, appName, userNamespace, "")).To(Succeed())
		})

		It("sample app is successfully deployed to dev environment", func() {
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.DeploymentIsCompleted(componentName, userNamespace, 1), time.Minute*20)).To(Succeed())
		})

		It("sample app's route can be accessed", func() {
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.RouteHostnameIsAccessible(componentName, userNamespace), time.Minute*10)).To(Succeed())
		})

		It("application snapshot is created", func() {
			snapshot, err = f.AsKubeAdmin.IntegrationController.GetApplicationSnapshot("", "", componentName, userNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(snapshot.Name).ToNot(BeEmpty())
		})

		It("Release is created", func() {
			release, err = f.AsKubeAdmin.ReleaseController.GetRelease("", snapshot.Name, userNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(release.Name).ToNot(BeEmpty())
		})

		It("Release PipelineRun is triggered", func() {
			Eventually(func() bool {
				pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
				if err != nil {
					GinkgoWriter.Printf("pipelineRun for component '%s' in namespace '%s' not created yet: %+v\n", componentName, managedNamespace, err)
					return false
				}
				return pipelineRun.HasStarted()
			}, 2*time.Minute, 2*time.Second).Should(BeTrue())
		})

		It("Release status is updated", func() {
			Eventually(func() bool {
				release, err = f.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get Relase CR in '%s' namespace: %+v\n", managedNamespace, err)
					return false
				}
				return release.HasStarted()
			}, 1*time.Minute, 2*time.Second).Should(BeTrue())
		})

		It("Release PipelineRun should eventually fail", func() {
			Eventually(func() bool {
				pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get PipelineRun for a release '%s' in '%s' namespace: %+v\n", release.Name, managedNamespace, err)
					return false
				}
				return pipelineRun.IsDone()
			}, releasePipelineTimeout, 10*time.Second).Should(BeTrue())
		})

		It("associated Release should be marked as failed", func() {
			Eventually(func() bool {
				release, err = f.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get Release CR in '%s' namespace: %+v\n", managedNamespace, err)
					return false
				}
				return release.IsDone() && !release.HasSucceeded()
			}, 1*time.Minute, 2*time.Second).Should(BeTrue())
		})

	})

	Describe("MVP Demo Chapter 2 - advanced pipeline, JVM rebuild, successful release", Label("mvp-demo-chapter-2"), Ordered, func() {

		var pacControllerHost, pacBranchName string
		var prNumber int
		var mergeResult *github.PullRequestMergeResult
		var mergeResultSha string

		BeforeAll(func() {
			// Used for identifying related webhook on GitHub - in order to delete it
			pacControllerRoute, err := f.AsKubeAdmin.CommonController.GetOpenshiftRoute("pipelines-as-code-controller", "pipelines-as-code")
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

			timeout := time.Minute * 5
			interval := time.Second * 1
			pacBranchName := fmt.Sprintf("appstudio-%s", componentName)

			var prSHA string
			Eventually(func() bool {
				prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(sampleRepoName)
				Expect(err).ShouldNot(HaveOccurred())

				for _, pr := range prs {
					if pr.Head.GetRef() == pacBranchName {
						prNumber = pr.GetNumber()
						prSHA = pr.GetHead().GetSHA()
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for init PaC PR to be created")

			// We actually don't need the "on-pull-request" PipelineRun to complete, so we can delete it
			Eventually(func() bool {
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, prSHA)
				if err == nil {
					Expect(kc.Tektonctrl.DeletePipelineRun(pipelineRun.Name, pipelineRun.Namespace)).To(Succeed())
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for init PaC PipelineRun to be present in the user namespace")

		})

		It("merging the PaC init branch eventually leads to triggering another PipelineRun", func() {
			Eventually(func() error {
				mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(sampleRepoName, prNumber)
				return err
			}, time.Minute).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v\n", err))

			mergeResultSha = mergeResult.GetSHA()

			timeout := time.Minute * 2
			interval := time.Second * 1

			Eventually(func() bool {
				pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, mergeResultSha)
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
		})

		It("SLSA level 3 customizable pipeline completes successfully", func() {
			Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(f.AsKubeAdmin.CommonController, componentName, appName, userNamespace, mergeResultSha)).To(Succeed())
		})

		It("resulting SBOM file can be downloaded", func() {
			pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, "")
			Expect(err).ShouldNot(HaveOccurred())

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

		It("application snapshot is created", func() {
			snapshot, err = f.AsKubeAdmin.IntegrationController.GetApplicationSnapshot("", pipelineRun.Name, "", userNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(snapshot.Name).ToNot(BeEmpty())
		})

		It("Release is created and Release PipelineRun is triggered and Release status is updated", func() {
			release, err = f.AsKubeAdmin.ReleaseController.GetRelease("", snapshot.Name, userNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(release.Name).ToNot(BeEmpty())

			Eventually(func() bool {
				pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
				if err != nil {
					GinkgoWriter.Printf("pipelineRun for component '%s' in namespace '%s' not created yet: %+v\n", componentName, managedNamespace, err)
					return false
				}
				return pipelineRun.HasStarted()
			}, 2*time.Minute, 2*time.Second).Should(BeTrue())

			Eventually(func() bool {
				release, err = f.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get Relase CR in '%s' namespace: %+v\n", managedNamespace, err)
					return false
				}
				return release.HasStarted()
			}, 1*time.Minute, 2*time.Second).Should(BeTrue())
		})

		It("Release PipelineRun should eventually succeed and associated Release should be marked as succeeded", func() {
			Eventually(func() bool {
				pipelineRun, err = f.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get PipelineRun for a release '%s' in '%s' namespace: %+v\n", release.Name, managedNamespace, err)
					return false
				}
				return pipelineRun.IsDone() && pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineTimeout, 10*time.Second).Should(BeTrue())

			Eventually(func() bool {
				release, err = f.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", userNamespace)
				if err != nil {
					GinkgoWriter.Printf("failed to get Release CR in '%s' namespace: %+v\n", managedNamespace, err)
					return false
				}
				return release.IsDone() && release.HasSucceeded()
			}, 1*time.Minute, 2*time.Second).Should(BeTrue())
		})

		It("JVM Build Service is used for rebuilding dependencies and completes rebuild of all artifacts and dependencies", func() {
			timeout := time.Minute * 20
			interval := time.Second * 10

			Eventually(func() bool {
				abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(userNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing artifactbuilds: %s\n", err.Error())
					return false
				}
				for _, ab := range abList.Items {
					if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
						GinkgoWriter.Printf("artifactbuild %s not complete\n", ab.Spec.GAV)
						return false
					}
				}
				dbList, err := f.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(userNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing dependencybuilds: %s\n", err.Error())
					return false
				}
				for _, db := range dbList.Items {
					if db.Status.State != v1alpha1.DependencyBuildStateComplete {
						GinkgoWriter.Printf("dependencybuild %s not complete\n", db.Spec.ScmInfo.SCMURL)
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for all artifactbuilds and dependencybuilds to complete")
		})

	})

})

func createUntrustedPipelineBundle() (string, error) {
	var err error
	var defaultBundleRef string
	var tektonObj runtime.Object

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	var newBuildahTaskRef, _ = name.ParseReference(fmt.Sprintf("%s:task-bundle-%s", constants.DefaultImagePushRepo, tag))
	var newDockerBuildPipelineRef, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", constants.DefaultImagePushRepo, tag))
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
