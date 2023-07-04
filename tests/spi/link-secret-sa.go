package spi

import (
	"fmt"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

/*
 * Component: spi
 * Description: SVPI-406 Check SA creation and linking to the secret requested by SPIAccessTokenBinding

 * Test Scenario 1: link a secret to an existing service account
 * Test Scenario 2: link a secret to an existing service account as image pull secret
 * Test Scenario 3: link a secret to a managed service account
 * For more details, check ServiceAccountTests in var.go

 * Flow of each test:
	* 1º - creates SPITokenBinding with SA associated
	* 2º - uploads token
	* 3º - checks if SA was linked to the secret
*/

var _ = framework.SPISuiteDescribe(Label("spi-suite", "link-secret-sa"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string

	for _, test := range ServiceAccountTests {
		test := test

		Describe("SVPI-406 - "+test.TestName, Ordered, func() {
			BeforeAll(func() {
				// Initialize the tests controllers
				fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
				Expect(err).NotTo(HaveOccurred())
				namespace = fw.UserNamespace
				Expect(namespace).NotTo(BeEmpty())

				// collect SPI ResourceQuota metrics (temporary)
				err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("token-upload-rest-endpoint", namespace, "appstudio-crds-spi")
				Expect(err).NotTo(HaveOccurred())
			})

			// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
			AfterAll(func() {
				// collect SPI ResourceQuota metrics (temporary)
				err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("link-secret-sa", namespace, "appstudio-crds-spi")
				Expect(err).NotTo(HaveOccurred())

				if !CurrentSpecReport().Failed() {
					Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(namespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokenDataInASpecificNamespace(namespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.CommonController.DeleteAllServiceAccountsInASpecificNamespace(namespace)).To(Succeed())
				}
			})

			var binding *v1beta1.SPIAccessTokenBinding
			secretName := utils.GetGeneratedNamespace("new-secret")
			nonExistingServiceAccountName := utils.GetGeneratedNamespace("new-service-account")
			serviceAccountName := nonExistingServiceAccountName

			It("creates service account", func() {
				if !test.IsManagedServiceAccount { // Test Scenario 1 and Test Scenario 2 (the service account should exist before the binding)
					existingServiceAccountName := utils.GetGeneratedNamespace("service-account")
					_, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(existingServiceAccountName, namespace, nil)
					Expect(err).NotTo(HaveOccurred())
					serviceAccountName = existingServiceAccountName
				}
			})

			It("creates SPIAccessTokenBinding with secret linked to a service account", func() {
				binding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBindingWithSA(
					SPIAccessTokenBindingPrefixName,
					namespace,
					serviceAccountName,
					RepoURL,
					secretName,
					test.IsImagePullSecret,
					test.IsManagedServiceAccount)
				Expect(err).NotTo(HaveOccurred())

				binding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(binding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())
			})

			// start of upload token
			It("SPITokenBinding to be in AwaitingTokenData phase", func() {
				Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
					binding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(binding.Name, namespace)
					Expect(err).NotTo(HaveOccurred())

					return binding.Status.Phase
				}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", binding.GetNamespace(), binding.GetName(), v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData))
			})

			It("uploads username and token using rest endpoint", func() {
				// the UploadUrl in SPITokenBinding should be available before uploading the token
				Eventually(func() string {
					binding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(binding.Name, namespace)
					Expect(err).NotTo(HaveOccurred())

					return binding.Status.UploadUrl
				}, 1*time.Minute, 10*time.Second).ShouldNot(BeEmpty(), fmt.Sprintf(".Status.UploadUrl for SPIAccessTokenBinding %s/%s is not set", binding.GetNamespace(), binding.GetName()))
				Expect(err).NotTo(HaveOccurred())

				// linked accessToken token should exist
				linkedAccessTokenName := binding.Status.LinkedAccessTokenName
				Expect(linkedAccessTokenName).NotTo(BeEmpty())

				// get the url to manually upload the token
				uploadURL := binding.Status.UploadUrl

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
				Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
					binding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(binding.Name, namespace)
					Expect(err).NotTo(HaveOccurred())
					return binding.Status.Phase
				}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseInjected), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", binding.GetNamespace(), binding.GetName(), v1beta1.SPIAccessTokenBindingPhaseInjected))
			})
			// end of upload token

			It("checks if service account was linked to the secret", func() {
				// get the service account name associated with the binding
				// this is a workaround to get the managed service account name that is generated
				serviceAccountNames := binding.Status.ServiceAccountNames
				Expect(serviceAccountNames).NotTo(BeEmpty())
				saName := serviceAccountNames[0]

				if !test.IsImagePullSecret {
					// Test Scenario 1 and 3
					Eventually(func() bool {
						sa, err := fw.AsKubeDeveloper.CommonController.GetServiceAccount(saName, namespace)
						Expect(err).NotTo(HaveOccurred())
						for _, secret := range sa.Secrets {
							if secret.Name == secretName {
								return true
							}
						}
						return false
					}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("The secret %s is not linked to the service account %s", secretName, saName))
				} else {
					// Test Scenario 2
					Eventually(func() bool {
						sa, err := fw.AsKubeDeveloper.CommonController.GetServiceAccount(saName, namespace)
						Expect(err).NotTo(HaveOccurred())
						for _, secret := range sa.ImagePullSecrets {
							if secret.Name == secretName {
								return true
							}
						}
						return false
					}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("The secret %s is not linked to the service account %s", secretName, saName))
				}
			})
		})
	}
})
