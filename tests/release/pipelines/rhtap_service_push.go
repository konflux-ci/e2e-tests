package pipelines

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/clients/github"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	rhtapServiceAccountName  = "release-service-account"
	rhtapCatalogPathInRepo   = "pipelines/rhtap-service-push/rhtap-service-push.yaml"
	rhtapGitSourceURL        = "https://github.com/redhat-appstudio-qe/devfile-sample-python-basic-test2"
	rhtapGitSrcSHA           = "f8ce9d92bfe65df108ac51c3d7429e5df08fe24d"
	rhtapGitSourceRepoName   = "devfile-sample-python-basic-test2"
	rhtapGitSrcDefaultSHA    = "47fc22092005aabebce233a9b6eab994a8152bbd"
	rhtapGitSrcDefaultBranch = "main"
)

var rhtapComponentName = "rhtap-comp-" + util.GenerateRandomString(4)
var gh *github.Github

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for rhtap-service-push pipeline", Label("release-pipelines", "rhtap-service-push"), func() {
	defer GinkgoRecover()
	var pyxisKeyDecoded, pyxisCertDecoded []byte

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var rhtapApplicationName = "rhtap-app-" + util.GenerateRandomString(4)
	var rhtapReleasePlanName = "rhtap-rp-" + util.GenerateRandomString(4)
	var rhtapReleasePlanAdmissionName = "rhtap-rpa-" + util.GenerateRandomString(4)
	var rhtapEnterpriseContractPolicyName = "rhtap-policy-" + util.GenerateRandomString(4)
	//Branch for creating pull request
	var testPRBranchName = fmt.Sprintf("%s-%s", "e2e-pr-branch", util.GenerateRandomString(6))
	var testBaseBranchName = fmt.Sprintf("%s-%s", "e2e-base-branch", util.GenerateRandomString(6))
	var sampleImage = "quay.io/hacbs-release-tests/e2e-rhtap-comp@sha256:c7cd12d46c8edc8b859738e09f3fdfd0f19718dadf8bd4efd30b3eecd1465681"
	var mergeResultSha string
	var sourcePrNum int

	var snapshot *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var pipelineRun *pipeline.PipelineRun

	Describe("Rhtap-service-push happy path", Label("RhtapServicePush"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace

			githubToken := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
			gh, err = github.NewGithubClient(githubToken, "hacbs-release")
			Expect(githubToken).ToNot(BeEmpty())
			Expect(err).ToNot(HaveOccurred())

			sourcePrNum, mergeResultSha = prepareMergedPR(devFw, testBaseBranchName, testPRBranchName)

			keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
			Expect(keyPyxisStage).ToNot(BeEmpty())

			certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
			Expect(certPyxisStage).ToNot(BeEmpty())

			// Creating k8s secret to access Pyxis stage based on base64 decoded of key and cert
			pyxisKeyDecoded, err = base64.StdEncoding.DecodeString(string(keyPyxisStage))
			Expect(err).ToNot(HaveOccurred())

			pyxisCertDecoded, err = base64.StdEncoding.DecodeString(string(certPyxisStage))
			Expect(err).ToNot(HaveOccurred())

			pyxisSecret, err := managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, "pyxis")
			if pyxisSecret == nil || errors.IsNotFound(err) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pyxis",
						Namespace: managedNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"cert": pyxisCertDecoded,
						"key":  pyxisKeyDecoded,
					},
				}

				_, err = managedFw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
				Expect(err).ToNot(HaveOccurred())
			}

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(rhtapApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(rhtapReleasePlanName, devNamespace, rhtapApplicationName, managedNamespace, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createRHTAPReleasePlanAdmission(rhtapReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, rhtapApplicationName, rhtapEnterpriseContractPolicyName, rhtapCatalogPathInRepo)

			createRHTAPEnterpriseContractPolicy(rhtapEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshot, err = releasecommon.CreateSnapshotWithImageSource(*devFw, rhtapComponentName, rhtapApplicationName, devNamespace, sampleImage, rhtapGitSourceURL, rhtapGitSrcSHA, "", "", "", "")
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			// store pipelineRun if there pipelineRun failed 
			if err = managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(pipelineRun.Name, pipelineRun); err != nil {
				GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
			}
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}

			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(rhtapApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(rhtapEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(rhtapReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {
			It("verifies the rhtap release pipelinerun is running and succeeds", func() {
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
			})

			It("verifies release CR completed and set succeeded.", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
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
			It("verifies if the PR in infra-deployments repo is created/updated", func() {
				Eventually(func() error {
					prs, err := gh.ListPullRequests("infra-deployments")
					Expect(err).ShouldNot(HaveOccurred())
					for _, pr := range prs {
						GinkgoWriter.Printf("PR branch: %s", pr.Head.GetRef())
						if strings.Contains(pr.Head.GetRef(), rhtapGitSourceRepoName) {
							contents, err := gh.GetFile("infra-deployments", "components/release/development/kustomization.yaml", rhtapGitSourceRepoName)
							Expect(err).ShouldNot(HaveOccurred())
							content, err := contents.GetContent()
							Expect(err).ShouldNot(HaveOccurred())
							GinkgoWriter.Printf("Content of PR #%d: %s \n", pr.GetNumber(), content)
							if strings.Contains(content, mergeResultSha) {
								GinkgoWriter.Printf("The reference is updated")
								return nil
							}

							body := pr.Body
							GinkgoWriter.Printf("Body of PR #%d: %s \n", pr.GetNumber(), *body)
							prLink := fmt.Sprintf(rhtapGitSourceURL+"/pull/%d", sourcePrNum)
							GinkgoWriter.Printf("The source PR link: %s", prLink)
							if strings.Contains(*body, prLink) {
								GinkgoWriter.Printf("The source PR#%d is added to the PR of infra-deployments", sourcePrNum)
								return nil
							}
						}
					}
					return fmt.Errorf("The reference is not updated and the source PR#%d is not added to the PR of infra-deployments", sourcePrNum)
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())
			})
		})
	})
})

func createRHTAPEnterpriseContractPolicy(rhtapECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
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

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(rhtapECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createRHTAPReleasePlanAdmission(rhtapRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, rhtapAppName, rhtapECPName, pathInRepoValue string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"name":       rhtapComponentName,
					"repository": "registry.stage.redhat.io/rhtap/konflux-release-e2e",
					"tags": []string{"latest", "latest-{{ timestamp }}", "testtag",
						"testtag-{{ timestamp }}", "testtag2", "testtag2-{{ timestamp }}"},
					"source": map[string]interface{}{
						"git": map[string]interface{}{
							"url": rhtapGitSourceURL,
						},
					},
				},
			},
		},
		"pyxis": map[string]interface{}{
			"server": "stage",
			"secret": "pyxis",
		},
		"targetGHRepo":                   "hacbs-release/infra-deployments",
		"githubAppID":                    "932323",
		"githubAppInstallationID":        "52284535",
		"infra-deployment-update-script": "sed -i -e 's|\\(https://github.com/hacbs-release/release-service/config/default?ref=\\)\\(.*\\)|\\1{{ revision }}|' -e 's/\\(newTag: \\).*/\\1{{ revision }}/' components/release/development/kustomization.yaml",
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(rhtapRPAName, managedNamespace, "", devNamespace, rhtapECPName, rhtapServiceAccountName, []string{rhtapAppName}, true, &tektonutils.PipelineRef{
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

// prepareMergedPR function is to prepare a merged PR in source repo for testing update-infra-deployments task
func prepareMergedPR(devFw *framework.Framework, testBaseBranchName, testPRBranchName string) (int, string) {
	//Create the ref, add the file,  create the PR and merge the PR
	err = devFw.AsKubeAdmin.CommonController.Github.CreateRef(rhtapGitSourceRepoName, rhtapGitSrcDefaultBranch, rhtapGitSrcDefaultSHA, testBaseBranchName)
	Expect(err).ShouldNot(HaveOccurred())

	err = devFw.AsKubeAdmin.CommonController.Github.CreateRef(rhtapGitSourceRepoName, rhtapGitSrcDefaultBranch, rhtapGitSrcDefaultSHA, testPRBranchName)
	Expect(err).ShouldNot(HaveOccurred())

	fileToCreatePath := fmt.Sprintf("%s/sample-file.txt", "testdir")
	createdFileSha, err := devFw.AsKubeAdmin.CommonController.Github.CreateFile(rhtapGitSourceRepoName, fileToCreatePath, fmt.Sprintf("sample test file inside %s", "testdir"), testPRBranchName)
	Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file: %s", fileToCreatePath))

	pr, err := devFw.AsKubeAdmin.CommonController.Github.CreatePullRequest(rhtapGitSourceRepoName, "sample pr title", "sample pr body", testPRBranchName, testBaseBranchName)
	Expect(err).ShouldNot(HaveOccurred())
	GinkgoWriter.Printf("PR #%d got created with sha %s\n", pr.GetNumber(), createdFileSha.GetSHA())

	var mergeResultSha string
	Eventually(func() error {
		mergeResult, err := devFw.AsKubeAdmin.CommonController.Github.MergePullRequest(rhtapGitSourceRepoName, pr.GetNumber())
		if err == nil {
			mergeResultSha := mergeResult.GetSHA()
			GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, pr.GetNumber())
			return nil
		}
		return fmt.Errorf("PR #%d merge failed", pr.GetNumber())
	}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())

	return pr.GetNumber(), mergeResultSha
}
