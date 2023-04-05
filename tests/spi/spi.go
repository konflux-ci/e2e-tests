package spi

import (
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

const ()

var ()

/*
 * Component: spi
 * Description: Contains tests covering basic spi scenarios
 */

var _ = framework.SPISuiteDescribe(Label("spi-suite"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
		Expect(err).NotTo(HaveOccurred())
		namespace = fw.UserNamespace
		Expect(namespace).NotTo(BeEmpty())
	})

	AfterAll(func() {
		Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
	})

	Describe("SVPI-398 - Token upload rest endpoint", func() {

		var SPITokenBinding *v1beta1.SPIAccessTokenBinding

		It("create SPITokenBinding", func() {
			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding("my-binding-", namespace, "https://github.com/albarbaro/devfile-sample-python-basic-private", "secretn-name", "secretType")
			Expect(err).NotTo(HaveOccurred())
		})

		It("SPITokenBinding to be in AwaitingTokenData phase", func() {
			Eventually(func() bool {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)

				if err != nil {
					return false
				}

				return (SPITokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData)
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "SPIAccessTokenBinding is not in AwaitingTokenData phase")
		})

		It("upload username and token using rest endpoint ", func() {
			Eventually(func() bool {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)

				if err != nil {
					return false
				}

				return SPITokenBinding.Status.UploadUrl != ""
			}, 1*time.Minute, 10*time.Second).Should(BeTrue(), "uploadUrl not set")

			Expect(err).NotTo(HaveOccurred())

			linkedAccessTokenName := SPITokenBinding.Status.LinkedAccessTokenName
			Expect(linkedAccessTokenName).NotTo(BeEmpty())

			uploadURL := SPITokenBinding.Status.UploadUrl
			Expect(uploadURL).NotTo(BeEmpty())

			// Get the token for the current openshift user
			bearerToken, err := utils.GetOpenshiftToken()

			Expect(err).NotTo(HaveOccurred())

			oauthCredentials := `{"access_token":"` + utils.GetEnv("GITHUB_TOKEN", "") + `", "username":"` + utils.GetEnv("USER", "albarbaro") + `"}`

			statusCode, err := fw.AsKubeDeveloper.SPIController.UploadWithRestEndpoint(uploadURL, oauthCredentials, bearerToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).Should(Equal(204))

		})

		It("SPITokenBinding to be in Injected phase", func() {

			Eventually(func() bool {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())
				return SPITokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseInjected
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "SPIAccessTokenBinding is not in Injected phase")

		})
	})

})
