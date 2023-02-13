package build

import (
	"fmt"
	"time"

	"github.com/devfile/library/pkg/util"
	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

var _ = framework.ChainsSuiteDescribe("Tekton Chains E2E tests", Label("ec", "HACBS"), func() {
	defer GinkgoRecover()

	var fwk *framework.Framework

	BeforeAll(func() {
		// Initialize the tests controllers
		var err error
		fwk, err = framework.NewFramework()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("infrastructure is running", Label("pipeline"), func() {
		It("verifies if the chains controller is running", func() {
			err := fwk.CommonController.WaitForPodSelector(fwk.CommonController.IsPodRunning, constants.TEKTON_CHAINS_NS, "app", "tekton-chains-controller", 60, 100)
			Expect(err).NotTo(HaveOccurred())
		})
		It("verifies if the correct secrets have been created", func() {
			_, err := fwk.CommonController.GetSecret(constants.TEKTON_CHAINS_NS, "chains-ca-cert")
			Expect(err).NotTo(HaveOccurred())
		})
		It("verifies if the correct roles are created", func() {
			_, csaErr := fwk.CommonController.GetRole("chains-secret-admin", constants.TEKTON_CHAINS_NS)
			Expect(csaErr).NotTo(HaveOccurred())
			_, srErr := fwk.CommonController.GetRole("secret-reader", "openshift-ingress-operator")
			Expect(srErr).NotTo(HaveOccurred())
		})
		It("verifies if the correct rolebindings are created", func() {
			_, csaErr := fwk.CommonController.GetRoleBinding("chains-secret-admin", constants.TEKTON_CHAINS_NS)
			Expect(csaErr).NotTo(HaveOccurred())
			_, csrErr := fwk.CommonController.GetRoleBinding("chains-secret-reader", "openshift-ingress-operator")
			Expect(csrErr).NotTo(HaveOccurred())
		})
		It("verifies if the correct service account is created", func() {
			_, err := fwk.CommonController.GetServiceAccount("chains-secrets-admin", constants.TEKTON_CHAINS_NS)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("test creating and signing an image and task", Label("pipeline"), func() {
		// Make the PipelineRun name and namespace predictable. For convenience, the name of the
		// PipelineRun that builds an image, is the same as the repository where the image is
		// pushed to.
		namespace := constants.TEKTON_CHAINS_E2E_NS
		buildPipelineRunName := fmt.Sprintf("buildah-demo-%s", util.GenerateRandomString(10))
		image := fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/%s/%s", namespace, buildPipelineRunName)
		var imageWithDigest string
		serviceAccountName := "pipeline"

		pipelineRunTimeout := int(time.Duration(20) * time.Minute)
		attestationTimeout := time.Duration(60) * time.Second

		var kubeController tekton.KubeController

		var policySource []ecp.Source

		BeforeAll(func() {
			kubeController = tekton.KubeController{
				Commonctrl: *fwk.CommonController,
				Tektonctrl: *fwk.TektonController,
				Namespace:  namespace,
			}

			// Create the e2e test namespace
			_, err := kubeController.Commonctrl.CreateTestNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace %q: %v", namespace, err)

			// Wait until the "pipeline" SA is created
			GinkgoWriter.Printf("Wait until the %q SA is created in namespace %q\n", serviceAccountName, namespace)
			Eventually(func() bool {
				sa, err := kubeController.Commonctrl.GetServiceAccount(serviceAccountName, namespace)
				return sa != nil && err == nil
			}).WithTimeout(1*time.Minute).WithPolling(100*time.Millisecond).Should(
				BeTrue(), "timed out when waiting for the %q SA to be created", serviceAccountName)

			cm, err := kubeController.Commonctrl.GetConfigMap("ec-defaults", "enterprise-contract-service")
			Expect(err).ToNot(HaveOccurred())
			// the default policy source
			policySource = []ecp.Source{
				{
					Name:   "ec-policies",
					Policy: []string{cm.Data["ec_policy_source"]},
					Data:   []string{cm.Data["ec_data_source"]},
				},
			}

			// if there is a ConfigMap e2e-tests/ec-config with keys `revision` and
			// `repository` values from those will replace the default policy source
			// this gives us a way to set the tests to use a different policy if we
			// break the tests in the default policy source
			// if config, err := fwk.CommonController.K8sClient.KubeInterface().CoreV1().ConfigMaps("e2e-tests").Get(context.TODO(), "ec-config", v1.GetOptions{}); err != nil {
			// 	if v, ok := config.Data["revision"]; ok {
			// 		policySource.Revision = &v
			// 	}
			// 	if v, ok := config.Data["repository"]; ok {
			// 		policySource.Repository = v
			// 	}
			// }

			// At a bare minimum, each spec within this context relies on the existence of
			// an image that has been signed by Tekton Chains. Trigger a demo task to fulfill
			// this purpose.

			bundles, err := fwk.TektonController.NewBundles()
			Expect(err).ShouldNot(HaveOccurred())
			dockerBuildBundle := bundles.DockerBuildBundle
			Expect(dockerBuildBundle).NotTo(Equal(""), "Can't continue without a docker-build pipeline got from selector config")
			pr, err := kubeController.RunPipeline(tekton.BuildahDemo{Image: image, Bundle: dockerBuildBundle}, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			// Verify that the build task was created as expected.
			Expect(pr.ObjectMeta.Name).To(Equal(buildPipelineRunName))
			Expect(pr.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())
			GinkgoWriter.Printf("The pipeline named %q in namespace %q succeeded\n", pr.ObjectMeta.Name, pr.ObjectMeta.Namespace)

			// The PipelineRun resource has been updated, refresh our reference.
			pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.ObjectMeta.Name, pr.ObjectMeta.Namespace)
			Expect(err).NotTo(HaveOccurred())

			// Verify TaskRun has the type hinting required by Tekton Chains
			digest, err := kubeController.GetTaskRunResult(pr, "build-container", "IMAGE_DIGEST")
			Expect(err).NotTo(HaveOccurred())
			i, err := kubeController.GetTaskRunResult(pr, "build-container", "IMAGE_URL")
			Expect(err).NotTo(HaveOccurred())
			Expect(i).To(Equal(image))

			// Specs now have a deterministic image reference for validation \o/
			imageWithDigest = fmt.Sprintf("%s@%s", image, digest)

			GinkgoWriter.Printf("The image signed by Tekton Chains is %s\n", imageWithDigest)
		})

		It("creates signature and attestation", func() {
			err := kubeController.AwaitAttestationAndSignature(imageWithDigest, attestationTimeout)
			Expect(err).NotTo(
				HaveOccurred(),
				"Could not find .att or .sig ImageStreamTags within the %s timeout. "+
					"Most likely the chains-controller did not create those in time. "+
					"Look at the chains-controller logs.",
				attestationTimeout.String(),
			)
			GinkgoWriter.Printf("Cosign verify pass with .att and .sig ImageStreamTags found for %s\n", imageWithDigest)
		})

		Context("verify-enterprise-contract task", func() {
			var generator tekton.VerifyEnterpriseContract
			var rekorHost string
			var verifyECTaskBundle string
			publicSecretName := "cosign-public-key"

			BeforeAll(func() {
				// Copy the public key from tekton-chains/signing-secrets to a new
				// secret that contains just the public key to ensure that access
				// to password and private key are not needed.
				publicKey, err := kubeController.GetPublicKey("signing-secrets", constants.TEKTON_CHAINS_NS)
				Expect(err).ToNot(HaveOccurred())
				GinkgoWriter.Printf("Copy public key from %s/signing-secrets to a new secret\n", constants.TEKTON_CHAINS_NS)
				Expect(kubeController.CreateOrUpdateSigningSecret(
					publicKey, publicSecretName, namespace)).To(Succeed())

				rekorHost, err = kubeController.GetRekorHost()
				Expect(err).ToNot(HaveOccurred())
				GinkgoWriter.Printf("Configured Rekor host: %s\n", rekorHost)

				cm, err := kubeController.Commonctrl.GetConfigMap("ec-defaults", "enterprise-contract-service")
				Expect(err).ToNot(HaveOccurred())
				verifyECTaskBundle = cm.Data["verify_ec_task_bundle"]
				Expect(verifyECTaskBundle).ToNot(BeEmpty())
				GinkgoWriter.Printf("Using verify EC task bundle: %s\n", verifyECTaskBundle)
			})

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
				// printPolicyConfiguration(baselinePolicies)
			})

			It("succeeds when policy is met", func() {
				pr, err := kubeController.RunPipeline(generator, pipelineRunTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())

				// Refresh our copy of the PipelineRun for latest results
				pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
				Expect(err).NotTo(HaveOccurred())

				tr, err := kubeController.GetTaskRunStatus(pr, "verify-enterprise-contract")
				Expect(err).NotTo(HaveOccurred())
				printTaskRunStatus(tr, namespace, *fwk.CommonController)
				GinkgoWriter.Printf("Make sure TaskRun %s of PipelineRun %s succeeded\n", tr.PipelineTaskName, pr.Name)
				Expect(tekton.DidTaskSucceed(tr)).To(BeTrue())
				GinkgoWriter.Printf("Make sure result for TaskRun %q succeeded\n", tr.PipelineTaskName)
				Expect(tr.Status.TaskRunResults).Should(ContainElements(
					tekton.MatchTaskRunResultWithJSONPathValue("HACBS_TEST_OUTPUT", "{$.result}", `["SUCCESS"]`),
				))
			})

			It("does not pass when tests are not satisfied on non-strict mode", func() {
				policy := ecp.EnterpriseContractPolicySpec{
					Sources: policySource,
					Configuration: &ecp.EnterpriseContractPolicyConfiguration{
						// The BuildahDemo pipeline used to generate the test data does not
						// include the required test tasks, so this policy should always fail.
						Include: []string{"test"},
					},
				}
				Expect(kubeController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(Succeed())
				// printPolicyConfiguration(policy)
				generator.Strict = false
				pr, err := kubeController.RunPipeline(generator, pipelineRunTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())

				// Refresh our copy of the PipelineRun for latest results
				pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
				Expect(err).NotTo(HaveOccurred())

				tr, err := kubeController.GetTaskRunStatus(pr, "verify-enterprise-contract")
				Expect(err).NotTo(HaveOccurred())

				printTaskRunStatus(tr, namespace, *fwk.CommonController)
				GinkgoWriter.Printf("Make sure TaskRun %s of PipelineRun %s succeeded\n", tr.PipelineTaskName, pr.Name)
				Expect(tekton.DidTaskSucceed(tr)).To(BeTrue())
				GinkgoWriter.Printf("Make sure result for TaskRun %q succeeded\n", tr.PipelineTaskName)
				Expect(tr.Status.TaskRunResults).Should(ContainElements(
					tekton.MatchTaskRunResultWithJSONPathValue("HACBS_TEST_OUTPUT", "{$.result}", `["FAILURE"]`),
				))
			})

			It("fails when tests are not satisfied on strict mode", func() {
				policy := ecp.EnterpriseContractPolicySpec{
					Sources: policySource,
					Configuration: &ecp.EnterpriseContractPolicyConfiguration{
						// The BuildahDemo pipeline used to generate the test data does not
						// include the required test tasks, so this policy should always fail.
						Include: []string{"test"},
					},
				}
				Expect(kubeController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(Succeed())
				// printPolicyConfiguration(policy)

				generator.Strict = true
				pr, err := kubeController.RunPipeline(generator, pipelineRunTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())

				// Refresh our copy of the PipelineRun for latest results
				pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
				Expect(err).NotTo(HaveOccurred())

				tr, err := kubeController.GetTaskRunStatus(pr, "verify-enterprise-contract")
				Expect(err).NotTo(HaveOccurred())

				printTaskRunStatus(tr, namespace, *fwk.CommonController)
				GinkgoWriter.Printf("Make sure TaskRun %s of PipelineRun %s failed\n", tr.PipelineTaskName, pr.Name)
				Expect(tekton.DidTaskSucceed(tr)).To(BeFalse())
				// Because the task fails, no results are created
			})

			It("fails when unexpected signature is used", func() {
				secretName := fmt.Sprintf("dummy-public-key-%s", util.GenerateRandomString(10))
				publicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
					"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAENZxkE/d0fKvJ51dXHQmxXaRMTtVz\n" +
					"BQWcmJD/7pcMDEmBcmk8O1yUPIiFj5TMZqabjS9CQQN+jKHG+Bfi0BYlHg==\n" +
					"-----END PUBLIC KEY-----")
				GinkgoWriter.Println("Create an unexpected public signing key")
				Expect(kubeController.CreateOrUpdateSigningSecret(publicKey, secretName, namespace)).To(Succeed())
				generator.PublicKey = fmt.Sprintf("k8s://%s/%s", namespace, secretName)

				pr, err := kubeController.RunPipeline(generator, pipelineRunTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())

				// Refresh our copy of the PipelineRun for latest results
				pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
				Expect(err).NotTo(HaveOccurred())

				tr, err := kubeController.GetTaskRunStatus(pr, "verify-enterprise-contract")
				Expect(err).NotTo(HaveOccurred())

				printTaskRunStatus(tr, namespace, *fwk.CommonController)
				GinkgoWriter.Printf("Make sure TaskRun %s of PipelineRun %s failed\n", tr.PipelineTaskName, pr.Name)
				Expect(tekton.DidTaskSucceed(tr)).To(BeFalse())
				// Because the task fails, no results are created
			})
		})
	})
})

// func printPolicyConfiguration(policy ecp.EnterpriseContractPolicySpec) {
// 	sources := ""
// 	for i, s := range policy.Sources {
// 		if i != 0 {
// 			sources += "\n"
// 		}
// 		if s.GitRepository != nil {
// 			if s.GitRepository.Revision != nil {
// 				sources += fmt.Sprintf("[%d] repository: '%s', revision: '%s'", i, s.GitRepository.Repository, *s.GitRepository.Revision)
// 			} else {
// 				sources += fmt.Sprintf("[%d] repository: '%s'", i, s.GitRepository.Repository)
// 			}
// 		}
// 	}
// 	exceptions := "[]"
// 	if policy.Exceptions != nil {
// 		exceptions = fmt.Sprintf("%v", policy.Exceptions.NonBlocking)
// 	}
// 	GinkgoWriter.Printf("Configured sources: %s\nand non-blocking policies: %v\n", sources, exceptions)
// }

func printTaskRunStatus(tr *v1beta1.PipelineRunTaskRunStatus, namespace string, sc common.SuiteController) {
	if tr.Status == nil {
		GinkgoWriter.Println("*** TaskRun status: nil")
		return
	}

	if y, err := yaml.Marshal(tr.Status); err == nil {
		GinkgoWriter.Printf("*** TaskRun status:\n%s\n", string(y))
	} else {
		GinkgoWriter.Printf("*** Unable to serialize TaskRunStatus to YAML: %#v; error: %s\n", tr.Status, err)
	}

	for _, s := range tr.Status.TaskRunStatusFields.Steps {
		if logs, err := sc.GetContainerLogs(tr.Status.PodName, s.ContainerName, namespace); err == nil {
			GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, s.ContainerName, logs)
		} else {
			GinkgoWriter.Printf("*** Can't fetch logs from pod '%s', container '%s': %s\n", tr.Status.PodName, s.ContainerName, err)
		}
	}
}
