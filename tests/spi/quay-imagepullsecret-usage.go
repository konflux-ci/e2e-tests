package spi

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	var QuayAuthToken string
	var QuayAuthUser string

	Describe("SVPI-407 - Check ImagePullsecret usage for the private Quay image", Ordered, func() {
		BeforeAll(func() {

			if os.Getenv("CI") != "true" {
				Skip(fmt.Sprintln("test skipped on local"))
			}
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())

			// Quay username and token are required by SPI to generate valid credentials
			QuayAuthToken = utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "")
			QuayAuthUser = utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "")
			Expect(QuayAuthToken).NotTo(BeEmpty())
			Expect(QuayAuthUser).NotTo(BeEmpty())
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

		var SPITokenBinding *v1beta1.SPIAccessTokenBinding
		var QuaySPITokenBindingName = "quay-spi-token-binding"

		It("creates SPITokenBinding", func() {
			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(QuaySPITokenBindingName, namespace, QuayPrivateRepoURL, "", corev1.SecretTypeDockerConfigJson)
			Expect(err).NotTo(HaveOccurred())
		})

		// start of upload token
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

		It("uploads username and token using rest endpoint", func() {
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
			oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "") + `", "username":"` + utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "") + `"}`
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

		// Create a pod using the generated ImagePullSecret to pull a private quay image
		It("Create a Pod from a Private Quay image", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "rtw"},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "quay-image",
							Image:           QuayPrivateRepoURL,
							ImagePullPolicy: corev1.PullAlways,
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: SPITokenBinding.Status.SyncedObjectRef.Name},
					},
				}}

			pod, err = fw.AsKubeAdmin.CommonController.KubeInterface().CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})

			Eventually(func() bool {
				pod, err := fw.AsKubeAdmin.CommonController.GetPod(namespace, pod.Name)
				if err != nil {
					return false
				}
				return pod.Status.Phase == corev1.PodRunning
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Pod not created successfully")
		})

	})
})
