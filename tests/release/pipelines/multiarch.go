package pipelines

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	//appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

const (
	multiarchServiceAccountName  = "release-service-account"
	multiarchCatalogPathInRepo   = "pipelines/rh-push-to-registry-redhat-io/rh-push-to-registry-redhat-io.yaml"
	multiarchGitSourceURL        = "https://github.com/redhat-appstudio-qe/multi-platform-test-prod"
	multiarchGitSourceRepoName   = "multi-platform-test-prod"
	multiarchGitSrcDefaultSHA    = "d85fb9fc9601c6efe161c1272d9be396ef6eea12"
	multiarchGitSrcDefaultBranch = "main"
)

var multiarchComponentName = "e2e-multi-platform-test"

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for multi arch test for release pipeline", Label("release-pipelines", "multiarch"), func() {
	defer GinkgoRecover()
	var pyxisKeyDecoded, pyxisCertDecoded []byte

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var multiarchApplicationName = "e2e-multi-platform-test-prod"
	var multiarchReleasePlanName = "e2e-multiarch-rp-uibu"
	var multiarchReleasePlanAdmissionName = "e2e-multiarch-rpa-wcjo"
	var multiarchEnterpriseContractPolicyName = "e2e-multiarch-policy-tahd"
	//Branch for creating pull request
	var testPRBranchName = fmt.Sprintf("%s-%s", "multiarch-pr-branch", util.GenerateRandomString(6))
	var testBaseBranchName = fmt.Sprintf("%s-%s", "multiarch-base-branch", util.GenerateRandomString(6))
	var sourcePrNum int

	var snapshot *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var buildPR *tektonv1.PipelineRun

	AfterEach(framework.ReportFailure(&devFw))

	Describe("Multi-arch happy path", Label("MultiArch"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace


			keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
			Expect(keyPyxisStage).ToNot(BeEmpty())

			certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
			Expect(certPyxisStage).ToNot(BeEmpty())

			// Creating k8s secret to access Pyxis stage based on base64 decoded of key and cert
			pyxisKeyDecoded, err = base64.StdEncoding.DecodeString(string(keyPyxisStage))
			Expect(err).ToNot(HaveOccurred())

			pyxisCertDecoded, err = base64.StdEncoding.DecodeString(string(certPyxisStage))
			Expect(err).ToNot(HaveOccurred())

			_, err = managedFw.AsKubeAdmin.CommonController.GetSecret(managedNamespace, releasecommon.RedhatAppstudioQESecret)
			if errors.IsNotFound(err) {
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

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.GetApplication(multiarchApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			if errors.IsNotFound(err) {
				_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(multiarchApplicationName, devNamespace)
				Expect(err).NotTo(HaveOccurred())
			}

			_, err = devFw.AsKubeDeveloper.HasController.GetComponent(multiarchComponentName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			if errors.IsNotFound(err) {
				testComponent = releasecommon.CreateComponent(*devFw, devNamespace, multiarchApplicationName, multiarchComponentName, releasecommon.AdditionalGitSourceComponentUrl, "", ".", constants.DockerFilePath, constants.DefaultDockerBuildPipelineBundle)
			}

			_, err = devFw.AsKubeDeveloper.ReleaseController.GetReleasePlan(multiarchReleasePlanName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			if errors.IsNotFound(err) {
				_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(multiarchReleasePlanName, devNamespace, multiarchApplicationName, managedNamespace, "true", nil, nil)
				Expect(err).NotTo(HaveOccurred())
			}

			_, err = devFw.AsKubeDeveloper.ReleaseController.GetReleasePlanAdmission(multiarchReleasePlanAdmissionName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			if errors.IsNotFound(err) {
				createMultiArchReleasePlanAdmission(multiarchReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, multiarchApplicationName, multiarchEnterpriseContractPolicyName, multiarchCatalogPathInRepo)
				createMultiArchEnterpriseContractPolicy(multiarchEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
			}

			sourcePrNum = preparePR(devFw, testBaseBranchName, testPRBranchName)

			GinkgoWriter.Printf("PR #%d got created and merged\n", sourcePrNum)

		})

		var _ = Describe("Post-release verification", func() {
			It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				managedFw = releasecommon.NewFramework(managedWorkspace)
				// Create a ticker that ticks every 3 minutes
				ticker := time.NewTicker(3 * time.Minute)
				// Schedule the stop of the ticker after 15 minutes
				time.AfterFunc(15*time.Minute, func() {
					ticker.Stop()
					fmt.Println("Stopped executing every 3 minutes.")
				})
				// Run a goroutine to handle the ticker ticks
				go func() {
					for range ticker.C {
						devFw = releasecommon.NewFramework(devWorkspace)
						managedFw = releasecommon.NewFramework(managedWorkspace)
					}
				}()
				Eventually(func() error {
					buildPR, err = devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(multiarchComponentName, multiarchApplicationName, devNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("Build PipelineRun has not been created yet for the component %s/%s\n", devNamespace, multiarchComponentName)
						return err
					}
					GinkgoWriter.Printf("PipelineRun %s reason: %s\n", buildPR.Name, buildPR.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason())
					if !buildPR.IsDone() {
						return fmt.Errorf("build pipelinerun %s in namespace %s did not finish yet", buildPR.Name, buildPR.Namespace)
					}
					if buildPR.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
						snapshot, err = devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPR.Name, "", devNamespace)
						if err != nil {
							return err
						}
						return nil
					} else {
						return fmt.Errorf(tekton.GetFailedPipelineRunLogs(devFw.AsKubeDeveloper.HasController.KubeRest(), devFw.AsKubeDeveloper.HasController.KubeInterface(), buildPR))
					}
				}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to be finished for the component %s/%s", devNamespace, multiarchComponentName))
			})

			It("verifies the multiarch release pipelinerun is running and succeeds", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				managedFw = releasecommon.NewFramework(managedWorkspace)

				releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(managedFw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
			})

			It("verifies release CR completed and set succeeded.", func() {
				devFw = releasecommon.NewFramework(devWorkspace)
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
					if err != nil {
						return err
					}
					GinkgoWriter.Println("Release CR: ", releaseCR.Name)
					if !releaseCR.IsReleased() {
						return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())
			})
		})
	})
})

