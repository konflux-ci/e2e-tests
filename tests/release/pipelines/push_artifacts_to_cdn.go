package pipelines

import (
	"encoding/json"
	"fmt"
	"time"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
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
	artifactsCatalogPathInRepo = "pipelines/managed/push-artifacts-to-cdn/push-artifacts-to-cdn.yaml"
	artifactsGitSourceURL      = "https://github.com/redhat-appstudio-qe/konflux-test-product"
	artifactsGitSrcSHA         = "2a9c70449cc34f0c0bb8df6e1e1e80eb4b71fa59"
	sampleImage                = "quay.io/hacbs-release-tests/e2e-push-artifacts@sha256:10e2f81778bf27224901fdd19915cada7330e5c4307e5f0e85f8f2c05ad9bd3d"
)

var (
	compRandomStr = util.GenerateRandomString(4)
	artifactsComponentName = "artifactsComp-" + compRandomStr
)

var _ = framework.ReleasePipelinesSuiteDescribe("e2e tests for push-artifacts-to-cdn pipeline", Label("release-pipelines", "push-artifacts-to-cdn"), func() {
	defer GinkgoRecover()

	var (
		devWorkspace     = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
		managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)
		devNamespace     = devWorkspace + "-tenant"
		managedNamespace = managedWorkspace + "-tenant"
		err              error
		devFw            *framework.Framework
		managedFw        *framework.Framework
		artifactsApplicationName = "artifacts-app-" + util.GenerateRandomString(4)
		artifactsReleasePlanName = "artifacts-rp-" + util.GenerateRandomString(4)
		artifactsReleasePlanAdmissionName = "artifacts-rpa-" + util.GenerateRandomString(4)
		artifactsEnterpriseContractPolicyName = "artifacts-policy-" + util.GenerateRandomString(4)
		snapshotPush     *appservice.Snapshot
		releaseCR        *releaseapi.Release
		pipelineRun      *tektonv1.PipelineRun
	)

	Describe("Push-artifacts-to-cdn happy path", Label("PushArtifactsToCDN"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace

			Expect(managedFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(
				managedNamespace, 
				releasecommon.RedhatAppstudioUserSecret, 
				releasecommon.ReleasePipelineServiceAccountDefault, 
				true,
			)).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(artifactsApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			createArtifactsReleasePlan(artifactsReleasePlanName, *devFw, devNamespace, artifactsApplicationName, managedNamespace, "true")
			createArtifactsReleasePlanAdmission(artifactsReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, artifactsApplicationName, artifactsEnterpriseContractPolicyName, artifactsCatalogPathInRepo)
			createArtifactsEnterpriseContractPolicy(artifactsEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(
				devFw.AsKubeAdmin, 
				artifactsComponentName, 
				artifactsApplicationName, 
				devNamespace, 
				sampleImage, 
				artifactsGitSourceURL, 
				artifactsGitSrcSHA, 
				"", "", "", "",
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(artifactsApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(artifactsEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(artifactsReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		Describe("Post-release verification", func() {
			It("verifies if release CR is created", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					return err
				}, 10*time.Minute, releasecommon.DefaultInterval).Should(Succeed(), 
					"timed out when trying to get release CR for snapshot %s/%s", devNamespace, snapshotPush.Name)
			})

			It("verifies the artifacts release pipelinerun is running and succeeds", func() {
				Eventually(func() error {
					pipelineRun, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
					if err != nil {
						return fmt.Errorf("PipelineRun has not been created yet for release %s/%s", releaseCR.GetNamespace(), releaseCR.GetName())
					}

					// Log pipeline run status and conditions for better debugging
					GinkgoWriter.Printf("PipelineRun %s/%s status: %s\n", pipelineRun.Namespace, pipelineRun.Name, pipelineRun.Status.GetCondition(apis.ConditionSucceeded).Status)
					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s condition - Type: %s, Status: %s, Reason: %s, Message: %s\n", 
							pipelineRun.Name, condition.Type, condition.Status, condition.Reason, condition.Message)
					}

					if !pipelineRun.IsDone() {
						return fmt.Errorf("PipelineRun %s/%s is still running", pipelineRun.Namespace, pipelineRun.Name)
					}

					if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
						GinkgoWriter.Printf("PipelineRun %s/%s completed successfully\n", pipelineRun.Namespace, pipelineRun.Name)
						return nil
					}

					// Handle failed pipeline run with improved logging
					GinkgoWriter.Printf("PipelineRun %s/%s failed, collecting logs and artifacts\n", pipelineRun.Namespace, pipelineRun.Name)
					
					if err = managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(artifactsComponentName, pipelineRun); err != nil {
						GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
					}
					
					prLogs, err := tekton.GetFailedPipelineRunLogs(
						managedFw.AsKubeAdmin.ReleaseController.KubeRest(), 
						managedFw.AsKubeAdmin.ReleaseController.KubeInterface(), 
						pipelineRun,
					)
					if err != nil {
						GinkgoWriter.Printf("failed to get PipelineRun logs for %s/%s: %+v\n", pipelineRun.Namespace, pipelineRun.Name, err)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(artifactsComponentName, pipelineRun)).To(Succeed())
						return nil
					}
										
					Expect(prLogs).To(Equal(""), fmt.Sprintf("The failed PipelineRun %s log: %s", pipelineRun.Name, prLogs))
					return nil
				}, 4*time.Hour, releasecommon.DefaultInterval).Should(Succeed(), 
					fmt.Sprintf("timed out when waiting for the release PipelineRun to be finished for the release %s/%s", releaseCR.GetName(), releaseCR.GetNamespace()))
			})

			It("verifies release CR completed and set succeeded.", func() {
				Eventually(func() error {
					releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotPush.Name, devNamespace)
					if err != nil {
						return fmt.Errorf("failed to get release CR for snapshot %s/%s: %w", devNamespace, snapshotPush.Name, err)
					}
					
					GinkgoWriter.Printf("Checking release CR %s/%s status\n", releaseCR.GetNamespace(), releaseCR.GetName())
					
					conditions := releaseCR.Status.Conditions
					if len(conditions) == 0 {
						return fmt.Errorf("release %s/%s has no conditions yet", releaseCR.GetNamespace(), releaseCR.GetName())
					}
					
					// Log all conditions for better debugging
					for _, condition := range conditions {
						GinkgoWriter.Printf("Release CR %s condition - Type: %s, Status: %s, Reason: %s, Message: %s\n", 
							releaseCR.GetName(), condition.Type, condition.Status, condition.Reason, condition.Message)
					}
					
					// Check for Released condition specifically
					for _, condition := range conditions {
						if condition.Type == "Released" {
							switch condition.Status {
							case "True":
								GinkgoWriter.Printf("Release CR %s/%s completed successfully\n", releaseCR.GetNamespace(), releaseCR.GetName())
								return nil
							case "False":
								GinkgoWriter.Printf("Release CR %s/%s failed: %s\n", releaseCR.GetName(), releaseCR.GetNamespace(), condition.Message)
								return fmt.Errorf("release %s/%s failed: %s", releaseCR.GetName(), releaseCR.GetNamespace(), condition.Message)
							default:
								return fmt.Errorf("release %s/%s is not marked as finished yet (status: %s)", 
									releaseCR.GetNamespace(), releaseCR.GetName(), condition.Status)
							}
						}
					}					
					return fmt.Errorf("release %s/%s does not have a 'Released' condition", releaseCR.GetNamespace(), releaseCR.GetName())
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

func createArtifactsReleasePlan(artifactsReleasePlanName string, devFw framework.Framework, devNamespace, artifactsAppName, managedNamespace string, autoRelease string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"releaseNotes": map[string]interface{}{
			"description": "releaseNotes description",
			"references":  []string{"https://server.com/ref1", "http://server2.com/ref2"},
			"solution":    "some solution",
			"synopsis":    "test synopsis",
			"topic":       "test topic",
			"cves": []map[string]interface{}{
				{
					"key":       "CVE-2024-8260",
					"component": artifactsComponentName,
				},
			},
			"issues": map[string]interface{}{
				"fixed": []map[string]interface{}{
					{
						"id":     "RELEASE-1401",
						"source": "issues.redhat.com",
					},
				},
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(artifactsReleasePlanName, devNamespace, artifactsAppName,
		managedNamespace, autoRelease, &runtime.RawExtension{
			Raw: data,
		}, nil, nil)
	Expect(err).NotTo(HaveOccurred())
}

func createArtifactsReleasePlanAdmission(artifactsRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, artifactsAppName, artifactsECPName, pathInRepoValue string) {
	var err error

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"name": artifactsComponentName,
					"repository": "registry.stage.redhat.io/rhtap/konflux-release-e2e",
					"files": []map[string]interface{}{
						{
							"filename": "releng-test-product-binaries-windows-amd64.gz",
							"source": "releng-test-product-binaries-windows-amd64.gz",
							"arch": "amd64",
							"os": "windows",
						},
						{
							"filename": "releng-test-product-binaries-linux-amd64.gz",
							"source": "releng-test-product-binaries-linux-amd64.gz",
							"arch": "amd64",
							"os": "linux",
						},
						{
							"filename": "releng-test-product-binaries-darwin-amd64.gz",
							"source": "releng-test-product-binaries-darwin-amd64.gz",
							"arch": "amd64",
							"os": "darwin",
						},
						{
							"filename": "releng-test-product-binaries-linux-arm64.gz",
							"source": "releng-test-product-binaries-linux-arm64.gz",
							"arch": "arm64",
							"os": "linux",
						},
						{
							"filename": "releng-test-product-binaries-darwin-arm64.gz",
							"source": "releng-test-product-binaries-darwin-arm64.gz",
							"arch": "arm64",
							"os": "darwin",
						},

					},
					"contentGateway": map[string]interface{}{
						"productName": "Releng Test Product",
						"productCode": "RelengTestProduct",
						"productVersionName": "1.6.0",
						"contentType": "binary",
					},
					"tags": []string{"time-{{ timestamp }}", "git-{{ git_sha }}" },
				},
			},

		},
		"releaseNotes": map[string]interface{}{
			"cpe":             "cpe:/a:example.com",
			"product_id":      []int{555},
			"product_name":    "test product",
			"product_stream":  "rhtas-tp1",
			"product_version": "v1.0",
			"type":            "RHSA",
		},
		"sign": map[string]interface{}{
			"configMapName":    "hacbs-signing-pipeline-config-redhatbeta2",
			"cosignSecretName": "test-cosign-secret",
		},

		"cdn": map[string]interface{}{
			"env": "production",
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
