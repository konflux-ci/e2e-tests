package pipelines

import (
	"encoding/json"
	"fmt"
	"time"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	artifactsCatalogPathInRepo  = "pipelines/managed/push-artifacts-to-cdn/push-artifacts-to-cdn.yaml"
	artifactsGitSourceURL       = "https://github.com/redhat-appstudio-qe/konflux-test-product"
	artifactsGitSrcSHA          = "2a9c70449cc34f0c0bb8df6e1e1e80eb4b71fa59"
)
var compRandomStr = util.GenerateRandomString(4)
var artifactsComponentName = "rhel-ai-nvidia-1.1-" + compRandomStr

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for push-artifacts-to-cdn pipeline", Label("release-pipelines", "push-artifacts-to-cdn"), func() {
	defer GinkgoRecover()

	var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var artifactsApplicationName = "artifacts-app-" + util.GenerateRandomString(4)
	var artifactsReleasePlanName = "artifacts-rp-" + util.GenerateRandomString(4)
	var artifactsReleasePlanAdmissionName = "artifacts-rpa-" + util.GenerateRandomString(4)
	var artifactsEnterpriseContractPolicyName = "artifacts-policy-" + util.GenerateRandomString(4)
	var sampleImage = "quay.io/hacbs-release-tests/e2e-push-artifacts@sha256:10e2f81778bf27224901fdd19915cada7330e5c4307e5f0e85f8f2c05ad9bd3d"

	var snapshotPush *appservice.Snapshot
	var releaseCR *releaseapi.Release
	var pipelineRun *tektonv1.PipelineRun

	Describe("Push-artifacts-to-cdn happy path", Label("PushArtifactsToCDN"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			managedNamespace = managedFw.UserNamespace

			err = managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, releasecommon.ReleasePipelineServiceAccountDefault, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(artifactsApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(artifactsReleasePlanName, devNamespace, artifactsApplicationName, managedNamespace, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createArtifactsReleasePlanAdmission(artifactsReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, artifactsApplicationName, artifactsEnterpriseContractPolicyName, artifactsCatalogPathInRepo)

			createArtifactsEnterpriseContractPolicy(artifactsEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(devFw.AsKubeAdmin, artifactsComponentName, artifactsApplicationName, devNamespace, sampleImage, artifactsGitSourceURL, artifactsGitSrcSHA, "", "", "", "")
                        Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(artifactsApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(artifactsEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(artifactsReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {

			It("verifies if release CR is created", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return err
					}
					return nil
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed(), "timed out when trying to get release CR for snapshot %s/%s", devNamespace, snapshotPush.Name)
			})

			It("verifies the artifacts release pipelinerun is running and succeeds", func() {

				Eventually(func() error {
					pipelineRun, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
					if err != nil {
						return fmt.Errorf("PipelineRun has not been created yet for release %s/%s", releaseCR.GetNamespace(), releaseCR.GetName())
					}

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pipelineRun.Name, condition.Reason)
					}

					if !pipelineRun.IsDone(){
						return fmt.Errorf("PipelineRun %s has still not finished yet", pipelineRun.Name)
					}

					if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
						return nil
					} else {
						if err = managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(artifactsComponentName, pipelineRun); err != nil {
							GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
						}
						if err = managedFw.AsKubeDeveloper.CommonController.StorePodsForPipelineRun(managedNamespace, pipelineRun.GetName()); err != nil {
							GinkgoWriter.Printf("failed to store pods for PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
					}
						prLogs := ""
						if prLogs, err = tekton.GetFailedPipelineRunLogs(managedFw.AsKubeAdmin.ReleaseController.KubeRest(), managedFw.AsKubeAdmin.ReleaseController.KubeInterface(), pipelineRun); err != nil {
							GinkgoWriter.Printf("failed to get PLR logs: %+v", err)
							Expect(err).ShouldNot(HaveOccurred())
							Expect(managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(artifactsComponentName, pipelineRun)).To(Succeed())

							return nil
						}
						GinkgoWriter.Printf("logs: %s", prLogs)
						Expect(prLogs).To(Equal(""), fmt.Sprintf("PipelineRun %s failed", pipelineRun.Name))
						return nil
					}
				}, 4*time.Hour, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the release PipelineRun to be finished for the release %s/%s", releaseCR.GetName(), releaseCR.GetNamespace()))
			})

			It("verifies release CR completed and set succeeded.", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return err
					}
					GinkgoWriter.Printf("releaseCR: %s ", releaseCR.Name)
					conditions := releaseCR.Status.Conditions
					GinkgoWriter.Printf("len of conditions: %d ", len(conditions))
					if len(conditions) > 0 {
						for _, c := range conditions {
							GinkgoWriter.Printf("type of c: %s ", c.Type)
							if c.Type == "Released" {
								GinkgoWriter.Printf("status of c: %s ", c.Status)
								if c.Status == "True" {
									GinkgoWriter.Println("Release CR is released")
									return nil
								} else if c.Status == "False" {
									GinkgoWriter.Println("Release CR failed")
									Expect(string(c.Status)).To(Equal("True"), fmt.Sprintf("Release %s failed", releaseCR.Name))
									return nil
								} else {
									return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
								}
							}
						}
					}
					return nil
				}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
			})

		})
	})
})

func createArtifactsEnterpriseContractPolicy(artifactsECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
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

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(artifactsECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createArtifactsReleasePlanAdmission(artifactsRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, artifactsAppName, artifactsECPName, pathInRepoValue string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"name": artifactsComponentName,
					"staged": map[string]interface{}{
						"destination": "rhelai-1_DOT_1-for-rhel-9-x86_64-isos",
						"files": []map[string]interface{}{
							{
								"filename": "rhel-ai-nvidia-1.1-"+compRandomStr+"{{ timestamp }}-x86_64-kvm.qcow2",
								"source": "disk.qcow2",
							},
							{
								"filename": "rhel-ai-nvidia-1.1-"+compRandomStr+"-{{ timestamp }}-x86_64.raw",
								"source": "disk.raw",
							},
							{
								"filename": "rhel-ai-nvidia-1.1-"+compRandomStr+"-{{ timestamp }}-x86_64-boot.iso",
								"source": "install.iso",
							},
						},
					},
					/*
					"contentGateway": map[string]interface{}{
						"productName": "E2ETest Red Hat Enterprise Linux AI",
						"productCode": "RHELAIE2ETest",
						"productVersionName": "RHELAI 1.1",
						"filePrefix": "rhel-ai-nvidia-1.1-"+compRandomStr,
					},
					*/
				},
			},
		},
		"tags": []string{"time-{{ timestamp }}", "git-{{ git_sha }}" },
		"cdn": map[string]interface{}{
			"env": "qa",
		},
	})
	Expect(err).NotTo(HaveOccurred())

/*	timeouts := &tektonv1.TimeoutFields{
			Pipeline: &metav1.Duration{Duration: 4 * time.Hour},
			Tasks:    &metav1.Duration{Duration: 2 * time.Hour},
	}
	*/

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(artifactsRPAName, managedNamespace, "", devNamespace, artifactsECPName, releasecommon.ReleasePipelineServiceAccountDefault, []string{artifactsAppName}, true, &tektonutils.PipelineRef{
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
