package pipelines

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	"github.com/konflux-ci/e2e-tests/pkg/clients/github"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	"github.com/devfile/library/v2/pkg/util"
	"knative.dev/pkg/apis"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	sampServiceAccountName = "release-service-account"
	sampSourceGitURL       = "https://github.com/redhat-appstudio-qe/devfile-sample-go-basic"
	sampGitSrcSHA          = "6b56d05ac8abb4c24d153e9689209a1018402aad"
	sampRepoOwner          = "redhat-appstudio-qe"
	sampRepo               = "devfile-sample-go-basic"
	sampCatalogPathInRepo  = "pipelines/release-to-github/release-to-github.yaml"
)

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for release-to-github pipeline", Pending, Label("release-pipelines", "release-to-github"), func() {
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
	var sampleImage = "quay.io/hacbs-release-tests/e2e-rel-to-github-comp@sha256:3a354e86ff26bbd4870ce8d62e180159094ebc2761db9572976b8f67c53add16"

	var snapshot *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var releasePR *tektonv1.PipelineRun
	var gh *github.Github
	var sampReleaseURL string
	var pipelineRun *pipeline.PipelineRun

	Describe("Release-to-github happy path", Label("releaseToGithub"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace

			githubUser := utils.GetEnv("GITHUB_USER", "redhat-appstudio-qe-bot")
			githubToken := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
			gh, err = github.NewGithubClient(githubToken, githubUser)
			Expect(githubToken).ToNot(BeEmpty())
			Expect(err).ToNot(HaveOccurred())

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
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(err).ToNot(HaveOccurred())

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioQESecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(sampApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(sampReleasePlanName, devNamespace, sampApplicationName, managedNamespace, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createGHReleasePlanAdmission(sampReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, sampApplicationName, sampEnterpriseContractPolicyName, sampCatalogPathInRepo, "false", "", "", "", "")

			createGHEnterpriseContractPolicy(sampEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshot, err = releasecommon.CreateSnapshotWithImageSource(*devFw, sampComponentName, sampApplicationName, devNamespace, sampleImage, sampSourceGitURL, sampGitSrcSHA, "", "", "", "")
                        Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if gh.CheckIfReleaseExist(sampRepoOwner, sampRepo, sampReleaseURL) {
				gh.DeleteRelease(sampRepoOwner, sampRepo, sampReleaseURL)
			}

			// store pipelineRun and Release CR
			if err = managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(pipelineRun.Name, pipelineRun); err != nil {
				GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
			}
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}

			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(sampApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(sampEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(sampReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {

			It("verifies release pipelinerun is running and succeeds", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
					if err != nil {
						return err
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())

				Eventually(func() error {
					pipelineRun, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
					if err != nil {
						return fmt.Errorf("PipelineRun has not been created yet for release %s/%s", releaseCR.GetNamespace(), releaseCR.GetName())
					}
					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pipelineRun.Name, condition.Reason)
					}

					if !pipelineRun.IsDone() {
						return fmt.Errorf("PipelineRun %s has still not finished yet", pipelineRun.Name)
					}

					if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
						return nil
					} else {
						prLogs := ""
						if prLogs, err = tekton.GetFailedPipelineRunLogs(managedFw.AsKubeAdmin.ReleaseController.KubeRest(), managedFw.AsKubeAdmin.ReleaseController.KubeInterface(), pipelineRun); err != nil {
							GinkgoWriter.Printf("failed to get PLR logs: %+v", err)
							Expect(err).ShouldNot(HaveOccurred())
							return nil
						}
						GinkgoWriter.Printf("logs: %s", prLogs)
						Expect(prLogs).To(Equal(""), fmt.Sprintf("PipelineRun %s failed", pipelineRun.Name))
						return nil
					}
				}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the release PipelineRun to be finished for the release %s/%s", releaseCR.GetName(), releaseCR.GetNamespace()))

				releasePR, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).NotTo(HaveOccurred())
			})

			It("verifies release CR completed and set succeeded.", func() {
				Eventually(func() error {
					releaseCR, err := devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
					if err != nil {
						return err
					}
					GinkgoWriter.Println("releaseCR: %s", releaseCR.Name)
					conditions := releaseCR.Status.Conditions
					GinkgoWriter.Println("len of conditions: %d", len(conditions))
					if len(conditions) > 0 {
						for _, c := range conditions {
							GinkgoWriter.Println("type of c: %s", c.Type)
							if c.Type == "Released" {
								GinkgoWriter.Println("status of c: %s", c.Status)
								if c.Status == "True" {
									GinkgoWriter.Println("Release CR is released")
									return nil
								} else if c.Status == "False" && c.Reason == "Progressing" {
									return fmt.Errorf("release %s/%s is in progressing", releaseCR.GetNamespace(), releaseCR.GetName())
								} else {
									GinkgoWriter.Println("Release CR failed/skipped")
									Expect(string(c.Status)).To(Equal("True"), fmt.Sprintf("Release %s failed/skipped", releaseCR.Name))
									return nil
								}
							}
						}
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())
			})

			It("verifies if the Release exists in github repo", func() {
				trReleasePr, err := managedFw.AsKubeAdmin.TektonController.GetTaskRunStatus(managedFw.AsKubeAdmin.CommonController.KubeRest(), releasePR, "create-github-release")
				Expect(err).NotTo(HaveOccurred())
				trReleaseURL := trReleasePr.Status.TaskRunStatusFields.Results[0].Value.StringVal
				releaseURL := strings.Replace(trReleaseURL, "\n", "", -1)
				Expect(gh.CheckIfReleaseExist(sampRepoOwner, sampRepo, releaseURL)).To(BeTrue(), fmt.Sprintf("release %s doesn't exist", releaseURL))
				sampReleaseURL = releaseURL
				if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
					GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
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
	Expect(err).NotTo(HaveOccurred())

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
