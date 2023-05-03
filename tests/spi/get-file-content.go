package spi

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

/*
 * Component: spi
 * Description: SVPI-402 - Get file content from a private Github repository
 */

var _ = framework.SPISuiteDescribe(Label("spi-suite", "get-file-content"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string

	Describe("SVPI-402 - Get file content from a private Github repository", Ordered, func() {
		BeforeAll(func() {
			if os.Getenv("CI") != "true" {
				Skip(fmt.Sprintln("test skipped on local execution"))
			}
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
				Expect(fw.AsKubeAdmin.CommonController.DeleteAllServiceAccountsInASpecificNamespace(namespace)).To(Succeed())
			}
		})

		var SPIFcr *v1beta1.SPIFileContentRequest

		It("creates SPIFileContentRequest", func() {
			SPIFcr, err = fw.AsKubeDeveloper.SPIController.CreateSPIFileContentRequest("gh-spi-filecontent-request", namespace, GithubPrivateRepoURL, GithubPrivateRepoFilePath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("SPIFileContentRequest should be in AwaitingTokenData phase", func() {
			Eventually(func() bool {
				SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)

				if err != nil {
					return false
				}

				return SPIFcr.Status.Phase == v1beta1.SPIFileContentRequestPhaseAwaitingTokenData
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "")

		})

		It("uploads username and token using rest endpoint", func() {
			// the UploadUrl in SPITokenBinding should be available before uploading the token
			Eventually(func() bool {
				SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)

				if err != nil {
					return false
				}

				return SPIFcr.Status.TokenUploadUrl != ""
			}, 1*time.Minute, 10*time.Second).Should(BeTrue(), "uploadUrl not set")
			Expect(err).NotTo(HaveOccurred())

			// get the url to manually upload the token
			uploadURL := SPIFcr.Status.TokenUploadUrl
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

		It("SPIFileContentRequest should be in Delivered phase and content should be provided", func() {
			Eventually(func() bool {
				SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)

				if err != nil {
					return false
				}

				return SPIFcr.Status.Phase == v1beta1.SPIFileContentRequestPhaseDelivered && SPIFcr.Status.Content != ""
			}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "content not provided by SPIFileContentRequest")

		})

	})
})
