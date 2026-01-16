package enterprisecontract

import (
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/conforma/crds/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/common"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/contract"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/yaml"
)

var _ = framework.EnterpriseContractSuiteDescribe("Conforma E2E tests", ginkgo.Label("ec"), func() {

	defer ginkgo.GinkgoRecover()

	var namespace string
	var fwk *framework.Framework
	var tektonChainsNs = constants.TEKTON_CHAINS_NS

	ginkgo.AfterEach(framework.ReportFailure(&fwk))

	ginkgo.BeforeAll(func() {
		var err error
		fwk, err = framework.NewFramework(utils.GetGeneratedNamespace(constants.TEKTON_CHAINS_E2E_USER))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(fwk.UserNamespace).NotTo(gomega.BeEmpty(), "failed to create sandbox user")
		namespace = fwk.UserNamespace

		if os.Getenv(constants.TEST_ENVIRONMENT_ENV) == constants.UpstreamTestEnvironment {
			tektonChainsNs = "tekton-pipelines"
		}
	})

	ginkgo.Context("infrastructure is running", ginkgo.Label("pipeline"), func() {
		ginkgo.It("verifies if the chains controller is running", func() {
			err := fwk.AsKubeAdmin.CommonController.WaitForPodSelector(fwk.AsKubeAdmin.CommonController.IsPodRunning, tektonChainsNs, "app", "tekton-chains-controller", 60, 100)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("verifies the signing secret is present", func() {
			timeout := time.Minute * 5
			interval := time.Second * 1

			gomega.Eventually(func() bool {
				config, err := fwk.AsKubeAdmin.CommonController.GetSecret(tektonChainsNs, constants.TEKTON_CHAINS_SIGNING_SECRETS_NAME)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				_, private := config.Data["cosign.key"]
				_, public := config.Data["cosign.pub"]
				_, password := config.Data["cosign.password"]

				return private && public && password
			}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for Tekton Chains signing secret %q to be present in %q namespace", constants.TEKTON_CHAINS_SIGNING_SECRETS_NAME, tektonChainsNs))
		})
	})

	ginkgo.Context("test creating and signing an image and task", ginkgo.Label("pipeline"), func() {
		// Make the PipelineRun name and namespace predictable. For convenience, the name of the
		// PipelineRun that builds an image, is the same as the repository where the image is
		// pushed to.
		var buildPipelineRunName, image, imageWithDigest string
		var pipelineRunTimeout int
		var defaultECP *ecp.EnterpriseContractPolicy

		ginkgo.BeforeAll(func() {
			buildPipelineRunName = fmt.Sprintf("buildah-demo-%s", util.GenerateRandomString(10))
			image = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), buildPipelineRunName)

			gomega.Expect(fwk.AsKubeAdmin.CommonController.CreateQuayRegistrySecret(namespace)).To(gomega.Succeed())

			pipelineRunTimeout = int(time.Duration(20) * time.Minute)

			var err error
			defaultECP, err = fwk.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Trigger a demo task to generate an image that has been signed by Tekton Chains within the context for each spec
			bundles, err := fwk.AsKubeAdmin.TektonController.NewBundles()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			dockerBuildBundle := bundles.DockerBuildBundle
			gomega.Expect(dockerBuildBundle).NotTo(gomega.Equal(""), "Can't continue without a docker-build pipeline got from selector config")
			pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(tekton.BuildahDemo{Image: image, Bundle: dockerBuildBundle, Namespace: namespace, Name: buildPipelineRunName}, namespace, pipelineRunTimeout)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Verify that the build task was created as expected.
			gomega.Expect(pr.Name).To(gomega.Equal(buildPipelineRunName))
			gomega.Expect(pr.Namespace).To(gomega.Equal(namespace))
			gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())
			ginkgo.GinkgoWriter.Printf("The pipeline named %q in namespace %q succeeded\n", pr.Name, pr.Namespace)

			// The PipelineRun resource has been updated, refresh our reference.
			pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify TaskRun has the type hinting required by Tekton Chains
			digest, err := fwk.AsKubeAdmin.TektonController.GetTaskRunResult(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "build-container", "IMAGE_DIGEST")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			i, err := fwk.AsKubeAdmin.TektonController.GetTaskRunResult(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "build-container", "IMAGE_URL")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(i).To(gomega.Equal(image))

			// Specs now have a deterministic image reference for validation \o/
			imageWithDigest = fmt.Sprintf("%s@%s", image, digest)

			ginkgo.GinkgoWriter.Printf("The image signed by Tekton Chains is %s\n", imageWithDigest)
		})

		ginkgo.It("creates signature and attestation", func() {
			err := fwk.AsKubeAdmin.TektonController.AwaitAttestationAndSignature(imageWithDigest, constants.ChainsAttestationTimeout)
			gomega.Expect(err).NotTo(
				gomega.HaveOccurred(),
				"Could not find .att or .sig ImageStreamTags within the %s timeout. "+
					"Most likely the chains-controller did not create those in time. "+
					"Look at the chains-controller logs.",
				constants.ChainsAttestationTimeout.String(),
			)
			ginkgo.GinkgoWriter.Printf("Cosign verify pass with .att and .sig ImageStreamTags found for %s\n", imageWithDigest)
		})

		ginkgo.Context("verify-enterprise-contract task", func() {
			var generator tekton.VerifyEnterpriseContract
			var rekorHost string
			var verifyECTaskBundle string
			publicSecretName := "cosign-public-key"

			ginkgo.BeforeAll(func() {
				// Copy the public key from openshift-pipelines/signing-secrets to a new
				// secret that contains just the public key to ensure that access
				// to password and private key are not needed.
				publicKey, err := fwk.AsKubeAdmin.TektonController.GetTektonChainsPublicKey()
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				ginkgo.GinkgoWriter.Printf("Copy public key from %s/signing-secrets to a new secret\n", tektonChainsNs)
				gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(
					publicKey, publicSecretName, namespace)).To(gomega.Succeed())

				rekorHost, err = fwk.AsKubeAdmin.TektonController.GetRekorHost()
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				ginkgo.GinkgoWriter.Printf("Configured Rekor host: %s\n", rekorHost)

				cm, err := fwk.AsKubeAdmin.CommonController.GetConfigMap("ec-defaults", "enterprise-contract-service")
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				verifyECTaskBundle = cm.Data["verify_ec_task_bundle"]
				gomega.Expect(verifyECTaskBundle).ToNot(gomega.BeEmpty())
				ginkgo.GinkgoWriter.Printf("Using verify EC task bundle: %s\n", verifyECTaskBundle)
			})

			ginkgo.BeforeEach(func() {
				generator = tekton.VerifyEnterpriseContract{
					TaskBundle:          verifyECTaskBundle,
					Name:                "verify-enterprise-contract",
					Namespace:           namespace,
					PolicyConfiguration: "ec-policy",
					PublicKey:           fmt.Sprintf("k8s://%s/%s", namespace, publicSecretName),
					Strict:              true,
					EffectiveTime:       "now",
					IgnoreRekor:         true,
				}
				generator.WithComponentImage(imageWithDigest)

				// Since specs could update the config policy, make sure it has a consistent
				// baseline at the start of each spec.
				baselinePolicies := contract.PolicySpecWithSourceConfig(
					// A simple policy that should always succeed in a cluster where
					// Tekton Chains is properly setup.
					defaultECP.Spec, ecp.SourceConfig{Include: []string{"slsa_provenance_available"}})
				gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, baselinePolicies)).To(gomega.Succeed())
				// printPolicyConfiguration(baselinePolicies)
			})

			ginkgo.It("succeeds when policy is met", func() {
				pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

				// Refresh our copy of the PipelineRun for latest results
				pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				printTaskRunStatus(tr, namespace, *fwk.AsKubeAdmin.CommonController)
				ginkgo.GinkgoWriter.Printf("Make sure TaskRun %s of PipelineRun %s succeeded\n", tr.PipelineTaskName, pr.Name)
				gomega.Expect(tekton.DidTaskRunSucceed(tr)).To(gomega.BeTrue())
				ginkgo.GinkgoWriter.Printf("Make sure result for TaskRun %q succeeded\n", tr.PipelineTaskName)
				gomega.Expect(tr.Status.Results).Should(gomega.Or(
					gomega.ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["SUCCESS"]`)),
				))
			})

			ginkgo.It("does not pass when tests are not satisfied on non-strict mode", func() {
				policy := contract.PolicySpecWithSourceConfig(
					// The BuildahDemo pipeline used to generate the test data does not
					// include the required test tasks, so this policy should always fail.
					defaultECP.Spec, ecp.SourceConfig{Include: []string{"test"}})
				gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(gomega.Succeed())
				// printPolicyConfiguration(policy)
				generator.Strict = false
				pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

				// Refresh our copy of the PipelineRun for latest results
				pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				printTaskRunStatus(tr, namespace, *fwk.AsKubeAdmin.CommonController)
				ginkgo.GinkgoWriter.Printf("Make sure TaskRun %s of PipelineRun %s succeeded\n", tr.PipelineTaskName, pr.Name)
				gomega.Expect(tekton.DidTaskRunSucceed(tr)).To(gomega.BeTrue())
				ginkgo.GinkgoWriter.Printf("Make sure result for TaskRun %q succeeded\n", tr.PipelineTaskName)
				gomega.Expect(tr.Status.Results).Should(gomega.Or(
					gomega.ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
				))
			})

			ginkgo.It("fails when tests are not satisfied on strict mode", func() {
				policy := contract.PolicySpecWithSourceConfig(
					// The BuildahDemo pipeline used to generate the test data does not
					// include the required test tasks, so this policy should always fail.
					defaultECP.Spec, ecp.SourceConfig{Include: []string{"test"}})
				gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(gomega.Succeed())
				// printPolicyConfiguration(policy)

				generator.Strict = true
				pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

				// Refresh our copy of the PipelineRun for latest results
				pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				printTaskRunStatus(tr, namespace, *fwk.AsKubeAdmin.CommonController)
				ginkgo.GinkgoWriter.Printf("Make sure TaskRun %s of PipelineRun %s failed\n", tr.PipelineTaskName, pr.Name)
				gomega.Expect(tekton.DidTaskRunSucceed(tr)).To(gomega.BeFalse())
				// Because the task fails, no results are created
			})

			ginkgo.It("fails when unexpected signature is used", func() {
				secretName := fmt.Sprintf("dummy-public-key-%s", util.GenerateRandomString(10))
				publicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
					"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAENZxkE/d0fKvJ51dXHQmxXaRMTtVz\n" +
					"BQWcmJD/7pcMDEmBcmk8O1yUPIiFj5TMZqabjS9CQQN+jKHG+Bfi0BYlHg==\n" +
					"-----END PUBLIC KEY-----")
				ginkgo.GinkgoWriter.Println("Create an unexpected public signing key")
				gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(publicKey, secretName, namespace)).To(gomega.Succeed())
				generator.PublicKey = fmt.Sprintf("k8s://%s/%s", namespace, secretName)

				pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

				// Refresh our copy of the PipelineRun for latest results
				pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				printTaskRunStatus(tr, namespace, *fwk.AsKubeAdmin.CommonController)
				ginkgo.GinkgoWriter.Printf("Make sure TaskRun %s of PipelineRun %s failed\n", tr.PipelineTaskName, pr.Name)
				gomega.Expect(tekton.DidTaskRunSucceed(tr)).To(gomega.BeFalse())
				// Because the task fails, no results are created
			})

			ginkgo.Context("ec-cli command", func() {
				ginkgo.It("verifies ec cli has error handling", func() {
					generator.WithComponentImage("quay.io/konflux-ci/ec-golden-image:latest")
					pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

					pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					gomega.Expect(tr.Status.Results).Should(gomega.Or(
						gomega.ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
					))
					//Get container step-report-json log details from pod
					reportLog, err := utils.GetContainerLogs(fwk.AsKubeAdmin.CommonController.KubeInterface(), tr.Status.PodName, "step-report-json", namespace)
					ginkgo.GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report-json", reportLog)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(reportLog).Should(gomega.ContainSubstring("No image attestations found matching the given public key"))
				})

				ginkgo.It("verifies ec validate accepts a list of image references", func() {
					secretName := fmt.Sprintf("golden-image-public-key%s", util.GenerateRandomString(10))
					ginkgo.GinkgoWriter.Println("Update public key to verify golden images")
					goldenImagePublicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
						"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZP/0htjhVt2y0ohjgtIIgICOtQtA\n" +
						"naYJRuLprwIv6FDhZ5yFjYUEtsmoNcW7rx2KM6FOXGsCX3BNc7qhHELT+g==\n" +
						"-----END PUBLIC KEY-----")
					gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(goldenImagePublicKey, secretName, namespace)).To(gomega.Succeed())
					generator.PublicKey = fmt.Sprintf("k8s://%s/%s", namespace, secretName)

					policy := contract.PolicySpecWithSourceConfig(
						defaultECP.Spec,
						ecp.SourceConfig{
							Include: []string{"@slsa3"},
							// This test validates an image via a floating tag (as designed). This makes
							// it hard to provide the expected git commit. Here we just ignore that
							// particular check.
							Exclude: []string{"slsa_source_correlated.source_code_reference_provided"},
						})
					gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(gomega.Succeed())

					generator.WithComponentImage("quay.io/konflux-ci/ec-golden-image:latest")
					generator.AppendComponentImage("quay.io/konflux-ci/ec-golden-image:e2e-test-unacceptable-task")
					pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

					pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					gomega.Expect(tr.Status.Results).Should(gomega.Or(
						gomega.ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["SUCCESS"]`)),
					))
					//Get container step-report-json log details from pod
					reportLog, err := utils.GetContainerLogs(fwk.AsKubeAdmin.CommonController.KubeInterface(), tr.Status.PodName, "step-report-json", namespace)
					ginkgo.GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report-json", reportLog)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})
			})

			ginkgo.Context("Release Policy", func() {
				ginkgo.It("verifies redhat products pass the redhat policy rule collection before release ", func() {
					secretName := fmt.Sprintf("golden-image-public-key%s", util.GenerateRandomString(10))
					ginkgo.GinkgoWriter.Println("Update public key to verify golden images")
					goldenImagePublicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
						"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZP/0htjhVt2y0ohjgtIIgICOtQtA\n" +
						"naYJRuLprwIv6FDhZ5yFjYUEtsmoNcW7rx2KM6FOXGsCX3BNc7qhHELT+g==\n" +
						"-----END PUBLIC KEY-----")
					gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(goldenImagePublicKey, secretName, namespace)).To(gomega.Succeed())
					redhatECP, error := fwk.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("redhat", "enterprise-contract-service")
					gomega.Expect(error).NotTo(gomega.HaveOccurred())
					generator.PublicKey = fmt.Sprintf("k8s://%s/%s", namespace, secretName)
					policy := contract.PolicySpecWithSourceConfig(
						redhatECP.Spec,
						ecp.SourceConfig{
							Include: []string{"@redhat"},
							// This test validates an image via a floating tag (as designed). This makes
							// it hard to provide the expected git commit. Here we just ignore that
							// particular check.
							Exclude: []string{"slsa_source_correlated.source_code_reference_provided", "cve.cve_results_found"},
						})
					gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(gomega.Succeed())

					generator.WithComponentImage("quay.io/konflux-ci/ec-golden-image:latest")
					pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

					pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					gomega.Expect(tr.Status.Results).ShouldNot(gomega.Or(
						gomega.ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
					))
				})
				ginkgo.It("verifies the release policy: Task are trusted", func() {
					secretName := fmt.Sprintf("golden-image-public-key%s", util.GenerateRandomString(10))
					ginkgo.GinkgoWriter.Println("Update public key to verify golden images")
					goldenImagePublicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
						"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEZP/0htjhVt2y0ohjgtIIgICOtQtA\n" +
						"naYJRuLprwIv6FDhZ5yFjYUEtsmoNcW7rx2KM6FOXGsCX3BNc7qhHELT+g==\n" +
						"-----END PUBLIC KEY-----")
					gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(goldenImagePublicKey, secretName, namespace)).To(gomega.Succeed())
					generator.PublicKey = fmt.Sprintf("k8s://%s/%s", namespace, secretName)
					policy := contract.PolicySpecWithSourceConfig(
						defaultECP.Spec,
						ecp.SourceConfig{Include: []string{
							// Account for "acceptable" to "trusted" renaming. gomega.Eventually remove "acceptable" from this list
							"trusted_task.trusted",
						}},
					)
					gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(gomega.Succeed())

					generator.WithComponentImage("quay.io/konflux-ci/ec-golden-image:e2e-test-unacceptable-task")
					pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

					pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					gomega.Expect(tr.Status.Results).Should(gomega.Or(
						gomega.ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["FAILURE"]`)),
					))

					//Get container step-report-json log details from pod
					reportLog, err := utils.GetContainerLogs(fwk.AsKubeAdmin.CommonController.KubeInterface(), tr.Status.PodName, "step-report-json", namespace)
					ginkgo.GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report-json", reportLog)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(reportLog).Should(gomega.MatchRegexp(`PipelineTask .* uses an untrusted task reference`))
				})

				ginkgo.It("verifies the release policy: Task references are pinned", func() {
					secretName := fmt.Sprintf("unpinned-task-bundle-public-key%s", util.GenerateRandomString(10))
					unpinnedTaskPublicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
						"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEPfwkY/ru2JRd6FSqIp7lT3gzjaEC\n" +
						"EAg+paWtlme2KNcostCsmIbwz+bc2aFV+AxCOpRjRpp3vYrbS5KhkmgC1Q==\n" +
						"-----END PUBLIC KEY-----")
					ginkgo.GinkgoWriter.Println("Update public <key to verify unpinned task image")
					gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(unpinnedTaskPublicKey, secretName, namespace)).To(gomega.Succeed())
					generator.PublicKey = fmt.Sprintf("k8s://%s/%s", namespace, secretName)

					policy := contract.PolicySpecWithSourceConfig(
						defaultECP.Spec,
						ecp.SourceConfig{Include: []string{"trusted_task.pinned"}},
					)
					gomega.Expect(fwk.AsKubeAdmin.TektonController.CreateOrUpdatePolicyConfiguration(namespace, policy)).To(gomega.Succeed())

					generator.WithComponentImage("quay.io/redhat-appstudio-qe/enterprise-contract-tests:e2e-test-unpinned-task-bundle")
					pr, err := fwk.AsKubeAdmin.TektonController.RunPipeline(generator, namespace, pipelineRunTimeout)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(fwk.AsKubeAdmin.TektonController.WatchPipelineRun(pr.Name, namespace, pipelineRunTimeout)).To(gomega.Succeed())

					pr, err = fwk.AsKubeAdmin.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					tr, err := fwk.AsKubeAdmin.TektonController.GetTaskRunStatus(fwk.AsKubeAdmin.CommonController.KubeRest(), pr, "verify-enterprise-contract")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					gomega.Expect(tr.Status.Results).Should(gomega.Or(
						gomega.ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["WARNING"]`)),
					))

					//Get container step-report-json log details from pod
					reportLog, err := utils.GetContainerLogs(fwk.AsKubeAdmin.CommonController.KubeInterface(), tr.Status.PodName, "step-report-json", namespace)
					ginkgo.GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, "step-report-json", reportLog)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(reportLog).Should(gomega.MatchRegexp(`Pipeline task .* uses an unpinned task reference`))
				})
			})
		})

	})
})

func printTaskRunStatus(tr *pipeline.PipelineRunTaskRunStatus, namespace string, sc common.SuiteController) {
	if tr.Status == nil {
		ginkgo.GinkgoWriter.Println("*** TaskRun status: nil")
		return
	}

	if y, err := yaml.Marshal(tr.Status); err == nil {
		ginkgo.GinkgoWriter.Printf("*** TaskRun status:\n%s\n", string(y))
	} else {
		ginkgo.GinkgoWriter.Printf("*** Unable to serialize TaskRunStatus to YAML: %#v; error: %s\n", tr.Status, err)
	}

	for _, s := range tr.Status.Steps {
		if logs, err := utils.GetContainerLogs(sc.KubeInterface(), tr.Status.PodName, s.Container, namespace); err == nil {
			ginkgo.GinkgoWriter.Printf("*** Logs from pod '%s', container '%s':\n----- START -----%s----- END -----\n", tr.Status.PodName, s.Container, logs)
		} else {
			ginkgo.GinkgoWriter.Printf("*** Can't fetch logs from pod '%s', container '%s': %s\n", tr.Status.PodName, s.Container, err)
		}
	}
}
