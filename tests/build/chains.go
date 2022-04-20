package build

import (
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.ChainsSuiteDescribe("Tekton Chains E2E tests", func() {
	defer g.GinkgoRecover()

	// Initialize the tests controllers
	framework, err := framework.NewFramweork()
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
		image := "image-registry.openshift-image-registry.svc:5000/tekton-chains/buildah-demo"
		taskTimeout := 180
		attestationTimeout := time.Duration(60) * time.Second
		kubeController := tekton.KubeController{
			Commonctrl: *framework.CommonController,
			Tektonctrl: *framework.TektonController,
			Namespace:  constants.TEKTON_CHAINS_NS,
		}
		// create a task, get the pod that it's running in, wait for the pod to finish, then verify it was successful
		g.It("run demo tasks", func() {
			tr, waitTrErr := kubeController.RunBuildahDemoTask(image, taskTimeout)
			Expect(waitTrErr).NotTo(HaveOccurred())
			waitErr := kubeController.WatchTaskPod(tr.Name, taskTimeout)
			Expect(waitErr).NotTo(HaveOccurred())
		})
		g.It("creates signature and attestation", func() {
			err := kubeController.AwaitAttestationAndSignature(image, attestationTimeout)
			Expect(err).NotTo(HaveOccurred(), "Could not find .att or .sig ImageStreamTags within the %s timeout. Most likely the chains-controller did not create those in time. Look at the chains-controller and buildah task logs.", attestationTimeout.String())
		})
		g.It("verify image attestation", func() {
			tr, waitTrErr := kubeController.RunVerifyTask("cosign-verify-attestation", image, taskTimeout)
			Expect(waitTrErr).NotTo(HaveOccurred())
			waitErr := kubeController.WatchTaskPod(tr.Name, taskTimeout)
			Expect(waitErr).NotTo(HaveOccurred())
		})
		g.It("cosign verify", func() {
			tr, waitTrErr := kubeController.RunVerifyTask("cosign-verify", image, taskTimeout)
			Expect(waitTrErr).NotTo(HaveOccurred())
			waitErr := kubeController.WatchTaskPod(tr.Name, taskTimeout)
			Expect(waitErr).NotTo(HaveOccurred())
		})
	})
})
