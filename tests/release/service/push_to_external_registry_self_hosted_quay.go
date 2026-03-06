package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ecp "github.com/conforma/crds/api/v1alpha1"
	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("Push to external registry (self-hosted Quay)", ginkgo.Label("release-service", "push-to-external-registry-self-hosted-quay"), func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	ginkgo.AfterEach(framework.ReportFailure(&fw))
	var err error
	var devNamespace = "ex-registry-sh"
	var managedNamespace = "ex-registry-sh-managed"

	var releaseCR *releaseApi.Release
	var snapshotPush *appservice.Snapshot
	var ecPolicyName = "ex-registry-sh-policy-" + util.GenerateRandomString(4)

	var quayInternalHost string
	var sampleImage string
	var releasedImagePushRepo string
	var taOciStorage string

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		devNamespace = fw.UserNamespace
		managedNamespace = utils.GetGeneratedNamespace(managedNamespace)

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error when creating managedNamespace: %v", err)

		quayTestConfig, err := fw.AsKubeAdmin.CommonController.GetConfigMap(releasecommon.QuayTestConfigName, releasecommon.QuayNamespace)
		if k8sErrors.IsNotFound(err) {
			ginkgo.Skip("Self-hosted Quay not available: quay-test-config ConfigMap not found in quay namespace (requires init-quay task)")
		}
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		quayInternalHost = quayTestConfig.Data["quay-internal-host"]
		gomega.Expect(quayInternalHost).NotTo(gomega.BeEmpty())
		imageDigest := quayTestConfig.Data["image-digest"]
		gomega.Expect(imageDigest).NotTo(gomega.BeEmpty())
		destRepo := quayTestConfig.Data["dest-repo"]
		gomega.Expect(destRepo).NotTo(gomega.BeEmpty())

		sampleImage = fmt.Sprintf("%s/%s@%s", quayInternalHost, destRepo, imageDigest)
		releasedImagePushRepo = fmt.Sprintf("%s/test-org/released-%s", quayInternalHost, releasecommon.ComponentName)
		taOciStorage = fmt.Sprintf("%s/test-org/trusted-artifacts", quayInternalHost)

		robotSecret, err := fw.AsKubeAdmin.CommonController.GetSecret(releasecommon.QuayNamespace, releasecommon.QuayRobotCredentialsName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "init-quay task must create quay-robot-credentials secret")

		robotUser := string(robotSecret.Data["username"])
		robotPassword := string(robotSecret.Data["password"])
		gomega.Expect(robotUser).NotTo(gomega.BeEmpty())
		gomega.Expect(robotPassword).NotTo(gomega.BeEmpty())

		dockerConfigJSON := buildDockerConfigJSON(quayInternalHost, robotUser, robotPassword)

		registrySecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      releasecommon.RedhatAppstudioUserSecret,
				Namespace: managedNamespace,
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: dockerConfigJSON,
			},
		}
		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, registrySecret)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		taSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      releasecommon.ReleaseCatalogTAQuaySecret,
				Namespace: managedNamespace,
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: dockerConfigJSON,
			},
		}
		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, taSecret)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		adminTokenSecret, err := fw.AsKubeAdmin.CommonController.GetSecret(releasecommon.QuayNamespace, releasecommon.QuayAdminTokenName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		adminToken := string(adminTokenSecret.Data["token"])
		gomega.Expect(adminToken).NotTo(gomega.BeEmpty())

		apiTokenSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "quay-api-token",
				Namespace: managedNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"token": adminToken,
			},
		}
		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, apiTokenSecret)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(
			releasecommon.ReleasePipelineServiceAccountDefault, managedNamespace,
			releasecommon.ManagednamespaceSecret, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, releasecommon.RedhatAppstudioUserSecret, releasecommon.ReleasePipelineServiceAccountDefault, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

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
						"name":       releasecommon.ComponentName,
						"repository": releasedImagePushRepo,
					},
				},
				"defaults": map[string]interface{}{
					"tags": []string{
						"latest",
					},
					"pushSourceContainer": false,
					"public":              true,
				},
				"registrySecret": "quay-api-token",
			},
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(
			releasecommon.TargetReleasePlanAdmissionName, managedNamespace, "",
			devNamespace, ecPolicyName, releasecommon.ReleasePipelineServiceAccountDefault,
			[]string{releasecommon.ApplicationNameDefault}, false,
			&tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: releasecommon.RelSvcCatalogURL},
					{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
					{Name: "pathInRepo", Value: "pipelines/managed/push-to-external-registry/push-to-external-registry.yaml"},
				},
				OciStorage: taOciStorage,
			},
			&runtime.RawExtension{Raw: data},
		)
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

		gitSourceURL := releasecommon.GitSourceComponentUrl
		gitSourceRevision := "d49914874789147eb2de9bb6a12cd5d150bfff92"

		snapshotPush, err = releasecommon.CreateSnapshotWithImageSource(fw.AsKubeAdmin, releasecommon.ComponentName, releasecommon.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
		ginkgo.GinkgoWriter.Println("snapshotPush.Name: %s", snapshotPush.GetName())
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(gomega.HaveOccurred())
			gomega.Expect(fw.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(devNamespace, time.Minute*5)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(devNamespace, time.Minute*5)).To(gomega.Succeed())
		}
	})

	var _ = ginkgo.Describe("Post-release verification", func() {

		ginkgo.It("verifies that a Release CR should have been created in the dev namespace", func() {
			gomega.Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				return err
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed(), "timed out when waiting for Release CR is created in Namespace %s", devNamespace)
		})

		ginkgo.It("verifies that Release PipelineRun should eventually succeed", func() {
			gomega.Expect(fw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(gomega.Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
		})

		ginkgo.It("verifies that a Release is marked as succeeded", func() {
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

func buildDockerConfigJSON(host, username, password string) []byte {
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	config := map[string]interface{}{
		"auths": map[string]interface{}{
			host: map[string]interface{}{
				"auth": auth,
			},
		},
	}
	data, _ := json.Marshal(config)
	return data
}
