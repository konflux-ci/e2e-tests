package spi

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	SPIAccessTokenBindingPrefixName = "e2e-access-token-binding"
)

// GetSPIAccessTokenBinding returns the requested SPIAccessTokenBinding object
func (s *SPIController) GetSPIAccessToken(name, namespace string) (*spi.SPIAccessToken, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	spiAccessToken := spi.SPIAccessToken{
		Spec: spi.SPIAccessTokenSpec{},
	}
	err := s.KubeRest().Get(context.TODO(), namespacedName, &spiAccessToken)
	if err != nil {
		return nil, err
	}
	return &spiAccessToken, nil
}

// Inject manually access tokens using spi API
func (s *SPIController) InjectManualSPIToken(namespace string, repoUrl string, oauthCredentials string, secretType v1.SecretType, secretName string) string {
	var spiAccessTokenBinding *spi.SPIAccessTokenBinding

	// Get the token for the current openshift user
	bearerToken, err := utils.GetOpenshiftToken()
	Expect(err).NotTo(HaveOccurred())

	// https://issues.redhat.com/browse/STONE-444. Is not possible to create more than 1 secret per user namespace
	secret, err := s.KubeInterface().CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		klog.Infof("secret %s already exists", secret.Name)

		return secret.Name
	}

	spiAccessTokenBinding, err = s.CreateSPIAccessTokenBinding(SPIAccessTokenBindingPrefixName, namespace, repoUrl, secretName, secretType)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() bool {
		// application info should be stored even after deleting the application in application variable
		spiAccessTokenBinding, err = s.GetSPIAccessTokenBinding(spiAccessTokenBinding.Name, namespace)

		if err != nil {
			return false
		}

		return (spiAccessTokenBinding.Status.Phase == spi.SPIAccessTokenBindingPhaseInjected || spiAccessTokenBinding.Status.Phase == spi.SPIAccessTokenBindingPhaseAwaitingTokenData)
	}, 2*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI controller didn't set SPIAccessTokenBinding to AwaitingTokenData/Injected")

	Eventually(func() bool {
		// application info should be stored even after deleting the application in application variable
		spiAccessTokenBinding, err = s.GetSPIAccessTokenBinding(spiAccessTokenBinding.Name, namespace)

		if err != nil {
			return false
		}

		return spiAccessTokenBinding.Status.UploadUrl != ""
	}, 5*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI oauth url not set. Please check if spi oauth-config configmap contain all necessary providers for tests.")

	if spiAccessTokenBinding.Status.Phase == spi.SPIAccessTokenBindingPhaseAwaitingTokenData {
		// If the phase is AwaitingTokenData then manually inject the git token
		// Get the oauth url and linkedAccessTokenName from the spiaccesstokenbinding resource
		Expect(err).NotTo(HaveOccurred())
		linkedAccessTokenName := spiAccessTokenBinding.Status.LinkedAccessTokenName

		// Before injecting the token, validate that the linkedaccesstoken resource exists, otherwise injecting will return a 404 error code
		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			_, err := s.GetSPIAccessToken(linkedAccessTokenName, namespace)
			return err == nil
		}, 1*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI controller didn't create the SPIAccessToken")

		// Format for quay.io token injection: `{"access_token":"tokenToInject","username":"redhat-appstudio-qe+redhat_appstudio_qe_bot"}`
		// Now that the spiaccesstokenbinding is in the AwaitingTokenData phase, inject the GitHub token
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		req, err := http.NewRequest("POST", spiAccessTokenBinding.Status.UploadUrl, bytes.NewBuffer([]byte(oauthCredentials)))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", string(bearerToken)))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).Should(Equal(204))
		defer resp.Body.Close()

		// Check to see if the token was successfully injected
		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			spiAccessTokenBinding, err = s.GetSPIAccessTokenBinding(spiAccessTokenBinding.Name, namespace)
			return err == nil && spiAccessTokenBinding.Status.Phase == spi.SPIAccessTokenBindingPhaseInjected
		}, 1*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI controller didn't set SPIAccessTokenBinding to Injected")
	}
	return secretName
}

// Remove all tokens from a given repository. Useful when creating a lot of resources and wanting to remove all of them
func (s *SPIController) DeleteAllAccessTokensInASpecificNamespace(namespace string) error {
	return s.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessToken{}, client.InNamespace(namespace))
}

// Remove all tokens from a given repository. Useful when creating a lot of resources and wanting to remove all of them
func (s *SPIController) DeleteAllBindingTokensInASpecificNamespace(namespace string) error {
	return s.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessTokenBinding{}, client.InNamespace(namespace))
}

// Remove all tokens from a given repository. Useful when creating a lot of resources and wanting to remove all of them
func (s *SPIController) DeleteAllAccessTokenDataInASpecificNamespace(namespace string) error {
	return s.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessTokenDataUpdate{}, client.InNamespace(namespace))
}
