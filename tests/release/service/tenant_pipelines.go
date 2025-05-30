package service

import (
	"encoding/json"
	"fmt"
	kubeapi "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
	"time"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("Release service tenant pipeline", Label("release-service", "tenant"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
	var kubeAdminClient *framework.ControllerHub
	var err error
	var devNamespace = "tenant-dev"
	var releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"
	var sampleImage = "quay.io/redhat-appstudio-qe/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var gitSourceURL = "https://github.com/redhat-appstudio-qe/dc-metro-map-release"
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	var tenantServiceAccountName = "tenant-service-account"
	var tenantPullSecretName = "tenant-pull-secret"

	var releaseCR *releaseApi.Release
	var snapshotPush *appservice.Snapshot
	var testEnvironment = utils.GetEnv("TEST_ENVIRONMENT", releasecommon.UpstreamTestEnvironment)

	BeforeAll(func() {
		if testEnvironment == releasecommon.DownstreamTestEnvironment {
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
			Expect(err).NotTo(HaveOccurred())
			devNamespace = fw.UserNamespace
		} else {
			var asAdminClient *kubeapi.CustomClient
			asAdminClient, err = kubeapi.NewAdminKubernetesClient()
			Expect(err).ShouldNot(HaveOccurred())
			kubeAdminClient, err = framework.InitControllerHub(asAdminClient)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = kubeAdminClient.CommonController.CreateTestNamespace(devNamespace)
			Expect(err).ShouldNot(HaveOccurred())
		}

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		_, err := kubeAdminClient.CommonController.GetSecret(devNamespace, tenantPullSecretName)
		if errors.IsNotFound(err) {
			_, err = kubeAdminClient.CommonController.CreateRegistryAuthSecret(tenantPullSecretName, devNamespace, sourceAuthJson)
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(err).ToNot(HaveOccurred())

		_, err = kubeAdminClient.CommonController.CreateServiceAccount(tenantServiceAccountName, devNamespace, []corev1.ObjectReference{{Name: tenantPullSecretName}}, nil)
		Expect(err).ToNot(HaveOccurred())

		_, err = kubeAdminClient.HasController.CreateApplication(releasecommon.ApplicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		data, err := json.Marshal(map[string]interface{}{
			"mapping": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"component":  releasecommon.ComponentName,
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

		_, err = kubeAdminClient.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, "", "", &runtime.RawExtension{
			Raw: data,
		}, tenantPipeline, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = kubeAdminClient.TektonController.CreatePVCInAccessMode(releasecommon.ReleasePvcName, devNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(kubeAdminClient, releasecommon.ComponentName, releasecommon.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
		Expect(err).ShouldNot(HaveOccurred())
		GinkgoWriter.Println("snapshotPush.Name: %s", snapshotPush.GetName())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() && testEnvironment == releasecommon.DownstreamTestEnvironment {
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that a Release CR should have been created in the dev namespace", func() {
			Eventually(func() error {
				releaseCR, err = kubeAdminClient.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				return err
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
		})

		It("verifies that Tenant PipelineRun is triggered", func() {
			Expect(kubeAdminClient.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, devNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
		})

		It("verifies that a Release is marked as succeeded.", func() {
			Eventually(func() error {
				releaseCR, err = kubeAdminClient.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
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
