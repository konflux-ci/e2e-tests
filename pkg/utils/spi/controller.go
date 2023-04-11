package spi

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
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

type SuiteController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*SuiteController, error) {
	// Initialize a new SPI controller with just the kube client
	return &SuiteController{
		kube,
	}, nil
}

// GetSPIAccessTokenBinding returns the requested SPIAccessTokenBinding object
func (s *SuiteController) GetSPIAccessTokenBinding(name, namespace string) (*spi.SPIAccessTokenBinding, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	spiAccessTokenBinding := spi.SPIAccessTokenBinding{
		Spec: spi.SPIAccessTokenBindingSpec{},
	}
	err := s.KubeRest().Get(context.TODO(), namespacedName, &spiAccessTokenBinding)
	if err != nil {
		return nil, err
	}
	return &spiAccessTokenBinding, nil
}

// CreateSPIAccessTokenBinding creates an SPIAccessTokenBinding object
func (s *SuiteController) CreateSPIAccessTokenBinding(name, namespace, repoURL, secretName string, secretType v1.SecretType) (*spi.SPIAccessTokenBinding, error) {
	spiAccessTokenBinding := spi.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    namespace,
		},
		Spec: spi.SPIAccessTokenBindingSpec{
			Permissions: spi.Permissions{
				Required: []spi.Permission{
					{
						Type: spi.PermissionTypeReadWrite,
						Area: spi.PermissionAreaRepository,
					},
				},
			},
			RepoUrl: repoURL,
			Secret: spi.SecretSpec{
				Name: secretName,
				Type: secretType,
			},
		},
	}
	err := s.KubeRest().Create(context.TODO(), &spiAccessTokenBinding)
	if err != nil {
		return nil, err
	}
	return &spiAccessTokenBinding, nil
}

// DeleteSPIAccessTokenBinding deletes an SPIAccessTokenBinding from a given name and namespace
func (h *SuiteController) DeleteSPIAccessTokenBinding(name, namespace string) error {
	application := spi.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), &application)
}

// GetSPIAccessTokenBinding returns the requested SPIAccessTokenBinding object
func (s *SuiteController) GetSPIAccessToken(name, namespace string) (*spi.SPIAccessToken, error) {
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
func (s *SuiteController) InjectManualSPIToken(namespace string, repoUrl string, oauthCredentials string, secretType v1.SecretType, secretName string) string {
	var spiAccessTokenBinding *v1beta1.SPIAccessTokenBinding

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

		return (spiAccessTokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseInjected || spiAccessTokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData)
	}, 2*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI controller didn't set SPIAccessTokenBinding to AwaitingTokenData/Injected")

	Eventually(func() bool {
		// application info should be stored even after deleting the application in application variable
		spiAccessTokenBinding, err = s.GetSPIAccessTokenBinding(spiAccessTokenBinding.Name, namespace)

		if err != nil {
			return false
		}

		return spiAccessTokenBinding.Status.UploadUrl != ""
	}, 5*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI oauth url not set. Please check if spi oauth-config configmap contain all necessary providers for tests.")

	if spiAccessTokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData {
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
			return err == nil && spiAccessTokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseInjected
		}, 1*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI controller didn't set SPIAccessTokenBinding to Injected")
	}
	return secretName
}

// Remove all tokens from a given repository. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllBindingTokensInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &v1beta1.SPIAccessTokenBinding{}, client.InNamespace(namespace))
}

// Remove all tokens from a given repository. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllAccessTokenDataInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &v1beta1.SPIAccessTokenDataUpdate{}, client.InNamespace(namespace))
}

// Remove all tokens from a given repository. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllAccessTokensInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &v1beta1.SPIAccessToken{}, client.InNamespace(namespace))
}

// Perform http POST call to upload a token at the given upload URL
func (h *SuiteController) UploadWithRestEndpoint(uploadURL string, oauthCredentials string, bearerToken string) (int, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, err := http.NewRequest("POST", uploadURL, bytes.NewBuffer([]byte(oauthCredentials)))
	if err != nil {
		return 0, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", string(bearerToken)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp.StatusCode, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// GetSPIAccessTokenBinding returns the requested SPIAccessTokenBinding object
func (s *SuiteController) UploadWithK8sSecret(secretName, namespace, spiTokenName, providerURL, username, tokenData string) (*v1.Secret, error) {
	k8sSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      secretName,
			Labels: map[string]string{
				"spi.appstudio.redhat.com/upload-secret": "token",
			},
		},
		Type: "Opaque",
		StringData: map[string]string{
			"tokenData": tokenData,
		},
	}
	if spiTokenName != "" {
		k8sSecret.StringData["spiTokenName"] = spiTokenName
	}
	if providerURL != "" {
		k8sSecret.StringData["providerUrl"] = providerURL
	}
	if username != "" {
		k8sSecret.StringData["userName"] = username
	}

	err := s.KubeRest().Create(context.TODO(), k8sSecret)
	if err != nil {
		return nil, err
	}
	return k8sSecret, nil
}
