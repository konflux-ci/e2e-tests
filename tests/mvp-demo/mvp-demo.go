package mvp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/go-github/v44/github"
	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
)

const (
	// This app might be replaced with service-registry in a future
	sampleRepoName             = "hacbs-test-project"
	componentDefaultBranchName = "main"

	testNamespace = "mvp-demo-dev-namespace"

	appName       = "mvp-test-app"
	componentName = "mvp-test-component"

	managedNamespace = "mvp-demo-managed-namespace"

	// This pipeline contains an image that comes from "not allowed" container image registry repo
	// https://github.com/hacbs-contract/ec-policies/blob/de8afa912e7a80d02abb82358ce7b23cf9a286c8/data/rule_data.yml#L9-L12
	// It is required in order to test that the release of the image failed based on a failed check in EC
	untrustedPipelineBundle = "quay.io/psturc/pipeline-docker-build:2023-02-17-162546@sha256:470155be2886a81fd03afae53b559beec038d449d712aa54b93788c7a719f50a"
)

var sampleRepoURL = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), sampleRepoName)

var _ = framework.MvpDemoSuiteDescribe("MVP Demo tests", Label("mvp-demo"), func() {

	defer GinkgoRecover()
	var err error
	var f *framework.Framework
	var sharedSecret *corev1.Secret

	var componentNewBaseBranch, userNamespace string

	var kc tekton.KubeController

	BeforeAll(func() {
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

		publicKey, err := kc.GetPublicKey("signing-secrets", constants.TEKTON_CHAINS_NS)
		Expect(err).ToNot(HaveOccurred())

		Expect(kc.CreateOrUpdateSigningSecret(publicKey, "cosign-public-key", managedNamespace)).To(Succeed())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlan("source-releaseplan", userNamespace, appName, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-strategy", managedNamespace, "release", "quay.io/hacbs-release/pipeline-release:main", "mvp-policy", "release-service-account", []appstudiov1alpha1.Params{
			{Name: "extraConfigGitUrl", Value: "https://github.com/redhat-appstudio-qe/strategy-configs.git"},
			{Name: "extraConfigPath", Value: "mvp.yaml"},
			{Name: "extraConfigRevision", Value: "main"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission("demo", userNamespace, appName, managedNamespace, "", "", "mvp-strategy")
		Expect(err).NotTo(HaveOccurred())

		defaultEcPolicy := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   string(publicKey),
			Sources: []ecp.Source{
				{
					Name: "ec-policies",
					Policy: []string{
						"git::https://github.com/hacbs-contract/ec-policies.git//policy",
					},
					Data: []string{
						"git::https://github.com/hacbs-contract/ec-policies.git//data",
					},
				},
			},
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"minimal"},
				Exclude:     []string{"cve"},
			},
		}
		_, err = f.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy("mvp-policy", managedNamespace, defaultEcPolicy)
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
						PipelineRef: v1beta1.PipelineRef{
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

		It("release should fail", func() {
			Eventually(func() bool {
				releaseCreated, err := f.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(userNamespace)
				if releaseCreated == nil || err != nil {
					return false
				}

				return releaseCreated.HasStarted() && releaseCreated.IsDone() && releaseCreated.Status.Conditions[0].Status == "False"
			}, 10*time.Minute, 10*time.Second).Should(BeTrue())
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

			Eventually(func() bool {
				prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(sampleRepoName)
				Expect(err).ShouldNot(HaveOccurred())

				for _, pr := range prs {
					if pr.Head.GetRef() == pacBranchName {
						prNumber = pr.GetNumber()
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for init PaC PR to be created")

		})

		It("merging the PaC init branch eventually leads to triggering another PipelineRun", func() {
			Eventually(func() error {
				mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(sampleRepoName, prNumber)
				return err
			}, time.Minute).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v", err))

			mergeResultSha = mergeResult.GetSHA()

			timeout := time.Minute * 2
			interval := time.Second * 1

			Eventually(func() bool {
				pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, appName, userNamespace, mergeResultSha)
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

		It("JVM Build Service is used for rebuilding dependencies", func() {
			timeout := time.Minute * 20
			interval := time.Second * 10
			err = wait.PollImmediate(interval, timeout, func() (done bool, err error) {
				abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(userNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing artifactbuilds: %s\n", err.Error())
					return false, nil
				}
				for _, ab := range abList.Items {
					if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
						return false, nil
					}
				}
				dbList, err := f.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(userNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing dependencybuilds: %s\n", err.Error())
					return false, nil
				}
				for _, db := range dbList.Items {
					if db.Status.State != v1alpha1.DependencyBuildStateComplete {
						return false, nil
					}
				}
				return true, nil
			})
			Expect(err).ShouldNot(HaveOccurred(), "timed out when waiting for some artifactbuilds and dependencybuilds to complete")
		})

		It("release should complete successfully", func() {
			Eventually(func() bool {
				releases, _ := f.AsKubeAdmin.ReleaseController.GetReleases(userNamespace)
				if len(releases.Items) < 1 {
					return false
				}
				for _, r := range releases.Items {
					for k, v := range r.Annotations {
						if k == "pac.test.appstudio.openshift.io/on-target-branch" && v == "["+componentNewBaseBranch+"]" {
							return r.HasStarted() && r.IsDone() && r.Status.Conditions[0].Status == "True"
						}
					}
				}
				return false
			}, 10*time.Minute, 10*time.Second).Should(BeTrue())
		})

	})

})
