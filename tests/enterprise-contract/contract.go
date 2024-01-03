package enterprisecontract

import (
	"fmt"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/contract"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

var _ = framework.EnterpriseContractSuiteDescribe("Enterprise Contract E2E tests", Label("ec", "HACBS"), func() {

	defer GinkgoRecover()
	var fwk *framework.Framework
	var err error
	var namespace string
	var defaultECP *ecp.EnterpriseContractPolicy
	var imageWithDigest string
	var pipelineRunTimeout int
	var generator tekton.VerifyEnterpriseContract
	var verifyECTaskBundle string
	publicSecretName := "cosign-public-key"
	goldenImagePublicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
		"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZP/0htjhVt2y0ohjgtIIgICOtQtA\n" +
		"naYJRuLprwIv6FDhZ5yFjYUEtsmoNcW7rx2KM6FOXGsCX3BNc7qhHELT+g==\n" +
		"-----END PUBLIC KEY-----")
	AfterEach(framework.ReportFailure(&fwk))

	BeforeAll(func() {
		fwk, err = framework.NewFramework(utils.GetGeneratedNamespace(constants.TEKTON_CHAINS_E2E_USER))
		Expect(err).NotTo(HaveOccurred())
		Expect(fwk.UserNamespace).NotTo(BeEmpty(), "failed to create sandbox user")
		namespace = fwk.UserNamespace
		publicKey, err := fwk.AsKubeAdmin.TektonController.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Copy public key from %s/signing-secrets to a new secret\n", constants.TEKTON_CHAINS_NS)
		Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(publicKey, publicSecretName, namespace)).To(Succeed())

		defaultECP, err = fwk.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred())

		cm, err := fwk.AsKubeAdmin.CommonController.GetConfigMap("ec-defaults", "enterprise-contract-service")
		Expect(err).ToNot(HaveOccurred())
		verifyECTaskBundle = cm.Data["verify_ec_task_bundle"]
		Expect(verifyECTaskBundle).ToNot(BeEmpty())
		GinkgoWriter.Printf("Using verify EC task bundle: %s\n", verifyECTaskBundle)
		imageWithDigest = "quay.io/redhat-appstudio/ec-golden-image@sha256:4b318620a32349fd37827163c67b5ff6e503f05b3ca4dde066ee03bb34be9ae1"
	})
	Context("ec-cli command verification", func() {
		BeforeEach(func() {
			generator = tekton.VerifyEnterpriseContract{
				TaskBundle:          verifyECTaskBundle,
				Name:                "verify-enterprise-contract",
				Namespace:           namespace,
				PolicyConfiguration: "ec-policy",
				PublicKey:           fmt.Sprintf("k8s://%s/%s", namespace, publicSecretName),
				Strict:              false,
				EffectiveTime:       "now",
				IgnoreRekor:         true,
			}
			generator.WithComponentImage(imageWithDigest)
			pipelineRunTimeout = int(time.Duration(5) * time.Minute)
			Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, defaultECP.Spec)).To(Succeed())
		})
		It("verifies ec cli has error handling", func() {
			pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(Succeed())

			pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
			Expect(err).NotTo(HaveOccurred())

			tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
			Expect(err).NotTo(HaveOccurred())

			Expect(tr.Status.Results).Should(Or(
				// TODO: delete the first option after https://issues.redhat.com/browse/RHTAP-810 is completed
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.OldTektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
			))
			//Get container step-report log details from pod
			reportLog, err := utils.GetContainerLogs(fwk.AsKubeAdmin.CommonController.KubeInterface(), tr.Status.PodName, "step-report", namespace)
			GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report", reportLog)
			Expect(err).NotTo(HaveOccurred())
			Expect(reportLog).Should(ContainSubstring("No image attestations found matching the given public key"))
		})

		It("verifies ec validate accepts a list of image references", func() {
			secretName := fmt.Sprintf("golden-image-public-key%s", util.GenerateRandomString(10))
			GinkgoWriter.Println("Update public key to verify golden images")
			Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(goldenImagePublicKey, secretName, namespace)).To(Succeed())
			generator.PublicKey = fmt.Sprintf("k8s://%s/%s", namespace, secretName)

			policy := contract.PolicySpecWithSourceConfig(
				defaultECP.Spec, ecp.SourceConfig{Include: []string{"minimal"}})
			Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(Succeed())

			generator.WithComponentImage("quay.io/redhat-appstudio/ec-golden-image:e2e-test-out-of-date-task")
			generator.AppendComponentImage("quay.io/redhat-appstudio/ec-golden-image:e2e-test-unacceptable-task")
			pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(Succeed())

			pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
			Expect(err).NotTo(HaveOccurred())

			tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
			Expect(err).NotTo(HaveOccurred())

			Expect(tr.Status.Results).Should(Or(
				// TODO: delete the first option after https://issues.redhat.com/browse/RHTAP-810 is completed
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.OldTektonTaskTestOutputName, "{$.result}", `["SUCCESS"]`)),
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["SUCCESS"]`)),
			))
			//Get container step-report log details from pod
			reportLog, err := utils.GetContainerLogs(fwk.AsKubeAdmin.CommonController.KubeInterface(), tr.Status.PodName, "step-report", namespace)
			GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report", reportLog)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Release Policy", func() {
		BeforeAll(func() {
			secretName := fmt.Sprintf("golden-image-public-key%s", util.GenerateRandomString(10))
			GinkgoWriter.Println("Update public key to verify golden images")
			Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(goldenImagePublicKey, secretName, namespace)).To(Succeed())
			generator = tekton.VerifyEnterpriseContract{
				TaskBundle:          verifyECTaskBundle,
				Name:                "verify-enterprise-contract",
				Namespace:           namespace,
				PolicyConfiguration: "ec-policy",
				PublicKey:           fmt.Sprintf("k8s://%s/%s", namespace, secretName),
				Strict:              false,
				EffectiveTime:       "now",
				IgnoreRekor:         true,
			}
			generator.WithComponentImage(imageWithDigest)
			pipelineRunTimeout = int(time.Duration(5) * time.Minute)
		})

		It("verifies the release policy: Task bundles are in acceptable bundle list", func() {
			policy := contract.PolicySpecWithSourceConfig(
				defaultECP.Spec,
				ecp.SourceConfig{Include: []string{"attestation_task_bundle.task_ref_bundles_acceptable"}},
			)
			Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(Succeed())

			generator.WithComponentImage("quay.io/redhat-appstudio/ec-golden-image:e2e-test-unacceptable-task")
			pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(Succeed())

			pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
			Expect(err).NotTo(HaveOccurred())

			tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
			Expect(err).NotTo(HaveOccurred())

			Expect(tr.Status.Results).Should(Or(
				// TODO: delete the first option after https://issues.redhat.com/browse/RHTAP-810 is completed
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.OldTektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
			))

			//Get container step-report log details from pod
			reportLog, err := utils.GetContainerLogs(fwk.AsKubeAdmin.CommonController.KubeInterface(), tr.Status.PodName, "step-report", namespace)
			GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report", reportLog)
			Expect(err).NotTo(HaveOccurred())
			Expect(reportLog).Should(MatchRegexp(`Pipeline task .* uses an unacceptable task bundle`))
		})

		It("verifies the release policy: Task bundle references pinned to digest", func() {
			secretName := fmt.Sprintf("unpinned-task-bundle-public-key%s", util.GenerateRandomString(10))
			unpinnedTaskPublicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
				"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEPfwkY/ru2JRd6FSqIp7lT3gzjaEC\n" +
				"EAg+paWtlme2KNcostCsmIbwz+bc2aFV+AxCOpRjRpp3vYrbS5KhkmgC1Q==\n" +
				"-----END PUBLIC KEY-----")
			GinkgoWriter.Println("Update public <key to verify unpinned task image")
			Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(unpinnedTaskPublicKey, secretName, namespace)).To(Succeed())
			generator.PublicKey = fmt.Sprintf("k8s://%s/%s", namespace, secretName)

			policy := contract.PolicySpecWithSourceConfig(
				defaultECP.Spec,
				ecp.SourceConfig{Include: []string{"attestation_task_bundle.task_ref_bundles_pinned"}},
			)
			Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(Succeed())

			generator.WithComponentImage("quay.io/redhat-appstudio-qe/enterprise-contract-tests:e2e-test-unpinned-task-bundle")
			pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(Succeed())

			pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
			Expect(err).NotTo(HaveOccurred())

			tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
			Expect(err).NotTo(HaveOccurred())

			Expect(tr.Status.Results).Should(Or(
				// TODO: delete the first option after https://issues.redhat.com/browse/RHTAP-810 is completed
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.OldTektonTaskTestOutputName, "{$.result}", `["WARNING"]`)),
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["WARNING"]`)),
			))

			//Get container step-report log details from pod
			reportLog, err := utils.GetContainerLogs(fwk.AsKubeAdmin.CommonController.KubeInterface(), tr.Status.PodName, "step-report", namespace)
			GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report", reportLog)
			Expect(err).NotTo(HaveOccurred())
			Expect(reportLog).Should(MatchRegexp(`Pipeline task .* uses an unpinned task bundle reference`))
		})
	})
})
