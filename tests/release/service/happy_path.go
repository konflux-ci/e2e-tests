package service

import (
	"encoding/json"
	"fmt"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/conforma/crds/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("Release service happy path", ginkgo.Label("release-service", "happy-path"), func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	ginkgo.AfterEach(framework.ReportFailure(&fw))
	var err error
	var compName string
	var devNamespace = "happy-path"
	var managedNamespace = "happy-path-managed"
	var snapshotPush *appservice.Snapshot
	var verifyConformaTaskName = "verify-conforma"
	var releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"
	var sampleImage = "quay.io/hacbs-release-tests/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var gitSourceURL = releasecommon.GitSourceComponentUrl
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	var ecPolicyName = "hpath-policy-" + util.GenerateRandomString(4)

	var releaseCR *releaseApi.Release

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error when creating managedNamespace: %v", err)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		gomega.Expect(sourceAuthJson).ToNot(gomega.BeEmpty())

		releaseCatalogTrustedArtifactsQuayAuthJson := utils.GetEnv("RELEASE_CATALOG_TA_QUAY_TOKEN", "")
		gomega.Expect(releaseCatalogTrustedArtifactsQuayAuthJson).ToNot(gomega.BeEmpty())

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(releasecommon.ReleasePipelineServiceAccountDefault, managedNamespace, releasecommon.ManagednamespaceSecret, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(releasecommon.RedhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Create secret for quay repo for trusted artifacts "quay.io/konflux-ci/release-service-trusted-artifacts".
		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(releasecommon.ReleaseCatalogTAQuaySecret, managedNamespace, releaseCatalogTrustedArtifactsQuayAuthJson)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, releasecommon.ReleasePipelineServiceAccountDefault, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		//temporarily usage
		releasePublicKeyDecoded := []byte("-----BEGIN PUBLIC KEY-----\n" +
			"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEocSG/SnE0vQ20wRfPltlXrY4Ib9B\n" +
			"CRnFUCg/fndZsXdz0IX5sfzIyspizaTbu4rapV85KirmSBU6XUaLY347xg==\n" +
			"-----END PUBLIC KEY-----")
		gomega.Expect(fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(
			releasePublicKeyDecoded, releasecommon.PublicSecretNameAuth, managedNamespace)).To(gomega.Succeed())

		defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   fmt.Sprintf("k8s://%s/%s", managedNamespace, releasecommon.PublicSecretNameAuth),
			Sources:     defaultEcPolicy.Spec.Sources,
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"@slsa3"},
				Exclude:     []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			},
		}
		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(ecPolicyName, managedNamespace, defaultEcPolicySpec)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(releasecommon.ApplicationNameDefault, devNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, managedNamespace, "", nil, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

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
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(releasecommon.TargetReleasePlanAdmissionName, managedNamespace, "", devNamespace, ecPolicyName, releasecommon.ReleasePipelineServiceAccountDefault, []string{releasecommon.ApplicationNameDefault}, false, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
			},
		}, &runtime.RawExtension{
			Raw: data,
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasecommon.ReleasePvcName, managedNamespace, corev1.ReadWriteOnce)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", releasecommon.ReleasePipelineServiceAccountDefault, managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(fw.AsKubeAdmin, releasecommon.ComponentName, releasecommon.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
		ginkgo.GinkgoWriter.Println("snapshotPush.Name: %s", snapshotPush.GetName())
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(gomega.Succeed())
		}
	})

	var _ = ginkgo.Describe("Post-release verification", func() {

		ginkgo.It("verifies that a Release CR should have been created in the dev namespace", func() {
			gomega.Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				return err
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
		})

		ginkgo.It("verifies that Release PipelineRun is triggered", func() {
			gomega.Expect(fw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(gomega.Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
		})

		ginkgo.It("verifies that Enterprise Contract Task has succeeded in the Release PipelineRun", func() {
			gomega.Eventually(func() error {
				pr, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				ecTaskRunStatus, err := fw.AsKubeAdmin.TektonController.GetTaskRunStatus(fw.AsKubeAdmin.CommonController.KubeRest(), pr, verifyConformaTaskName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				ginkgo.GinkgoWriter.Printf("the status of the %s TaskRun on the release pipeline is: %v", verifyConformaTaskName, ecTaskRunStatus.Status.Conditions)
				gomega.Expect(tekton.DidTaskSucceed(ecTaskRunStatus)).To(gomega.BeTrue())
				return nil
			}, releasecommon.ReleasePipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
		})

		ginkgo.It("verifies that a Release is marked as succeeded.", func() {
			gomega.Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				if err != nil {
					return err
				}
				if !releaseCR.IsReleased() {
					return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
				}
				return nil
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
		})
	})
})
