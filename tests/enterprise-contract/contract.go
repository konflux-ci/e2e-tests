package enterprisecontract

import (
	"fmt"
	"time"

	"github.com/devfile/library/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

var _ = framework.EnterpriseContractSuiteDescribe("Enterprise Contract E2E tests", Label("enterprise-contract", "HACBS"), func() {

	defer GinkgoRecover()
	var fwk *framework.Framework
	var err error
	var namespace string
	var kubeController tekton.KubeController
	var policySource []ecp.Source
	var imageWithDigest string
	var pipelineRunTimeout int
	var generator tekton.VerifyEnterpriseContract
	var verifyECTaskBundle string
	publicSecretName := "cosign-public-key"

	BeforeAll(func() {
		Skip("Skip until: https://issues.redhat.com/browse/RHTAPBUGS-236 it is closed")
		fwk, err = framework.NewFramework(constants.TEKTON_CHAINS_E2E_USER)
		Expect(err).NotTo(HaveOccurred())
		Expect(fwk.UserNamespace).NotTo(BeNil(), "failed to create sandbox user")
		namespace = fwk.UserNamespace
		kubeController = tekton.KubeController{
			Commonctrl: *fwk.AsKubeAdmin.CommonController,
			Tektonctrl: *fwk.AsKubeAdmin.TektonController,
			Namespace:  namespace,
		}
		publicKey, err := kubeController.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Copy public key from %s/signing-secrets to a new secret\n", constants.TEKTON_CHAINS_NS)
		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretName, namespace)).To(Succeed())

		defaultEcp, err := kubeController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred())
		policySource = defaultEcp.Spec.Sources

		cm, err := kubeController.Commonctrl.GetConfigMap("ec-defaults", "enterprise-contract-service")
		Expect(err).ToNot(HaveOccurred())
		verifyECTaskBundle = cm.Data["verify_ec_task_bundle"]
		Expect(verifyECTaskBundle).ToNot(BeEmpty())
		GinkgoWriter.Printf("Using verify EC task bundle: %s\n", verifyECTaskBundle)
		imageWithDigest = "quay.io/redhat-appstudio/ec-golden-image:latest"
	})

	Context("Release Policy", func() {
		BeforeAll(func() {
			secretName := fmt.Sprintf("golden-image-public-key%s", util.GenerateRandomString(10))
			//The staging public key for verificaiton image
			publicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
				"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZP/0htjhVt2y0ohjgtIIgICOtQtA\n" +
				"naYJRuLprwIv6FDhZ5yFjYUEtsmoNcW7rx2KM6FOXGsCX3BNc7qhHELT+g==\n" +
				"-----END PUBLIC KEY-----")
			GinkgoWriter.Println("Create golden image public signing key")
			Expect(kubeController.CreateOrUpdateSigningSecret(publicKey, secretName, namespace)).To(Succeed())
			generator = tekton.VerifyEnterpriseContract{
				Bundle:              verifyECTaskBundle,
				Image:               imageWithDigest,
				Name:                "verify-enterprise-contract",
				Namespace:           namespace,
				PolicyConfiguration: "ec-policy",
				PublicKey:           fmt.Sprintf("k8s://%s/%s", namespace, secretName),
				SSLCertDir:          "/var/run/secrets/kubernetes.io/serviceaccount",
				Strict:              false,
				EffectiveTime:       "now",
			}
			pipelineRunTimeout = int(time.Duration(5) * time.Minute)
		})

		It("verifies the release policy: Task bundle is not acceptable", func() {
			policy := ecp.EnterpriseContractPolicySpec{
				Sources: policySource,
				Configuration: &ecp.EnterpriseContractPolicyConfiguration{
					Include: []string{"attestation_task_bundle.task_ref_bundles_acceptable"},
				},
			}
			Expect(kubeController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(Succeed())

			generator.Image = "quay.io/redhat-appstudio/ec-golden-image:e2e-test-unacceptable-task"
			pr, err := kubeController.RunPipeline(generator, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())

			pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
			Expect(err).NotTo(HaveOccurred())

			tr, err := kubeController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
			Expect(err).NotTo(HaveOccurred())

			Expect(tr.Status.TaskRunResults).Should(Or(
				// TODO: delete the first option after https://issues.redhat.com/browse/RHTAP-810 is completed
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.OldTektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
			))

			//Get container step-report log details from pod
			reportLog, err := kubeController.Commonctrl.GetContainerLogs(tr.Status.PodName, "step-report", namespace)
			GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report", reportLog)
			Expect(err).NotTo(HaveOccurred())
			Expect(reportLog).Should(ContainSubstring("msg: Pipeline task 'build-container' uses an unacceptable task bundle"))
		})

		It("verifies the release policy: Task bundle is out of date", func() {
			policy := ecp.EnterpriseContractPolicySpec{
				Sources: policySource,
				Configuration: &ecp.EnterpriseContractPolicyConfiguration{
					Include: []string{"attestation_task_bundle.task_ref_bundles_current"},
				},
			}
			Expect(kubeController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(Succeed())

			generator.Image = "quay.io/redhat-appstudio/ec-golden-image:e2e-test-out-of-date-task"
			generator.EffectiveTime = "2023-03-31T00:00:00Z"
			pr, err := kubeController.RunPipeline(generator, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())

			pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
			Expect(err).NotTo(HaveOccurred())

			tr, err := kubeController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
			Expect(err).NotTo(HaveOccurred())

			Expect(tr.Status.TaskRunResults).Should(Or(
				// TODO: delete the first option after https://issues.redhat.com/browse/RHTAP-810 is completed
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.OldTektonTaskTestOutputName, "{$.result}", `["WARNING"]`)),
				ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["WARNING"]`)),
			))

			//Get container step-report log details from pod
			reportLog, err := kubeController.Commonctrl.GetContainerLogs(tr.Status.PodName, "step-report", namespace)
			GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report", reportLog)
			Expect(err).NotTo(HaveOccurred())
			Expect(reportLog).Should(ContainSubstring("Pipeline task 'build-container' uses an out of date task bundle"))
		})
	})
})
