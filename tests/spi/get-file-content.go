package spi

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

/*
 * Component: spi
 * Description: SVPI-402 - Get file content from a private Github repository
 */

// pending because https://github.com/redhat-appstudio/service-provider-integration-operator/pull/706 will break the tests
// we will need to update the current test after merging the PR
var _ = framework.SPISuiteDescribe(Label("spi-suite", "get-file-content"), Pending, func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	AfterEach(framework.ReportFailure(&fw))

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

			// collect SPI ResourceQuota metrics (temporary)
			err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("token-upload-rest-endpoint", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())
		})

		// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
		AfterAll(func() {
			// collect SPI ResourceQuota metrics (temporary)
			err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("get-file-content", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())

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

			SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("SPIFileContentRequest should be in AwaitingTokenData phase", func() {
			Eventually(func() v1beta1.SPIFileContentRequestPhase {
				SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPIFcr.Status.Phase
			}, 2*time.Minute, 10*time.Second).Should(Equal(v1beta1.SPIFileContentRequestPhaseAwaitingTokenData), fmt.Sprintf("SPIFileContentRequest %s/%s '.Status.Phase' field didn't have the expected value", SPIFcr.GetNamespace(), SPIFcr.GetName()))

		})

		It("uploads username and token using rest endpoint", func() {
			// the UploadUrl in SPITokenBinding should be available before uploading the token
			Eventually(func() string {
				SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPIFcr.Status.TokenUploadUrl
			}, 1*time.Minute, 10*time.Second).ShouldNot(BeEmpty(), fmt.Sprintf(".Status.TokenUploadUrl field in SPIFileContentRequest %s/%s is empty", SPIFcr.GetNamespace(), SPIFcr.GetName()))

			// get the url to manually upload the token
			uploadURL := SPIFcr.Status.TokenUploadUrl

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
			Eventually(func() v1beta1.SPIFileContentRequestStatus {
				SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPIFcr.Status
			}, 2*time.Minute, 10*time.Second).Should(MatchFields(IgnoreExtras, Fields{
				"Phase":   Equal(v1beta1.SPIFileContentRequestPhaseDelivered),
				"Content": Not(BeEmpty()),
			}), "SPIFileContentRequest %s/%s '.Status' does not contain expected field values", SPIFcr.GetNamespace(), SPIFcr.GetName())

		})

	})
})
