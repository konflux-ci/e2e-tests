package pipelines

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	ecp "github.com/conforma/crds/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	multiarchCatalogPathInRepo = "pipelines/managed/rh-advisories/rh-advisories.yaml"
	multiarchGitSourceURL      = "https://github.com/redhat-appstudio-qe/multi-platform-test-prod"
	multiarchGitSrcSHA         = "fd4b6c28329ab3df77e7ad7beebac1829836561d"
)

var multiarchComponentName = "multiarch-comp-" + util.GenerateRandomString(4)

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for multi arch with rh-advisories pipeline", ginkgo.Pending, ginkgo.Label("release-pipelines", "rh-advisories", "multiarch-advisories"), func() {
	defer ginkgo.GinkgoRecover()

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var multiarchApplicationName = "multiarch-app-" + util.GenerateRandomString(4)
	var multiarchReleasePlanName = "multiarch-rp-" + util.GenerateRandomString(4)
	var multiarchReleasePlanAdmissionName = "multiarch-rpa-" + util.GenerateRandomString(4)
	var multiarchEnterpriseContractPolicyName = "multiarch-policy-" + util.GenerateRandomString(4)
	var sampleImage = "quay.io/hacbs-release-tests/e2e-multi-platform-test@sha256:23ce99c70f86f879c67a82ef9aa088c7e9a52dc09630c5913587161bda6259e2"

	var snapshotPush *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var releasePR *pipeline.PipelineRun
	var pipelineRun *pipeline.PipelineRun

	ginkgo.Describe("Multi arch test happy path", ginkgo.Label("multiArchAdvisories"), func() {
		ginkgo.BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace

			pyxisFieldEnvMap := map[string]string{
				"key":  constants.PYXIS_STAGE_KEY_ENV,
				"cert": constants.PYXIS_STAGE_CERT_ENV,
			}
			releasecommon.CreateOpaqueSecret(managedFw, managedNamespace, "pyxis", pyxisFieldEnvMap)

			atlasFieldEnvMap := map[string]string{
				"sso_account": constants.ATLAS_STAGE_ACCOUNT_ENV,
				"sso_token":   constants.ATLAS_STAGE_TOKEN_ENV,
			}
			releasecommon.CreateOpaqueSecret(managedFw, managedNamespace, "atlas-staging-sso-secret", atlasFieldEnvMap)

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, releasecommon.ReleasePipelineServiceAccountDefault, true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(multiarchApplicationName, devNamespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			createMultiArchReleasePlan(multiarchReleasePlanName, *devFw, devNamespace, multiarchApplicationName, managedNamespace, "true")

			createMultiArchReleasePlanAdmission(multiarchReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, multiarchApplicationName, multiarchEnterpriseContractPolicyName, multiarchCatalogPathInRepo)

			createMultiArchEnterpriseContractPolicy(multiarchEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(devFw.AsKubeAdmin, multiarchComponentName, multiarchApplicationName, devNamespace, sampleImage, multiarchGitSourceURL, multiarchGitSrcSHA, "", "", "", "")
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.AfterAll(func() {
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

			gomega.Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(multiarchApplicationName, devNamespace, false)).NotTo(gomega.HaveOccurred())
			gomega.Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(multiarchEnterpriseContractPolicyName, managedNamespace, false)).NotTo(gomega.HaveOccurred())
			gomega.Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(multiarchReleasePlanAdmissionName, managedNamespace, false)).NotTo(gomega.HaveOccurred())
		})

		var _ = ginkgo.Describe("Post-release verification", func() {

			ginkgo.It("verifies the release CR is created", func() {
				gomega.Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return err
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(gomega.Succeed(), "timed out when trying to get release CR for snapshot %s/%s", devNamespace, snapshotPush.Name)
			})

			ginkgo.It("verifies the multiarch release pipelinerun is running and succeeds", func() {
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
			})

			ginkgo.It("verifies release CR completed and set succeeded.", func() {
				gomega.Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return err
					}
					err = releasecommon.CheckReleaseStatus(releaseCR)
					return err
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(gomega.Succeed())
			})

			ginkgo.It("verifies if the repository URL is valid", func() {
				releasePR, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				advisoryURL := releasePR.Status.Results[0].Value.StringVal
				pattern := `https://access\.stage\.redhat\.com/errata/(RHBA|RHSA|RHEA)-\d{4}:\d+`
				re, err := regexp.Compile(pattern)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(re.MatchString(advisoryURL)).To(gomega.BeTrue(), fmt.Sprintf("Advisory_url %s is not valid", advisoryURL))
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
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

}

func createMultiArchReleasePlan(multiarchReleasePlanName string, devFw framework.Framework, devNamespace, multiarchAppName, managedNamespace string, autoRelease string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"releaseNotes": map[string]interface{}{
			"description": "releaseNotes description",
			"references":  []string{"https://server.com/ref1", "http://server2.com/ref2"},
			"solution":    "some solution",
			"synopsis":    "test synopsis",
			"topic":       "test topic",
		},
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(multiarchReleasePlanName, devNamespace, multiarchAppName,
		managedNamespace, autoRelease, &runtime.RawExtension{
			Raw: data,
		}, nil, nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func createMultiArchReleasePlanAdmission(multiarchRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, multiarchAppName, multiarchECPName, pathInRepoValue string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"name":       multiarchComponentName,
					"repository": "registry.stage.redhat.io/rhtap/konflux-release-e2e",
					"tags": []string{"latest", "latest-{{ timestamp }}", "testtag",
						"testtag-{{ timestamp }}", "testtag2", "testtag2-{{ timestamp }}"},
				},
			},
		},
		"pyxis": map[string]interface{}{
			"server": "stage",
			"secret": "pyxis",
		},
		"atlas": map[string]interface{}{
			"server": "stage",
		},
		"releaseNotes": map[string]interface{}{
			"cpe":             "cpe:/a:example.com",
			"product_id":      []int{555},
			"product_name":    "test product",
			"product_stream":  "rhtas-tp1",
			"product_version": "v1.0",
			"type":            "RHBA",
		},
		"sign": map[string]interface{}{
			"configMapName":    "hacbs-signing-pipeline-config-redhatbeta2",
			"cosignSecretName": "test-cosign-secret",
		},
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(multiarchRPAName, managedNamespace, "", devNamespace, multiarchECPName, releasecommon.ReleasePipelineServiceAccountDefault, []string{multiarchAppName}, false, &tektonutils.PipelineRef{
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
