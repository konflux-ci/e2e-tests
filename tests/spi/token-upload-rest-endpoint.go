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
 * Description: SVPI-398 - Token upload rest endpoint and SVPI-404 - Check access to GitHub repository
 * Note: To avoid code repetition, SVPI-404 was integrated with SVPI-398

 * Test Scenario 1: Token upload rest endpoint [public repository]
 * Test Scenario 2: Token upload rest endpoint [private repository]
 * For more details, check AccessCheckTests in var.go

 * Flow of each test:
	* 1º - creates SPITokenBinding
	* 2º - checks access to GitHub repository before token upload
	* 3º - uploads token
	* 4º - checks access to GitHub repository after token upload
*/

var _ = framework.SPISuiteDescribe(Label("spi-suite", "token-upload-rest-endpoint"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string

	for _, test := range AccessCheckTests {
		test := test

		Describe("SVPI-398 - Token upload rest endpoint: "+test.TestName, Ordered, func() {
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
				err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("token-upload-rest-endpoint", namespace, "appstudio-crds-spi")
				Expect(err).NotTo(HaveOccurred())

				if !CurrentSpecReport().Failed() {
					Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(namespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokenDataInASpecificNamespace(namespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessChecksInASpecificNamespace(namespace)).To(Succeed())
				}
			})

			var SPITokenBinding *v1beta1.SPIAccessTokenBinding
			It("creates SPITokenBinding", func() {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(SPITokenBindingName, namespace, test.RepoURL, "", "kubernetes.io/basic-auth")
				Expect(err).NotTo(HaveOccurred())

				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())
			})

			var SPIAccessCheck *v1beta1.SPIAccessCheck
			var SPIAccessToken *v1beta1.SPIAccessToken

			Describe("SVPI-404 - Check access to GitHub repository before token upload", func() {
				It("creates SPIAccessCheck", func() {
					SPIAccessCheck, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessCheck(SPIAccessCheckPrefixName, namespace, test.RepoURL)
					Expect(err).NotTo(HaveOccurred())

					SPIAccessCheck, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessCheck(SPIAccessCheck.Name, namespace)
					Expect(err).NotTo(HaveOccurred())
				})

				It("checks if repository is accessible", func() {
					Eventually(func() v1beta1.SPIAccessCheckAccessibility {
						SPIAccessCheck, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessCheck(SPIAccessCheck.Name, namespace)
						Expect(err).NotTo(HaveOccurred())
						// at this stage, before token upload, accessibility should be unknown (in case of private repo) or public (in case of public repo)
						return SPIAccessCheck.Status.Accessibility
					}, 1*time.Minute, 5*time.Second).Should(Or(Equal(v1beta1.SPIAccessCheckAccessibilityUnknown), Equal(v1beta1.SPIAccessCheckAccessibilityPublic)),
						fmt.Sprintf("SPIAccessCheck %s/%s has wrong info in '.Status.Accessibility' field", SPIAccessCheck.GetNamespace(), SPIAccessCheck.GetName()))

					if test.Accessibility == v1beta1.SPIAccessCheckAccessibilityPublic {
						//  if public, the repository should be accessible
						Expect(SPIAccessCheck.Status.Accessible).To(Equal(true))
						Expect(SPIAccessCheck.Status.Accessibility).To(Equal(test.Accessibility))
					} else {
						//  if private, the repository should not be accessible since the token was not upload yet
						Expect(SPIAccessCheck.Status.Accessible).To(Equal(false))
						Expect(SPIAccessCheck.Status.Accessibility).To(Equal(v1beta1.SPIAccessCheckAccessibilityUnknown))
					}

					Expect(SPIAccessCheck.Status.Type).To(Equal(test.RepoType))
					Expect(SPIAccessCheck.Status.ServiceProvider).To(Equal(test.ServiceProvider))
				})
			})

			// start of upload token
			It("SPITokenBinding to be in AwaitingTokenData phase", func() {
				// wait SPITokenBinding to be in AwaitingTokenData phase before trying to upload a token
				Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
					SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
					Expect(err).NotTo(HaveOccurred())

					return SPITokenBinding.Status.Phase
				}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData))
			})

			It("uploads username and token using rest endpoint", func() {
				// the UploadUrl in SPITokenBinding should be available before uploading the token
				Eventually(func() string {
					SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
					Expect(err).NotTo(HaveOccurred())

					return SPITokenBinding.Status.UploadUrl
				}, 1*time.Minute, 10*time.Second).ShouldNot(BeEmpty(), fmt.Sprintf(".Status.TokenUploadUrl field in SPIFileContentRequest %s/%s is empty", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName()))
				Expect(err).NotTo(HaveOccurred())

				// linked accessToken token should exist
				linkedAccessTokenName := SPITokenBinding.Status.LinkedAccessTokenName
				Expect(linkedAccessTokenName).NotTo(BeEmpty())

				// get the url to manually upload the token
				uploadURL := SPITokenBinding.Status.UploadUrl

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
					SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
					Expect(err).NotTo(HaveOccurred())
					return SPITokenBinding.Status.Phase
				}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseInjected), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenBindingPhaseInjected))
			})

			It("SPIAccessToken exists and is in Ready phase", func() {
				Eventually(func() (v1beta1.SPIAccessTokenPhase, error) {
					SPIAccessToken, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessToken(SPITokenBinding.Status.LinkedAccessTokenName, namespace)
					if err != nil {
						return "", err
					}
					return SPIAccessToken.Status.Phase, nil
				}, 2*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenPhaseReady), fmt.Sprintf("SPIAccessToken for SPITokenBinding %s/%s should be in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenPhaseReady))
			})
			// end of upload token

			Describe("SVPI-404 - Check access to GitHub repository after token upload", func() {
				It("creates SPIAccessCheck", func() {
					SPIAccessCheck, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessCheck(SPIAccessCheckPrefixName, namespace, test.RepoURL)
					Expect(err).NotTo(HaveOccurred())

					SPIAccessCheck, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessCheck(SPIAccessCheck.Name, namespace)
					Expect(err).NotTo(HaveOccurred())
				})

				It("checks if repository is accessible", func() {
					Eventually(func() bool {
						SPIAccessCheck, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessCheck(SPIAccessCheck.Name, namespace)
						Expect(err).NotTo(HaveOccurred())

						// both public and private repositories should be accessible, since the token was already uploaded
						return SPIAccessCheck.Status.Accessible
					}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("repository '%s' is not accessible", test.RepoURL))

					Expect(SPIAccessCheck.Status.Accessibility).To(Equal(test.Accessibility))
					Expect(SPIAccessCheck.Status.Type).To(Equal(test.RepoType))
					Expect(SPIAccessCheck.Status.ServiceProvider).To(Equal(test.ServiceProvider))
				})
			})
		})
	}
})
