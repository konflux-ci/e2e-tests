package spi

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
 * Component: spi
 * Description: SVPI-402 - Get file content from a private Github repository
 */

var _ = framework.SPISuiteDescribe(Label("spi-suite", "gh-oauth-flow"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var CYPRESS_GH_USER string
	var CYPRESS_GH_PASSWORD string
	var CYPRESS_GH_2FA_CODE string

	var OAUTH_REDIRECT_PROXY_URL string = "https://spi-oauth-redirect-proxy-pj7h-rhtap-qe-shared-tenant.apps.stone-prd-m01.84db.p1.openshiftapps.com/oauth/callback"

	Describe("SVPI-395 - Github OAuth flow to upload token", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos-oauth"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())

			CYPRESS_GH_USER = utils.GetEnv("CYPRESS_GH_USER", "")
			Expect(CYPRESS_GH_USER).NotTo(BeEmpty(), "Please provide CYPRESS_GH_USER")

			CYPRESS_GH_PASSWORD = utils.GetEnv("CYPRESS_GH_PASSWORD", "")
			Expect(CYPRESS_GH_PASSWORD).NotTo(BeEmpty(), "Please provide CYPRESS_GH_PASSWORD")

			CYPRESS_GH_2FA_CODE = utils.GetEnv("CYPRESS_GH_2FA_CODE", "")
			Expect(CYPRESS_GH_2FA_CODE).NotTo(BeEmpty(), "Please provide CYPRESS_GH_2FA_CODE env")

		})

		// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
		AfterAll(func() {

			if !CurrentSpecReport().Failed() {
				//Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
				//Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(namespace)).To(Succeed())
			}
		})

		var SPITokenBinding *v1beta1.SPIAccessTokenBinding
		var CYPRESS_SPI_OAUTH_URL string
		tokenBindingName := "spi-token-binding-oauth-"

		It("ensure OauthRedirectProxyUrl is set", func() {
			spiNamespace := "spi-system"
			config, err := fw.AsKubeAdmin.CommonController.GetConfigMap("spi-oauth-service-environment-config", spiNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, redirectUrlSet := config.Data["OAUTH_REDIRECT_PROXY_URL"]

			if redirectUrlSet {
				config.Data["OAUTH_REDIRECT_PROXY_URL"] = OAUTH_REDIRECT_PROXY_URL
				_, err = fw.AsKubeAdmin.CommonController.UpdateConfigMap(config, spiNamespace)
				Expect(err).NotTo(HaveOccurred())
				_, err := fw.AsKubeAdmin.CommonController.RolloutRestartDeployment("spi-oauth-service", spiNamespace)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					deployment, err := fw.AsKubeAdmin.CommonController.GetDeployment("spi-oauth-service", spiNamespace)
					Expect(err).NotTo(HaveOccurred())
					return deployment.Status.AvailableReplicas == 1
				}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "SPIAccessTokenBinding is not in Injected phase")

			}

			config, err = fw.AsKubeAdmin.CommonController.GetConfigMap("spi-oauth-service-environment-config", spiNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Data["OAUTH_REDIRECT_PROXY_URL"]).NotTo(BeEmpty())

			// TBD -> check/ensure shared-config is provided for oauth providers

		})

		It("creates SPITokenBinding", func() {
			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(tokenBindingName, namespace, RepoURL, "", "kubernetes.io/basic-auth")
			Expect(err).NotTo(HaveOccurred())

			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
			Expect(err).NotTo(HaveOccurred())

			CYPRESS_SPI_OAUTH_URL = SPITokenBinding.Status.OAuthUrl
			Expect(CYPRESS_SPI_OAUTH_URL).NotTo(BeEmpty())

			k8s_token, err := utils.GetOpenshiftToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(k8s_token).NotTo(BeEmpty())

			// TBD -> get normal user token??
			CYPRESS_SPI_OAUTH_URL = CYPRESS_SPI_OAUTH_URL + "&k8s_token=" + k8s_token
		})

		It("run browser oauth login flow in cypress pod", func() {

			// Now we create a short-living pod that will use cypress to perform the browser login flow
			cypressPod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cypress-script",
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "cypress-test",
							Image:   "cypress/included",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"git clone https://github.com/albarbaro/proxy-sample.git; cd proxy-sample/cypress; npm install; cypress run --spec cypress/e2e/spec.cy.js;"},
							Env: []corev1.EnvVar{
								{
									Name:  "CYPRESS_GH_USER",
									Value: CYPRESS_GH_USER,
								},
								{
									Name:  "CYPRESS_GH_PASSWORD",
									Value: CYPRESS_GH_PASSWORD,
								},
								{
									Name:  "CYPRESS_GH_2FA_CODE",
									Value: CYPRESS_GH_2FA_CODE,
								},
								{
									Name:  "CYPRESS_SPI_OAUTH_URL",
									Value: CYPRESS_SPI_OAUTH_URL,
								},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			}

			_, err := fw.AsKubeAdmin.CommonController.KubeInterface().CoreV1().Pods(namespace).Create(context.TODO(), cypressPod, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			// check pod is completed without error

			Eventually(func() bool {
				pod, err := fw.AsKubeAdmin.CommonController.GetPod(namespace, cypressPod.Name)
				Expect(err).NotTo(HaveOccurred())

				return (pod.Status.Phase == corev1.PodSucceeded)
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), "Cypress pod did not completed oauth flow")

			// check tokenbinding is injected

		})

	})
})
