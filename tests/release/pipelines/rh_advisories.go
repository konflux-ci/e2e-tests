package pipelines

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	advsServiceAccountName = "release-service-account"
	advsCatalogPathInRepo  = "pipelines/managed/rh-advisories/rh-advisories.yaml"
	advsGitSourceURL       = "https://github.com/redhat-appstudio-qe/devfile-sample-python-basic-release"
	advsGitSrcSHA          = "33ff89edf85fb01a37d3d652d317080223069fc7"
)

var advsComponentName = "advs-comp-" + util.GenerateRandomString(4)

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for rh-advisories pipeline", Label("release-pipelines", "rh-advisories"), func() {
	defer GinkgoRecover()

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var advsApplicationName = "advs-app-" + util.GenerateRandomString(4)
	var advsReleasePlanName = "advs-rp-" + util.GenerateRandomString(4)
	var advsReleasePlanAdmissionName = "advs-rpa-" + util.GenerateRandomString(4)
	var advsEnterpriseContractPolicyName = "advs-policy-" + util.GenerateRandomString(4)
	var sampleImage = "quay.io/hacbs-release-tests/e2e-rhio-comp@sha256:bf2fb2c7d63c924ff9170c27f0f15558f6a59bdfb5ad9613eb61d3e4bc1cff0a"

	var snapshotPush *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var releasePR *tektonv1.PipelineRun
	var pipelineRun *pipeline.PipelineRun

	Describe("Rh-advisories happy path", Label("rhAdvisories"), func() {
		BeforeAll(func() {
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

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(advsApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			createADVSReleasePlan(advsReleasePlanName, *devFw, devNamespace, advsApplicationName, managedNamespace, "true")

			createADVSReleasePlanAdmission(advsReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, advsApplicationName, advsEnterpriseContractPolicyName, advsCatalogPathInRepo)

			createADVSEnterpriseContractPolicy(advsEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(*devFw, advsComponentName, advsApplicationName, devNamespace, sampleImage, advsGitSourceURL, advsGitSrcSHA, "", "", "", "")
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			// store pipelineRun and Release CR
			if err = managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(pipelineRun.Name, pipelineRun); err != nil {
				GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
			}
			if err = managedFw.AsKubeDeveloper.TektonController.StoreTaskRunsForPipelineRun(managedFw.AsKubeDeveloper.CommonController.KubeRest(), pipelineRun); err != nil {
				GinkgoWriter.Printf("failed to store TaskRuns for PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
			}
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}

			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(advsApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(advsEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(advsReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {

			It("verifies the advs release pipelinerun is running and succeeds", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
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
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return err
					}
					err = releasecommon.CheckReleaseStatus(releaseCR)
					return err
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed())
			})

			It("verifies if the repository URL is valid", func() {
				releasePR, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).NotTo(HaveOccurred())
				advisoryURL := releasePR.Status.PipelineRunStatusFields.Results[0].Value.StringVal
				pattern := `https?://[^/\s]+/[^/\s]+/[^/\s]+/+\-\/blob\/main\/data\/advisories\/[^\/]+\/[^\/]+\/[^\/]+\/advisory\.yaml`
				re, err := regexp.Compile(pattern)
				Expect(err).NotTo(HaveOccurred())
				Expect(re.MatchString(advisoryURL)).To(BeTrue(), fmt.Sprintf("Advisory_url %s is not valid", advisoryURL))
			})
		})
	})
})

func createADVSEnterpriseContractPolicy(advsECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
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

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(advsECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createADVSReleasePlan(advsReleasePlanName string, devFw framework.Framework, devNamespace, advsAppName, managedNamespace string, autoRelease string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"releaseNotes": map[string]interface{}{
			"description": "releaseNotes description",
			"references":  []string{"https://server.com/ref1", "http://server2.com/ref2"},
			"solution":    "some solution",
			"synopsis":    "test synopsis",
			"topic":       "test topic",
			"cves":        []map[string]interface{}{
				{
					"key":  "CVE-2024-8260",
					"component" : advsComponentName,
				},
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(advsReleasePlanName, devNamespace, advsAppName,
		managedNamespace, autoRelease, &runtime.RawExtension{
			Raw: data,
		}, nil, nil)
	Expect(err).NotTo(HaveOccurred())
}

func createADVSReleasePlanAdmission(advsRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, advsAppName, advsECPName, pathInRepoValue string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"name":       advsComponentName,
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
			"product_id":      555,
			"product_name":    "test product",
			"product_stream":  "rhtas-tp1",
			"product_version": "v1.0",
			"type":            "RHSA",
		},
		"sign": map[string]interface{}{
			"configMapName":    "hacbs-signing-pipeline-config-redhatbeta2",
			"cosignSecretName": "test-cosign-secret",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(advsRPAName, managedNamespace, "", devNamespace, advsECPName, advsServiceAccountName, []string{advsAppName}, true, &tektonutils.PipelineRef{
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