func createMultiArchEnterpriseContractPolicy(multiarchECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
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

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(multiarchECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createMultiArchReleasePlanAdmission(multiarchRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, multiarchAppName, multiarchECPName, pathInRepoValue string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"name": multiarchComponentName,
					"repository": "quay.io/redhat-pending/rhtap----konflux-release-e2e",
					"tags": []string{"latest", "latest-{{ timestamp }}", "testtag",
						"testtag-{{ timestamp }}", "testtag2", "testtag2-{{ timestamp }}"},
					"source": map[string]interface{}{
						"git": map[string]interface{}{
							"url": multiarchGitSourceURL,
						},
					},
				},
			},
		},
		"pyxis": map[string]interface{}{
			"server": "stage",
			"secret": "pyxis",
		},
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(multiarchRPAName, managedNamespace, "", devNamespace, multiarchECPName, multiarchServiceAccountName, []string{multiarchAppName}, true, &tektonutils.PipelineRef{
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

// preparePR function is to prepare a merged PR for triggerng a push event 
func preparePR(devFw *framework.Framework, testBaseBranchName, testPRBranchName string) (int) {
	//Create the ref, add the file,  create the PR and merge the PR
	err = devFw.AsKubeAdmin.CommonController.Github.CreateRef(multiarchGitSourceRepoName, multiarchGitSrcDefaultBranch, multiarchGitSrcDefaultSHA, testBaseBranchName)
	Expect(err).ShouldNot(HaveOccurred())

	err = devFw.AsKubeAdmin.CommonController.Github.CreateRef(multiarchGitSourceRepoName, multiarchGitSrcDefaultBranch, multiarchGitSrcDefaultSHA, testPRBranchName)
	Expect(err).ShouldNot(HaveOccurred())

	// Update the pac configuration for "push event 
	fileName := "multi-platform-test-prod-push.yaml"
	fileResponse, err := devFw.AsKubeAdmin.CommonController.Github.GetFile(multiarchGitSourceRepoName, ".tekton/" + fileName, testPRBranchName)
	Expect(err).ShouldNot(HaveOccurred())

	fileContent, err := fileResponse.GetContent()
	Expect(err).ShouldNot(HaveOccurred())

	fileContent = strings.ReplaceAll(fileContent, "[main]", "[" + testBaseBranchName  + "]")
	repoContentResponse, err := devFw.AsKubeAdmin.CommonController.Github.UpdateFile(multiarchGitSourceRepoName, ".tekton/" + fileName, fileContent, testPRBranchName, *fileResponse.SHA)
	Expect(err).ShouldNot(HaveOccurred())

	pr, err := devFw.AsKubeAdmin.CommonController.Github.CreatePullRequest(multiarchGitSourceRepoName, "update pac configuration title", "update pac configuration body", testPRBranchName, testBaseBranchName)
	Expect(err).ShouldNot(HaveOccurred())
	GinkgoWriter.Printf("PR #%d got created with sha %s\n", pr.GetNumber(), *repoContentResponse.Commit.SHA)

	var mergeResultSha string
	Eventually(func() error {
		mergeResult, err := devFw.AsKubeAdmin.CommonController.Github.MergePullRequest(multiarchGitSourceRepoName, pr.GetNumber())
		if err == nil {
			mergeResultSha := mergeResult.GetSHA()
			GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, pr.GetNumber())
			return nil
		}
		return fmt.Errorf("PR merge failed")
	}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())

	GinkgoWriter.Printf("PR #%d got merged with sha %s\n", pr.GetNumber(), mergeResultSha)

	return pr.GetNumber()
}

