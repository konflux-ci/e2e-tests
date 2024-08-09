package pipelines

import (
	"encoding/json"
	"fmt"
	"time"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("Release service tenant pipeline", Label("release-pipelines", "tenant"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
	var err error
	var compName string
	var devNamespace string
	var releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"

	var component *appservice.Component
	var releaseCR *releaseApi.Release
	var baseBranchName, pacBranchName string

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace("tenant-dev"))
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		// Linking the build secret to the pipeline service account in dev namespace.
		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, releasecommon.HacbsReleaseTestsTokenSecret, constants.DefaultPipelineServiceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(releasecommon.ApplicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		component, baseBranchName, pacBranchName = releasecommon.CreateComponent(*fw, devNamespace, releasecommon.ApplicationNameDefault, releasecommon.ComponentName, releasecommon.DcMetroMapGitSourceURL, releasecommon.DcMetroMapGitRevision, ".", "Dockerfile", constants.DefaultDockerBuildPipelineBundle)

		data, err := json.Marshal(map[string]interface{}{
			"mapping": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"component":  compName,
						"repository": releasedImagePushRepo,
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		tenantPipeline := &tektonutils.ParameterizedPipeline{}
		tenantPipeline.ServiceAccountName = constants.DefaultPipelineServiceAccount
		tenantPipeline.Timeouts = tektonv1.TimeoutFields{
			Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
		}

		tenantPipeline.PipelineRef = tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: "https://github.com/jinqi7/integration-examples"},
				{Name: "revision", Value: "main"},
				{Name: "pathInRepo", Value: "pipelines/integration_pipeline_pass.yaml"},
			},
		}

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, "", "", &runtime.RawExtension{
			Raw: data,
		}, tenantPipeline)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasecommon.ReleasePvcName, devNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
		// Delete new branches created by PaC and a testing branch used as a component's base branch
		err = devFw.AsKubeAdmin.CommonController.Github.DeleteRef(releasecommon.DcMetroMapGitRepoName, pacBranchName)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
		}
		err = devFw.AsKubeAdmin.CommonController.Github.DeleteRef(releasecommon.DcMetroMapGitRepoName, baseBranchName)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
		}

		// Delete created webhook from GitHub
		Expect(build.CleanupWebhooks(devFw, releasecommon.DcMetroMapGitRepoName)).ShouldNot(HaveOccurred())
	})

	var _ = Describe("Post-release verification", func() {
		It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "",
				fw.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, nil)).To(Succeed())
		})

		It("verifies that a Release CR should have been created in the dev namespace", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				return err
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
		})

		It("verifies that Tenant PipelineRun is triggered", func() {
			Expect(fw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, devNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
		})

		It("verifies that a Release is marked as succeeded.", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				if err != nil {
					return err
				}
				if !releaseCR.IsReleased() {
					return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
				}
				return nil
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
		})
	})
})
