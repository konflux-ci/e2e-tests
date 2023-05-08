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
 * Description: SVPI-399 - Upload token with k8s secret

 * Test Scenario 1: Upload token with k8s secret (associate it to existing SPIAccessToken)
 * Test Scenario 2: Upload token with k8s secret (create new SPIAccessToken automatically)

  * Flow of Test Scenario 1:
	* 1º - creates SPITokenBinding
	* 2º - creates secret with access token and associate it to an existing SPIAccessToken
	* 3º - SPITokenBinding should be in Injected phase
	* 4º - upload secret should be automatically be removed
	* 5º - SPIAccessToken exists and is in Read phase

  * Flow of Test Scenario 2:
	* 1º - creates secret with access token and associate it to an existing SPIAccessToken
	* 2º - upload secret should be automatically be removed
	* 3º - SPIAccessToken exists and is in Read phase
*/

var _ = framework.SPISuiteDescribe(Label("spi-suite", "token-upload-k8s"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string

	Describe("SVPI-399 - Upload token with k8s secret (associate it to existing SPIAccessToken)", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())
		})

		// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokenDataInASpecificNamespace(namespace)).To(Succeed())
			}
		})

		// create a new SPITokenBinding and get the generated SPIAccessToken; we will associate the secret to it
		var SPITokenBinding *v1beta1.SPIAccessTokenBinding
		var K8sSecret *v1.Secret
		secretName := "access-token-binding-k8s-secret"
		tokenBindingName := "spi-token-binding-k8s-"

		It("creates SPITokenBinding", func() {
			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(tokenBindingName, namespace, RepoURL, "", "kubernetes.io/basic-auth")
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates secret with access token and associate it to an existing SPIAccessToken", func() {
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

			K8sSecret, err = fw.AsKubeDeveloper.SPIController.UploadWithK8sSecret(secretName, namespace, linkedAccessTokenName, RepoURL, "", tokenData)
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
			Eventually(func() bool {
				_, err := fw.AsKubeDeveloper.CommonController.GetSecret(namespace, K8sSecret.Name)

				if err == nil {
					return false
				}

				return k8sErrors.IsNotFound(err)
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "upload secret not removed")
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
		BeforeAll(func() {
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())
		})

		// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
		AfterAll(func() {
			// collect SPI ResourceQuota metrics (temporary)
			err := fw.AsKubeAdmin.CommonController.GetSpiResourceQuotaInfo("token-upload-k8s", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())

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

		It("creates secret with access token and associate it to an existing SPIAccessToken", func() {
			tokenData := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
			Expect(tokenData).NotTo(BeEmpty())

			K8sSecret, err = fw.AsKubeDeveloper.SPIController.UploadWithK8sSecret(secretName, namespace, nonExistingAccessTokenName, RepoURL, "", tokenData)
			Expect(err).NotTo(HaveOccurred())
		})

		It("upload secret should be automatically be removed", func() {
			Eventually(func() bool {
				_, err := fw.AsKubeDeveloper.CommonController.GetSecret(namespace, K8sSecret.Name)

				if err == nil {
					return false
				}

				return k8sErrors.IsNotFound(err)
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "upload secret not removed")
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
