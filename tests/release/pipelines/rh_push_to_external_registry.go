package pipelines

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/release"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleasePipelinesSuiteDescribe("[HACBS-1571]test-release-e2e-push-image-to-pyxis", Label("release-pipelines", "pushPyxis"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
	var err error
	var devNamespace, managedNamespace, compName, additionalCompName string
	var avgControllerQueryTimeout = 5 * time.Minute

	var imageIDs []string
	var pyxisKeyDecoded, pyxisCertDecoded []byte
	//	var releasePR, releasePR2 *pipeline.PipelineRun
	var releasePR *pipeline.PipelineRun
	var scGitRevision = fmt.Sprintf("test-pyxis-%s", util.GenerateRandomString(4))
	var sampleImage = "quay.io/hacbs-release-tests/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var additionalImage = "quay.io/hacbs-release-tests/simplepython@sha256:87ebb63d7b7ba0196093195592c03f5f6e23db9b889c7325e5e081feb16755a1"
	var gitSourceURL = releasecommon.GitSourceComponentUrl
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	var gitAdditionSrcURL = releasecommon.AdditionalGitSourceComponentUrl
	var gitAdditionSrcRevision = "47fc22092005aabebce233a9b6eab994a8152bbd"
	var ecPolicyName = "pushpyxis-policy-" + util.GenerateRandomString(4)

	var snapshotPush *appservice.Snapshot
	var releaseCR *releaseApi.Release

	BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace("push-pyxis"))
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace
		managedNamespace = utils.GetGeneratedNamespace("push-pyxis-managed")

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace")

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
		Expect(keyPyxisStage).ToNot(BeEmpty())

		certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
		Expect(certPyxisStage).ToNot(BeEmpty())

		// Create secret for the release registry repo "hacbs-release-tests".
		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(releasecommon.RedhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		// Creating k8s secret to access Pyxis stage based on base64 decoded of key and cert
		pyxisKeyDecoded, err = base64.StdEncoding.DecodeString(string(keyPyxisStage))
		Expect(err).ToNot(HaveOccurred())

		pyxisCertDecoded, err = base64.StdEncoding.DecodeString(string(certPyxisStage))
		Expect(err).ToNot(HaveOccurred())

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

		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
		Expect(err).ToNot(HaveOccurred())

		//temporarily usage
		releasePublicKeyDecoded := []byte("-----BEGIN PUBLIC KEY-----\n" +
			"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEocSG/SnE0vQ20wRfPltlXrY4Ib9B\n" +
			"CRnFUCg/fndZsXdz0IX5sfzIyspizaTbu4rapV85KirmSBU6XUaLY347xg==\n" +
			"-----END PUBLIC KEY-----")

		Expect(fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(
			releasePublicKeyDecoded, releasecommon.PublicSecretNameAuth, managedNamespace)).To(Succeed())
		defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred())
		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   fmt.Sprintf("k8s://%s/%s", managedNamespace, releasecommon.PublicSecretNameAuth),
			Sources:     defaultEcPolicy.Spec.Sources,
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"@slsa3"},
				Exclude:     []string{"tests/release/pipelines/push_to_external_registry.go"},
			},
		}
		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(ecPolicyName, managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred())

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(releasecommon.ReleasePipelineServiceAccountDefault, managedNamespace, releasecommon.ManagednamespaceSecret, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, releasecommon.ReleasePipelineServiceAccountDefault, true)
		Expect(err).ToNot(HaveOccurred())

		compName = releasecommon.ComponentName
		additionalCompName = releasecommon.AdditionalComponentName

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, managedNamespace, "true", nil, nil, nil)
		Expect(err).NotTo(HaveOccurred())

		data, err := json.Marshal(map[string]interface{}{
			"mapping": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"name":       compName,
						"repository": "quay.io/" + utils.GetQuayIOOrganization() + "/dcmetromap",
					},
					{
						"name":       additionalCompName,
						"repository": "quay.io/" + utils.GetQuayIOOrganization() + "/simplepython",
					},
				},
				"defaults": map[string]interface{}{
					"tags": []string{
						"latest",
					},
				},
			},
			"pyxis": map[string]interface{}{
				"server": "stage",
				"secret": "pyxis",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(releasecommon.TargetReleasePlanAdmissionName, managedNamespace, "", devNamespace, ecPolicyName, releasecommon.ReleasePipelineServiceAccountDefault, []string{releasecommon.ApplicationNameDefault}, true, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/rh-push-to-external-registry/rh-push-to-external-registry.yaml"},
			},
		}, &runtime.RawExtension{
			Raw: data,
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasecommon.ReleasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", releasecommon.ReleasePipelineServiceAccountDefault, managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(releasecommon.ApplicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(*fw, releasecommon.ComponentName, releasecommon.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, releasecommon.AdditionalComponentName, additionalImage, gitAdditionSrcURL, gitAdditionSrcRevision)
		GinkgoWriter.Println("snapshotPush.Name: %s", snapshotPush.GetName())
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterAll(func() {
		err = fw.AsKubeAdmin.CommonController.Github.DeleteRef(constants.StrategyConfigsRepo, scGitRevision)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
		}
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("tests that Release CR is created for the Snapshot", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease("", snapshotPush.GetName(), devNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Release CR for snapshot %s/%s: %v\n", snapshotPush.GetNamespace(), releasecommon.ComponentName, err)
					return err
				}
				return nil
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed(), "timed out waiting for Release CRs to be created in %s namespace", devNamespace)
		})

		It("verifies a release PipelineRun is started and succeeded in managed namespace", func() {
			releasePR, err = fw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToGetStarted(releaseCR, managedNamespace)
			Expect(err).NotTo(HaveOccurred())

			Expect(fw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
		})

		It("validate the result of task create-pyxis-image contains image ids", func() {
			Eventually(func() []string {
				trReleaseLogs, err := fw.AsKubeAdmin.TektonController.GetTaskRunLogs(releasePR.GetName(), "create-pyxis-image", releasePR.GetNamespace())
				Expect(err).NotTo(HaveOccurred())

				trReleaseImageIDs, err := fw.AsKubeAdmin.ReleaseController.GetPyxisImageIDsFromCreatePyxisImageTaskLogs(trReleaseLogs)
				Expect(err).NotTo(HaveOccurred())

				Expect(trReleaseImageIDs).NotTo(BeEmpty(), fmt.Sprintf("Invalid ImageID in results of task create-pyxis-image. TaskRun log: %+s", trReleaseLogs))
				imageIDs = trReleaseImageIDs

				return imageIDs
			}, avgControllerQueryTimeout, releasecommon.DefaultInterval).Should(HaveLen(2))
		})

		It("tests that Release CR has completed", func() {
			Eventually(func() error {
				var errMsg string
				for _, cr := range []*releaseApi.Release{releaseCR} {
					cr, err = fw.AsKubeAdmin.ReleaseController.GetRelease("", cr.Spec.Snapshot, devNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					if !cr.IsReleased() {
						errMsg += fmt.Sprintf("release %s/%s is not marked as finished yet", cr.GetNamespace(), cr.GetName())
					}
				}
				if len(errMsg) > 1 {
					return fmt.Errorf("%s", errMsg)
				}
				return nil
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed(), "timed out waiting for Release CRs to be created in %s namespace", devNamespace)
		})

		It("validates that imageIds from task create-pyxis-image exist in Pyxis.", func() {
			for _, imageID := range imageIDs {
				Eventually(func() error {
					body, err := fw.AsKubeAdmin.ReleaseController.GetPyxisImageByImageID(releasecommon.PyxisStageImagesApiEndpoint, imageID,
						[]byte(pyxisCertDecoded), []byte(pyxisKeyDecoded))
					Expect(err).NotTo(HaveOccurred(), "failed to get response body")

					sbomImage := release.Image{}
					Expect(json.Unmarshal(body, &sbomImage)).To(Succeed(), "failed to unmarshal body content: %s", string(body))

					return nil
				}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
			}
		})
	})
})
