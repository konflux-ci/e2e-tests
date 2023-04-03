package enterprisecontract

import (
	"fmt"

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

	})

	Context("Command validation and Release Policy", func() {

		BeforeEach(func() {
			generator = tekton.VerifyEnterpriseContract{
				Bundle:              verifyECTaskBundle,
				Image:               imageWithDigest,
				Name:                "verify-enterprise-contract",
				Namespace:           namespace,
				PolicyConfiguration: "ec-policy",
				PublicKey:           fmt.Sprintf("k8s://%s/%s", namespace, publicSecretName),
				SSLCertDir:          "/var/run/secrets/kubernetes.io/serviceaccount",
				Strict:              true,
			}

			// Since specs could update the config policy, make sure it has a consistent
			// baseline at the start of each spec.
			baselinePolicies := ecp.EnterpriseContractPolicySpec{
				Configuration: &ecp.EnterpriseContractPolicyConfiguration{
					// A simple policy that should always succeed in a cluster where
					// Tekton Chains is properly setup.
					Include: []string{"slsa_provenance_available"},
				},
				Sources: policySource,
			}
			Expect(kubeController.CreateOrUpdatePolicyConfiguration(namespace, baselinePolicies)).To(Succeed())
		})

		It("HACBS-1837: ec validate accepts a list of image references ", func() {

			//images to be validated are from a application snapshot file
			generator.Image = applicationSnapshotConfig
			pr, err := kubeController.RunPipeline(generator, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())

			pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
			Expect(err).NotTo(HaveOccurred())

			tr, err := kubeController.GetTaskRunStatus(pr, "verify-enterprise-contract")
			Expect(err).NotTo(HaveOccurred())

			Expect(tr.Status.TaskRunResults).Should(ContainElements(
				tekton.MatchTaskRunResultWithJSONPathValue("HACBS_TEST_OUTPUT", "{$.result}", `["success"]`),
			))

		})
	})
})
