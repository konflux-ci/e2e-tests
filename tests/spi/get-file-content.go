package spi

import (
	"fmt"
	"os"
	"time"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

/*
 * Component: spi
 * Description: SVPI-402 - Get file content from a private Github repository
 * Use case: SPIAccessToken Usage
 */

var _ = framework.SPISuiteDescribe(Label("spi-suite", "get-file-content"), Pending, func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var SPIFcr *v1beta1.SPIFileContentRequest
	var SPITokenBinding *v1beta1.SPIAccessTokenBinding
	AfterEach(framework.ReportFailure(&fw))

	Describe("SVPI-402 - Get file content from a private Github repository with SPIAccessToken", Ordered, func() {
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
			err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("get-file-content", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			// collect SPI ResourceQuota metrics (temporary)
			err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("get-file-content", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())

			if !CurrentSpecReport().Failed() {
				Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
			}
		})

		It("creates SPITokenBinding", func() {
			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(SPITokenBindingName, namespace, GithubPrivateRepoURL, "", "kubernetes.io/basic-auth")
			Expect(err).NotTo(HaveOccurred())

			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("uploads token", func() {
			// SPITokenBinding to be in AwaitingTokenData phase
			Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, SPITokenBinding.Namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPITokenBinding.Status.Phase
			}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData))

			// start upload username and token using rest endpoint
			// the UploadUrl in SPITokenBinding should be available before uploading the token
			Eventually(func() string {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, SPITokenBinding.Namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPITokenBinding.Status.UploadUrl
			}, 1*time.Minute, 10*time.Second).ShouldNot(BeEmpty(), fmt.Sprintf(".Status.UploadUrl for SPIAccessTokenBinding %s/%s is not set", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName()))

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
			// end upload username and token using rest endpoint

			// SPITokenBinding to be in Injected phase
			Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
				binding, err := fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, SPITokenBinding.Namespace)
				Expect(err).NotTo(HaveOccurred())
				return binding.Status.Phase
			}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseInjected), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenBindingPhaseInjected))
		})

		It("creates SPIFileContentRequest", func() {
			SPIFcr, err = fw.AsKubeDeveloper.SPIController.CreateSPIFileContentRequest("gh-spi-filecontent-request", namespace, GithubPrivateRepoURL, GithubPrivateRepoFilePath)
			Expect(err).NotTo(HaveOccurred())

			SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("SPIFileContentRequest should be in Delivered phase and content should be provided", func() {
			fw.AsKubeDeveloper.SPIController.IsSPIFileContentRequestInDeliveredPhase(SPIFcr)
		})
	})
})
