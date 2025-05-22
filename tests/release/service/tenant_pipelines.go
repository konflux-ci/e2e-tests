package service

import (
	"encoding/json"
	"fmt"
	"time"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("Release service tenant pipeline", Label("release-service", "tenant"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
	var err error
	var tenantNamespace string
	var releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"
	var sampleImage = "quay.io/redhat-appstudio-qe/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var gitSourceURL = "https://github.com/redhat-appstudio-qe/dc-metro-map-release"
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	var tenantServiceAccountName = "tenant-service-account"
	var tenantPullSecretName="tenant-pull-secret"

	var releaseCR *releaseApi.Release
	var snapshotPush *appservice.Snapshot

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace("tenant-dev"))
		Expect(err).NotTo(HaveOccurred())
		tenantNamespace = fw.UserNamespace

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		_, err := fw.AsKubeAdmin.CommonController.GetSecret(tenantNamespace, tenantPullSecretName)
		if errors.IsNotFound(err) {
			_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(tenantPullSecretName, tenantNamespace, sourceAuthJson)
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateServiceAccount(tenantServiceAccountName, tenantNamespace, []corev1.ObjectReference{{Name: tenantPullSecretName}}, nil)
                Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(releasecommon.ApplicationNameDefault, tenantNamespace)
		Expect(err).NotTo(HaveOccurred())

		data, err := json.Marshal(map[string]interface{}{
			"mapping": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"component": releasecommon.ComponentName,
						"repository": releasedImagePushRepo,
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		tenantPipeline := &tektonutils.ParameterizedPipeline{}
		tenantPipeline.ServiceAccountName = tenantServiceAccountName 
		tenantPipeline.Timeouts = tektonv1.TimeoutFields{
                                Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
                        }

		tenantPipeline.PipelineRef = tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: "https://github.com/redhat-appstudio-qe/pipeline_examples"},
				{Name: "revision", Value: "main"},
				{Name: "pathInRepo", Value: "pipelines/simple_pipeline.yaml"},
			},
		}

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, tenantNamespace, releasecommon.ApplicationNameDefault, "", "", &runtime.RawExtension{
                        Raw: data,
                }, tenantPipeline, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasecommon.ReleasePvcName, tenantNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(*fw, releasecommon.ComponentName, releasecommon.ApplicationNameDefault, tenantNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
		Expect(err).ShouldNot(HaveOccurred())
		GinkgoWriter.Println("snapshotPush.Name: %s", snapshotPush.GetName())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that a Release CR should have been created in the dev namespace", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(tenantNamespace)
				return err
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
		})

		It("verifies that Tenant PipelineRun is triggered", func() {
			Expect(fw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, tenantNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
		})

		It("verifies that a Release is marked as succeeded.", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(tenantNamespace)
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
