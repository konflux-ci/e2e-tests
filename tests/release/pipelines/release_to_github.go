package pipelines

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ecp "github.com/conforma/crds/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/clients/github"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	sampSourceGitURL      = "https://github.com/redhat-appstudio-qe/devfile-sample-go-basic"
	sampGitSrcSHA         = "6b56d05ac8abb4c24d153e9689209a1018402aad"
	sampRepoOwner         = "redhat-appstudio-qe"
	sampRepo              = "devfile-sample-go-basic"
	sampCatalogPathInRepo = "pipelines/managed/release-to-github/release-to-github.yaml"
)

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for release-to-github pipeline", ginkgo.Pending, ginkgo.Label("release-pipelines", "release-to-github"), func() {
	defer ginkgo.GinkgoRecover()

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
	var sampleImage = "quay.io/hacbs-release-tests/e2e-rel-to-github-comp@sha256:3a354e86ff26bbd4870ce8d62e180159094ebc2761db9572976b8f67c53add16"

	var snapshot *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var releasePR *pipeline.PipelineRun
	var gh *github.Github
	var sampReleaseURL string
	var pipelineRun *pipeline.PipelineRun

	ginkgo.Describe("Release-to-github happy path", ginkgo.Label("releaseToGithub"), func() {
		ginkgo.BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace

			githubUser := utils.GetEnv("GITHUB_USER", "redhat-appstudio-qe-bot")
			githubToken := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
			gh, err = github.NewGithubClient(githubToken, githubUser)
			gomega.Expect(githubToken).ToNot(gomega.BeEmpty())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			_, err = managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, releasecommon.RedhatAppstudioQESecret)
			if errors.IsNotFound(err) {
				githubSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      releasecommon.RedhatAppstudioQESecret,
						Namespace: managedNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"token": []byte(githubToken),
					},
				}
				_, err = managedFw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, githubSecret)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			}
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioQESecret, releasecommon.ReleasePipelineServiceAccountDefault, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, releasecommon.ReleasePipelineServiceAccountDefault, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(sampApplicationName, devNamespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(sampReleasePlanName, devNamespace, sampApplicationName, managedNamespace, "true", nil, nil, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			createGHReleasePlanAdmission(sampReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, sampApplicationName, sampEnterpriseContractPolicyName, sampCatalogPathInRepo, "false", "", "", "", "")

			createGHEnterpriseContractPolicy(sampEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshot, err = releasecommon.CreateSnapshotWithImageSource(devFw.AsKubeAdmin, sampComponentName, sampApplicationName, devNamespace, sampleImage, sampSourceGitURL, sampGitSrcSHA, "", "", "", "")
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.AfterAll(func() {
			if gh.CheckIfReleaseExist(sampRepoOwner, sampRepo, sampReleaseURL) {
				gh.DeleteRelease(sampRepoOwner, sampRepo, sampReleaseURL)
			}

			// store pipelineRun and Release CR
			if err = managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(pipelineRun.Name, pipelineRun); err != nil {
				ginkgo.GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
			}
			if err = managedFw.AsKubeDeveloper.TektonController.StoreTaskRunsForPipelineRun(managedFw.AsKubeDeveloper.CommonController.KubeRest(), pipelineRun); err != nil {
				ginkgo.GinkgoWriter.Printf("failed to store TaskRuns for PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
			}
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				ginkgo.GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}

			gomega.Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(sampApplicationName, devNamespace, false)).NotTo(gomega.HaveOccurred())
			gomega.Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(sampEnterpriseContractPolicyName, managedNamespace, false)).NotTo(gomega.HaveOccurred())
			gomega.Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(sampReleasePlanAdmissionName, managedNamespace, false)).NotTo(gomega.HaveOccurred())
		})

		var _ = ginkgo.Describe("Post-release verification", func() {

			ginkgo.It("verifies if release CR is created", func() {
				gomega.Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
					if err != nil {
						return err
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(gomega.Succeed(), "timed out when trying to get release CR for snapshot %s/%s", devNamespace, snapshot.Name)
			})

			ginkgo.It("verifies the release pipelinerun is running and succeeds", func() {
				gomega.Eventually(func() error {
					pipelineRun, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
					if err != nil {
						return fmt.Errorf("PipelineRun has not been created yet for release %s/%s", releaseCR.GetNamespace(), releaseCR.GetName())
					}
					for _, condition := range pipelineRun.Status.Conditions {
						ginkgo.GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pipelineRun.Name, condition.Reason)
					}

					if !pipelineRun.IsDone() {
						return fmt.Errorf("PipelineRun %s has still not finished yet", pipelineRun.Name)
					}

					if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
						return nil
					} else {
						prLogs := ""
						if prLogs, err = tekton.GetFailedPipelineRunLogs(managedFw.AsKubeAdmin.ReleaseController.KubeRest(), managedFw.AsKubeAdmin.ReleaseController.KubeInterface(), pipelineRun); err != nil {
							ginkgo.GinkgoWriter.Printf("failed to get PLR logs: %+v", err)
							gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
							return nil
						}
						ginkgo.GinkgoWriter.Printf("logs: %s", prLogs)
						gomega.Expect(prLogs).To(gomega.Equal(""), fmt.Sprintf("PipelineRun %s failed", pipelineRun.Name))
						return nil
					}
				}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the release PipelineRun to be finished for the release %s/%s", releaseCR.GetName(), releaseCR.GetNamespace()))

				releasePR, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("verifies release CR completed and set succeeded.", func() {
				gomega.Eventually(func() error {
					releaseCR, err := devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
					if err != nil {
						return err
					}
					err = releasecommon.CheckReleaseStatus(releaseCR)
					return err
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(gomega.Succeed())
			})

			ginkgo.It("verifies if the Release exists in github repo", func() {
				trReleasePr, err := managedFw.AsKubeAdmin.TektonController.GetTaskRunStatus(managedFw.AsKubeAdmin.CommonController.KubeRest(), releasePR, "create-github-release")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				trReleaseURL := trReleasePr.Status.Results[0].Value.StringVal
				releaseURL := strings.ReplaceAll(trReleaseURL, "\n", "")
				gomega.Expect(gh.CheckIfReleaseExist(sampRepoOwner, sampRepo, releaseURL)).To(gomega.BeTrue(), fmt.Sprintf("release %s doesn't exist", releaseURL))
				sampReleaseURL = releaseURL
				if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
					ginkgo.GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
				}
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
			Exclude: []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			Include: []string{"@slsa3"},
		},
	}

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(sampECPName, managedNamespace, defaultEcPolicySpec)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

}

func createGHReleasePlanAdmission(sampRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, sampAppName, sampECPName, pathInRepoValue, hotfix, issueId, preGA, productName, productVersion string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"github": map[string]interface{}{
			"githubSecret": releasecommon.RedhatAppstudioQESecret,
		},
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(sampRPAName, managedNamespace, "", devNamespace, sampECPName, releasecommon.ReleasePipelineServiceAccountDefault, []string{sampAppName}, false, &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasecommon.RelSvcCatalogURL},
			{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
			{Name: "pathInRepo", Value: pathInRepoValue},
		},
	}, &runtime.RawExtension{
		Raw: data,
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
