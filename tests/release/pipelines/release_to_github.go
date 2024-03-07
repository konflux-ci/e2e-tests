package pipelines

import (
	"encoding/json"
	"fmt"
	"time"
	"strings"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/github"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	releaseapi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	releasecommon "github.com/redhat-appstudio/e2e-tests/tests/release"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	sampServiceAccountName   = "release-service-account"
	sampSourceGitURL         = "https://github.com/redhat-appstudio-qe/devfile-sample-go-basic"
	sampReleaseURL           = "https://github.com/redhat-appstudio-qe/devfile-sample-go-basic/releases/tag/v2.1"
	sampRepoOwner            = "redhat-appstudio-qe"
	sampRepo                 = "devfile-sample-go-basic"
	sampCatalogPathInRepo    = "pipelines/release-to-github/release-to-github.yaml"
)

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for release-to-github pipeline", Label("release-pipelines", "release-to-github"), func() {
	defer GinkgoRecover()

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var sampApplicationName = "samp-app-" + util.GenerateRandomString(4)
	var sampComponentName = "samp-comp-" + util.GenerateRandomString(4)
	var sampReleasePlanName = "samp-rp-" + util.GenerateRandomString(4)
	var sampReleasePlanAdmissionName = "samp-rpa-" + util.GenerateRandomString(4)
	var sampEnterpriseContractPolicyName = "samp-policy-" + util.GenerateRandomString(4)

	var snapshot *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var releasePR, buildPR *tektonv1.PipelineRun
	var gh *github.Github

	AfterEach(framework.ReportFailure(&devFw))

	Describe("Release-to-github happy path", Label("releaseToGithub"), func() {
		var component *appservice.Component
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			managedNamespace = managedFw.UserNamespace

			// Linking the build secret to the pipeline service account in dev namespace.
			err = devFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, releasecommon.HacbsReleaseTestsTokenSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			githubUser := utils.GetEnv("GITHUB_USER","")
			githubToken := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
			gh, err = github.NewGithubClient(githubToken, githubUser)
			Expect(githubToken).ToNot(BeEmpty())

			// Remove the release if the release exists
			if gh.CheckIfReleaseExist(sampRepoOwner, sampRepo, sampReleaseURL) {
				gh.DeleteRelease(sampRepoOwner, sampRepo, sampReleaseURL)
			}

			_, err = managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, releasecommon.RedhatAppstudioUserSecret)
			if errors.IsNotFound(err) {
				githubSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      releasecommon.RedhatAppstudioUserSecret,
						Namespace: managedNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"token": []byte(githubToken),
					},
				}
				_, err = managedFw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, githubSecret)
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(err).ToNot(HaveOccurred())

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(sampApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(sampReleasePlanName, devNamespace, sampApplicationName, managedNamespace, "true")
			Expect(err).NotTo(HaveOccurred())

			createGHReleasePlanAdmission(sampReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, sampApplicationName, sampEnterpriseContractPolicyName, sampCatalogPathInRepo, "false", "", "", "", "")
			component = releasecommon.CreateComponentByCDQ(*devFw, devNamespace, managedNamespace, sampApplicationName, sampComponentName, sampSourceGitURL)
			createGHEnterpriseContractPolicy(sampEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(sampApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(sampEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(sampReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {
			It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				Eventually(func() error {
					buildPR, err = devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(component.Name, sampApplicationName, devNamespace, "")
					if err != nil {
						return err
					}
					if !buildPR.IsDone() {
						return fmt.Errorf("build pipelinerun %s in namespace %s did not finish yet", buildPR.Name, buildPR.Namespace)
					}
					Expect(tekton.HasPipelineRunSucceeded(buildPR)).To(BeTrue(), fmt.Sprintf("build pipelinerun %s/%s did not succeed", buildPR.GetNamespace(), buildPR.GetName()))
					snapshot, err = devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPR.Name, "", devNamespace)
					if err != nil {
						return err
					}
					return nil
				}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for build pipelinerun to be created")
				Expect(devFw.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(component, "", devFw.AsKubeDeveloper.TektonController, &has.RetryOptions{Retries: 3, Always: true})).To(Succeed())
			})
			It("verifies the samp release pipelinerun is running and succeeds", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				managedFw = releasecommon.NewFramework(managedWorkspace)

				releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(func() error {
					releasePR, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
					if err != nil {
						return err
					}
					GinkgoWriter.Println("Release PR: ", releasePR.Name)
					if  !releasePR.IsDone(){
						return fmt.Errorf("release pipelinerun %s in namespace %s did not finished yet", releasePR.Name, releasePR.Namespace)
					}
					return nil
				}, releasecommon.ReleasePipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for release pipelinerun to succeed")
				Expect(tekton.HasPipelineRunSucceeded(releasePR)).To(BeTrue(), fmt.Sprintf("release pipelinerun %s/%s did not succeed", releasePR.GetNamespace(), releasePR.GetName()))
			})

			It("verifies release CR completed and set succeeded.", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				Eventually(func() error {
					releaseCr, err := devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
					if err != nil {
						return err
					}
					GinkgoWriter.Println("Release CR: ", releaseCr.Name)
					if !releaseCr.IsReleased() {
						return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
					}
					return nil
				}, 10 * time.Minute, releasecommon.DefaultInterval).Should(Succeed())
			})

			It("verifies if the Release exists in github repo", func() {
				managedFw = releasecommon.NewFramework(managedWorkspace)
				trReleasePr, err := managedFw.AsKubeAdmin.TektonController.GetTaskRunStatus(managedFw.AsKubeAdmin.CommonController.KubeRest(), releasePR, "create-github-release")
				Expect(err).NotTo(HaveOccurred())
				trReleaseURL := trReleasePr.Status.TaskRunStatusFields.Results[0].Value.StringVal
				releaseURL := strings.Replace(trReleaseURL, "\n", "", -1)
				Expect(gh.CheckIfReleaseExist(sampRepoOwner, sampRepo, releaseURL)).To(BeTrue(), fmt.Sprintf("release %s doesn't exist", releaseURL))
			})
		})
	})
})

func createGHEnterpriseContractPolicy(sampECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
	defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
		Description: "Red Hat's enterprise requirements",
		PublicKey:   "k8s://openshift-pipelines/public-key",
		Sources: []ecp.Source{{
			Name:   "Default",
			Policy: []string{releasecommon.EcPolicyLibPath, releasecommon.EcPolicyReleasePath},
			Data:   []string{releasecommon.EcPolicyDataBundle, releasecommon.EcPolicyDataPath},
		}},
		Configuration: &ecp.EnterpriseContractPolicyConfiguration{
			Exclude: []string{"cve", "step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			Include: []string{"minimal", "slsa3"},
		},
	}

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(sampECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createGHReleasePlanAdmission(sampRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, sampAppName, sampECPName, pathInRepoValue, hotfix, issueId, preGA, productName, productVersion string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"github": map[string]interface{}{
			"githubSecret": releasecommon.RedhatAppstudioUserSecret,
		},
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(sampRPAName, managedNamespace, "", devNamespace, sampECPName, sampServiceAccountName, []string{sampAppName}, true, &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasecommon.RelSvcCatalogURL},
			{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
			{Name: "pathInRepo", Value: pathInRepoValue},
		},
	}, &runtime.RawExtension{
		Raw: data,
	})
	Expect(err).NotTo(HaveOccurred())
}
