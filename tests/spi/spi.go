package spi

import (
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

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

	Describe("SVPI-398 - Token upload rest endpoint", Ordered, func() {

		// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokenDataInASpecificNamespace(namespace)).To(Succeed())
			}
		})

		var SPITokenBinding *v1beta1.SPIAccessTokenBinding
		tokenBindingName := "spi-token-binding-rest-"

		It("create SPITokenBinding", func() {
			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(tokenBindingName, namespace, repoURL, "", "kubernetes.io/basic-auth")
			Expect(err).NotTo(HaveOccurred())
		})

		It("SPITokenBinding to be in AwaitingTokenData phase", func() {
			// wait SPITokenBinding to be in AwaitingTokenData phase before trying to upload a token
			Eventually(func() bool {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)

				if err != nil {
					return false
				}

				return (SPITokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData)
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "SPIAccessTokenBinding is not in AwaitingTokenData phase")
		})

		It("upload username and token using rest endpoint ", func() {
			// the UploadUrl in SPITokenBinding should be available before uploading the token
			Eventually(func() bool {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)

				if err != nil {
					return false
				}

				return SPITokenBinding.Status.UploadUrl != ""
			}, 1*time.Minute, 10*time.Second).Should(BeTrue(), "uploadUrl not set")
			Expect(err).NotTo(HaveOccurred())

			// linked accessToken token should exsist
			linkedAccessTokenName := SPITokenBinding.Status.LinkedAccessTokenName
			Expect(linkedAccessTokenName).NotTo(BeEmpty())

			// get the url to manually upload the token
			uploadURL := SPITokenBinding.Status.UploadUrl
			Expect(uploadURL).NotTo(BeEmpty())

			// Get the token for the current openshift user
			bearerToken, err := utils.GetOpenshiftToken()
			Expect(err).NotTo(HaveOccurred())

			// build and upload the payload using the uploadURL. it should return 204
			oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`
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

		It("SPIAccessToken exists and is in Read phase", func() {
			Eventually(func() bool {
				SPIAccessToken, err := fw.AsKubeDeveloper.SPIController.GetSPIAccessToken(SPITokenBinding.Status.LinkedAccessTokenName, namespace)

				if err != nil {
					return false
				}

				return (SPIAccessToken.Status.Phase == v1beta1.SPIAccessTokenPhaseReady)
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "SPIAccessToken should be in ready phase")

		})
	})

	Describe("SVPI-399 - Upload token with k8s secret (associate it to existing SPIAccessToken)", Ordered, func() {

		// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
		AfterAll(func() {
			Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
			Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(namespace)).To(Succeed())
			Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokenDataInASpecificNamespace(namespace)).To(Succeed())
		})

		// create a new SPITokenBinding and get the generated SPIAccessToken; we will associate the secret to it
		var SPITokenBinding *v1beta1.SPIAccessTokenBinding
		var K8sSecret *v1.Secret
		secretName := "access-token-binding-k8s-secret"
		tokenBindingName := "spi-token-binding-k8s-"

		It("create SPITokenBinding", func() {
			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(tokenBindingName, namespace, repoURL, "", "kubernetes.io/basic-auth")
			Expect(err).NotTo(HaveOccurred())
		})

		It("create secret with access token and associate it to an existing SPIAccessToken", func() {
			Eventually(func() bool {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)

				if err != nil {
					return false
				}

				return (SPITokenBinding.Status.LinkedAccessTokenName != "")
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "LinkedAccessTokenName should not be empty")

			linkedAccessTokenName := SPITokenBinding.Status.LinkedAccessTokenName
			tokenData := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
			Expect(tokenData).NotTo(BeEmpty())

			K8sSecret, err = fw.AsKubeDeveloper.SPIController.UploadWithK8sSecret(secretName, namespace, linkedAccessTokenName, repoURL, "", tokenData)
			Expect(err).NotTo(HaveOccurred())
		})

		It("SPITokenBinding should be in Injected phase", func() {

			Eventually(func() bool {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				if err != nil {
					return false
				}
				return SPITokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseInjected
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "SPIAccessTokenBinding is not in Injected phase")

		})

		It("upload secret should be automatically be removed", func() {
			_, err := fw.AsKubeDeveloper.CommonController.GetSecret(namespace, K8sSecret.Name)
			Expect(k8sErrors.IsNotFound(err)).To(BeTrue())
		})

		It("SPIAccessToken exists and is in Read phase", func() {
			Eventually(func() bool {
				SPIAccessToken, err := fw.AsKubeDeveloper.SPIController.GetSPIAccessToken(SPITokenBinding.Status.LinkedAccessTokenName, namespace)

				if err != nil {
					return false
				}

				return (SPIAccessToken.Status.Phase == v1beta1.SPIAccessTokenPhaseReady)
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "SPIAccessToken should be in ready phase")

		})
	})

	Describe("SVPI-399 - Upload token with k8s secret (create new SPIAccessToken automatically)", Ordered, func() {

		// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokenDataInASpecificNamespace(namespace)).To(Succeed())
			}
		})

		// we create a secret specifying a non-existing SPIAccessToken name: it should be created automatically by SPI
		var K8sSecret *v1.Secret
		secretName := "access-token-k8s-secret"
		nonExistingAccessTokenName := "new-access-token-k8s"

		It("create secret with access token and associate it to an existing SPIAccessToken", func() {

			tokenData := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
			Expect(tokenData).NotTo(BeEmpty())

			K8sSecret, err = fw.AsKubeDeveloper.SPIController.UploadWithK8sSecret(secretName, namespace, nonExistingAccessTokenName, repoURL, "", tokenData)
			Expect(err).NotTo(HaveOccurred())
		})

		It("upload secret should be autometically be removed", func() {
			_, err := fw.AsKubeDeveloper.CommonController.GetSecret(namespace, K8sSecret.Name)
			Expect(k8sErrors.IsNotFound(err)).To(BeTrue())
		})

		It("SPIAccessToken exists and is in Read phase", func() {
			Eventually(func() bool {
				SPIAccessToken, err := fw.AsKubeDeveloper.SPIController.GetSPIAccessToken(nonExistingAccessTokenName, namespace)

				if err != nil {
					return false
				}

				return (SPIAccessToken.Status.Phase == v1beta1.SPIAccessTokenPhaseReady)
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "SPIAccessToken should be in ready phase")

		})
	})

})
