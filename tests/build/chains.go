package build

import (
	"fmt"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.ChainsSuiteDescribe("Tekton Chains E2E tests", func() {
	defer g.GinkgoRecover()

	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	g.Context("infrastructure is running", func() {
		g.It("verify the chains controller is running", func() {
			err := framework.CommonController.WaitForPodSelector(framework.CommonController.IsPodRunning, constants.TEKTON_CHAINS_NS, "app", "tekton-chains-controller", 60, 100)
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("verify the correct secrets have been created", func() {
			_, err := framework.CommonController.GetSecret(constants.TEKTON_CHAINS_NS, "chains-ca-cert")
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("verify the correct roles are created", func() {
			_, csaErr := framework.CommonController.GetRole("chains-secret-admin", constants.TEKTON_CHAINS_NS)
			Expect(csaErr).NotTo(HaveOccurred())
			_, srErr := framework.CommonController.GetRole("secret-reader", "openshift-ingress-operator")
			Expect(srErr).NotTo(HaveOccurred())
		})
		g.It("verify the correct rolebindings are created", func() {
			_, csaErr := framework.CommonController.GetRoleBinding("chains-secret-admin", constants.TEKTON_CHAINS_NS)
			Expect(csaErr).NotTo(HaveOccurred())
			_, csrErr := framework.CommonController.GetRoleBinding("chains-secret-reader", "openshift-ingress-operator")
			Expect(csrErr).NotTo(HaveOccurred())
		})
		g.It("verify the correct service account is created", func() {
			_, err := framework.CommonController.GetServiceAccount("chains-secrets-admin", constants.TEKTON_CHAINS_NS)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	g.Context("test creating and signing an image and task", func() {
		// Make the TaskRun name and namespace predictable. For convenience, the name of the
		// TaskRun that builds an image, is the same as the repository where the image is
		// pushed to.
		namespace := "tekton-chains"
		buildTaskRunName := fmt.Sprintf("buildah-demo-%s", util.GenerateRandomString(10))
		image := fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/%s/%s", namespace, buildTaskRunName)

		taskTimeout := 180
		attestationTimeout := time.Duration(60) * time.Second
		kubeController := tekton.KubeController{
			Commonctrl: *framework.CommonController,
			Tektonctrl: *framework.TektonController,
			Namespace:  constants.TEKTON_CHAINS_NS,
		}

		var imageWithDigest string

		g.BeforeAll(func() {
			// At a bare minimum, each spec within this context relies on the existence of
			// an image that has been signed by Tekton Chains. Trigger a demo task to fulfill
			// this purpose.
			tr, err := kubeController.RunBuildahDemoTask(image, taskTimeout)
			Expect(err).NotTo(HaveOccurred())
			// Verify that the build task was created as expected.
			Expect(buildTaskRunName).To(Equal(tr.ObjectMeta.Name))
			Expect(namespace).To(Equal(tr.ObjectMeta.Namespace))
			Expect(kubeController.WatchTaskPod(tr.Name, taskTimeout)).To(Succeed())

			// The TaskRun resource has been updated, refresh our reference.
			tr, err = kubeController.Tektonctrl.GetTaskRun(tr.ObjectMeta.Name, tr.ObjectMeta.Namespace)
			Expect(err).NotTo(HaveOccurred())

			// Verify TaskRun has the type hinting required by Tekton Chains
			digest, err := kubeController.GetTaskRunResult(tr, "IMAGE_DIGEST")
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("%s", err))
			Expect(kubeController.GetTaskRunResult(tr, "IMAGE_URL")).To(Equal(image))

			// Specs now have a deterministic image reference for validation \o/
			imageWithDigest = fmt.Sprintf("%s@%s", image, digest)
		})

		g.It("creates signature and attestation", func() {
			err = kubeController.AwaitAttestationAndSignature(imageWithDigest, attestationTimeout)
			Expect(err).NotTo(
				HaveOccurred(),
				"Could not find .att or .sig ImageStreamTags within the %s timeout. "+
					"Most likely the chains-controller did not create those in time. "+
					"Look at the chains-controller logs.",
				attestationTimeout.String(),
			)
		})
		g.It("verify image attestation", func() {
			tr, waitTrErr := kubeController.RunVerifyTask("cosign-verify-attestation", imageWithDigest, taskTimeout)
			Expect(waitTrErr).NotTo(HaveOccurred())
			waitErr := kubeController.WatchTaskPod(tr.Name, taskTimeout)
			Expect(waitErr).NotTo(HaveOccurred())
		})
		g.It("cosign verify", func() {
			tr, waitTrErr := kubeController.RunVerifyTask("cosign-verify", imageWithDigest, taskTimeout)
			Expect(waitTrErr).NotTo(HaveOccurred())
			waitErr := kubeController.WatchTaskPod(tr.Name, taskTimeout)
			Expect(waitErr).NotTo(HaveOccurred())
		})

		g.Context("verify-enterprise-contract task", func() {
			var taskParams tekton.VerifyECTaskParams
			var rekorHost string
			publicSecretName := "cosign-public-key"

			g.BeforeAll(func() {
				// Copy the public key from tekton-chains/signing-secrets to a new
				// secret that contains just the public key to ensure that access
				// to password and private key are not needed.
				publicKey, err := kubeController.GetPublicKey("signing-secrets", "tekton-chains")
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeController.CreateOrUpdateSigningSecret(
					publicKey, publicSecretName, namespace)).To(Succeed())

				rekorHost, err = kubeController.GetRekorHost()
				Expect(err).ToNot(HaveOccurred())
			})

			g.BeforeEach(func() {
				taskParams = tekton.VerifyECTaskParams{
					TaskName:     "verify-enterprise-contract",
					ImageRef:     imageWithDigest,
					PublicSecret: fmt.Sprintf("k8s://%s/%s", namespace, publicSecretName),
					PipelineName: "pipeline-run-that-does-not-exist",
					RekorHost:    rekorHost,
					SslCertDir:   "/var/run/secrets/kubernetes.io/serviceaccount",
					StrictPolicy: "1",
				}

				// Since specs could update the config policy, make sure it has a consistent
				// baseline at the start of each spec.
				Expect(kubeController.CreateOrUpdateConfigPolicy(
					namespace, `{"non_blocking_checks":["not_useful"]}`)).To(Succeed())
			})

			g.It("succeeds when policy is met", func() {
				// Setup a policy config to ignore the policy check for tests
				Expect(kubeController.CreateOrUpdateConfigPolicy(
					namespace, `{"non_blocking_checks":["not_useful", "test"]}`)).To(Succeed())
				tr, err := kubeController.RunVerifyECTask(taskParams, taskTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeController.WatchTaskPod(tr.Name, taskTimeout)).To(Succeed())

				// Refresh our copy of the TaskRun for latest results
				tr, err = kubeController.Tektonctrl.GetTaskRun(tr.Name, tr.Namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(tr.Status.TaskRunResults).To(Equal([]v1beta1.TaskRunResult{
					{Name: "OUTPUT", Value: "[]\n"},
					{Name: "PASSED", Value: "true\n"},
				}))
			})

			g.It("does not pass when tests are not satisfied on non-strict mode", func() {
				taskParams.StrictPolicy = "0"
				tr, err := kubeController.RunVerifyECTask(taskParams, taskTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeController.WatchTaskPod(tr.Name, taskTimeout)).To(Succeed())

				// Refresh our copy of the TaskRun for latest results
				tr, err = kubeController.Tektonctrl.GetTaskRun(tr.Name, tr.Namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(tr.Status.TaskRunResults).To(Equal([]v1beta1.TaskRunResult{
					{Name: "OUTPUT", Value: "[\n  {\n    \"msg\": \"Empty test data provided\"\n  }\n]\n"},
					{Name: "PASSED", Value: "false\n"},
				}))
			})

			g.It("fails when tests are not satisfied on strict mode", func() {
				tr, err := kubeController.RunVerifyECTask(taskParams, taskTimeout)
				Expect(err).NotTo(HaveOccurred())
				err = kubeController.WatchTaskPod(tr.Name, taskTimeout)
				Expect(err).To(HaveOccurred())

				// Refresh our copy of the TaskRun for latest results
				tr, err = kubeController.Tektonctrl.GetTaskRun(tr.Name, tr.Namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(tr.IsSuccessful()).To(BeFalse())
				// Because the task fails, no results are created
			})

			g.It("fails when unexpected signature is used", func() {
				secretName := fmt.Sprintf("dummy-public-key-%s", util.GenerateRandomString(10))
				publicKey := []byte("-----BEGIN PUBLIC KEY-----\n" +
					"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAENZxkE/d0fKvJ51dXHQmxXaRMTtVz\n" +
					"BQWcmJD/7pcMDEmBcmk8O1yUPIiFj5TMZqabjS9CQQN+jKHG+Bfi0BYlHg==\n" +
					"-----END PUBLIC KEY-----")
				Expect(kubeController.CreateOrUpdateSigningSecret(publicKey, secretName, namespace)).To(Succeed())
				taskParams.PublicSecret = fmt.Sprintf("k8s://%s/%s", namespace, secretName)

				tr, err := kubeController.RunVerifyECTask(taskParams, taskTimeout)
				Expect(err).NotTo(HaveOccurred())
				err = kubeController.WatchTaskPod(tr.Name, taskTimeout)
				Expect(err).To(HaveOccurred())

				// Refresh our copy of the TaskRun for latest results
				tr, err = kubeController.Tektonctrl.GetTaskRun(tr.Name, tr.Namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(tr.IsSuccessful()).To(BeFalse())
				// Because the task fails, no results are created
			})
		})

	})
})
