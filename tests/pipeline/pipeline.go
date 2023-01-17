package pipeline

/* This was generated from a template file. Please feel free to update as necessary */

import (
	"fmt"
	"time"

	"github.com/devfile/library/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

var _ = framework.PipelineSuiteDescribe("Pipeline E2E tests", Label("pipeline"), func() {

	defer GinkgoRecover()
	var fwk *framework.Framework
	// use 'fwk' to access common controllers or the specific service controllers within the framework
	BeforeAll(func() {
		// Initialize the tests controllers
		var err error
		fwk, err = framework.NewFramework()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Trigger PipelineRun directly by calling Pipeline-service", Label("pipeline"), func() {
		// Declare variables here.
		namespace := constants.PIPELINE_SERVICE_E2E_NS
		buildPipelineRunName := fmt.Sprintf("buildah-demo-%s", util.GenerateRandomString(10))
		image := fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/%s/%s", namespace, buildPipelineRunName)
		var imageWithDigest string
		serviceAccountName := "pipeline"

		pipelineRunTimeout := 600
		attestationTimeout := time.Duration(60) * time.Second

		var kubeController tekton.KubeController

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

			// At a bare minimum, each spec within this context relies on the existence of
			// an image that has been signed by Tekton Chains. Trigger a demo task to fulfill
			// this purpose.
			pr, err := kubeController.RunPipeline(tekton.BuildahDemo{Image: image, Bundle: fwk.TektonController.Bundles.BuildTemplatesBundle}, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			// Verify that the pipelinerun is executed as expected.
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

		AfterAll(func() {
			// Do cleanup only in case the test succeeded
			if !CurrentSpecReport().Failed() {
				Expect(fwk.TektonController.DeleteAllPipelineRunsInASpecificNamespace(namespace)).To(Succeed())
				Expect(fwk.CommonController.DeleteNamespace(namespace)).To(Succeed())
			}
		})

		Context("Test Tekton Chanin", func() {
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
		})
	})
})
