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
	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
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
				LinkableSecretSpec: rs.LinkableSecretSpec{
					Name: secretName,
					Type: secretType,
				},
			},
		},
	}
	err := s.KubeRest().Create(context.TODO(), &spiAccessTokenBinding)
	if err != nil {
		return nil, err
	}
	return &spiAccessTokenBinding, nil
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

// Remove all SPIAccessTokenBinding from a given namespace. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllBindingTokensInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessTokenBinding{}, client.InNamespace(namespace))
}

// Remove all SPIAccessTokenDataUpdate from a given namespace. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllAccessTokenDataInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessTokenDataUpdate{}, client.InNamespace(namespace))
}

// Remove all SPIAccessToken from a given namespace. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllAccessTokensInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessToken{}, client.InNamespace(namespace))
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

// UploadWithK8sSecret returns the requested Secret object
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

// CreateSPIAccessCheck creates a SPIAccessCheck object
func (s *SuiteController) CreateSPIAccessCheck(name, namespace, repoURL string) (*spi.SPIAccessCheck, error) {
	spiAccessCheck := spi.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    namespace,
		},
		Spec: spi.SPIAccessCheckSpec{RepoUrl: repoURL},
	}
	err := s.KubeRest().Create(context.TODO(), &spiAccessCheck)
	if err != nil {
		return nil, err
	}
	return &spiAccessCheck, nil
}

// GetSPIAccessCheck returns the requested SPIAccessCheck object
func (s *SuiteController) GetSPIAccessCheck(name, namespace string) (*spi.SPIAccessCheck, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	spiAccessCheck := spi.SPIAccessCheck{
		Spec: spi.SPIAccessCheckSpec{},
	}
	err := s.KubeRest().Get(context.TODO(), namespacedName, &spiAccessCheck)
	if err != nil {
		return nil, err
	}
	return &spiAccessCheck, nil
}

// CreateSPIAccessTokenBindingWithSA creates SPIAccessTokenBinding with secret linked to a service account
// There are three ways of linking a secret to a service account:
// - Linking a secret to an existing service account
// - Linking a secret to an existing service account as image pull secret
// - Using a managed service account
func (s *SuiteController) CreateSPIAccessTokenBindingWithSA(name, namespace, serviceAccountName, repoURL, secretName string, isImagePullSecret, isManagedServiceAccount bool) (*spi.SPIAccessTokenBinding, error) {
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
				LinkableSecretSpec: rs.LinkableSecretSpec{
					Name: secretName,
					Type: "kubernetes.io/dockerconfigjson",
					LinkedTo: []rs.SecretLink{
						{
							ServiceAccount: rs.ServiceAccountLink{
								Reference: v1.LocalObjectReference{
									Name: serviceAccountName,
								},
							},
						},
					},
				},
			},
		},
	}

	if isImagePullSecret {
		spiAccessTokenBinding.Spec.Secret.LinkedTo[0].ServiceAccount.As = rs.ServiceAccountLinkTypeImagePullSecret
	}

	if isManagedServiceAccount {
		spiAccessTokenBinding.Spec.Secret.Type = "kubernetes.io/basic-auth"
		spiAccessTokenBinding.Spec.Secret.LinkedTo = []rs.SecretLink{
			{
				ServiceAccount: rs.ServiceAccountLink{
					Managed: rs.ManagedServiceAccountSpec{
						GenerateName: serviceAccountName,
					},
				},
			},
		}
	}

	err := s.KubeRest().Create(context.TODO(), &spiAccessTokenBinding)
	if err != nil {
		return nil, err
	}
	return &spiAccessTokenBinding, nil
}

// DeleteAllSPIAccessChecksInASpecificNamespace deletes all SPIAccessCheck from a given namespace
func (h *SuiteController) DeleteAllAccessChecksInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessCheck{}, client.InNamespace(namespace))
}

func (s *SuiteController) CreateSPIFileContentRequest(name, namespace, repoURL, filePath string) (*spi.SPIFileContentRequest, error) {
	spiFcr := spi.SPIFileContentRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    namespace,
		},
		Spec: spi.SPIFileContentRequestSpec{RepoUrl: repoURL, FilePath: filePath},
	}
	err := s.KubeRest().Create(context.TODO(), &spiFcr)
	if err != nil {
		return nil, err
	}
	return &spiFcr, nil
}

// GetSPIAccessCheck returns the requested SPIAccessCheck object
func (s *SuiteController) GetSPIFileContentRequest(name, namespace string) (*spi.SPIFileContentRequest, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	spiFcr := spi.SPIFileContentRequest{
		Spec: spi.SPIFileContentRequestSpec{},
	}
	err := s.KubeRest().Get(context.TODO(), namespacedName, &spiFcr)
	if err != nil {
		return nil, err
	}
	return &spiFcr, nil
}

// CreateRemoteSecret creates a RemoteSecret object
func (s *SuiteController) CreateRemoteSecret(name, namespace string, targetNamespaces []string) (*rs.RemoteSecret, error) {
	remoteSecret := rs.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: rs.RemoteSecretSpec{
			Secret: rs.LinkableSecretSpec{
				GenerateName: "some-secret-",
			},
		},
	}

	targets := make([]rs.RemoteSecretTarget, 0)
	for _, target := range targetNamespaces {
		targets = append(targets, rs.RemoteSecretTarget{
			Namespace: target,
		})
	}
	remoteSecret.Spec.Targets = targets

	err := s.KubeRest().Create(context.TODO(), &remoteSecret)
	if err != nil {
		return nil, err
	}
	return &remoteSecret, nil
}

// GetRemoteSecret returns the requested RemoteSecret object
func (s *SuiteController) GetRemoteSecret(name, namespace string) (*rs.RemoteSecret, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	remoteSecret := rs.RemoteSecret{}

	err := s.KubeRest().Get(context.TODO(), namespacedName, &remoteSecret)
	if err != nil {
		return nil, err
	}
	return &remoteSecret, nil
}

// Remove all RemoteSecret from a given namespace. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllRemoteSecretsInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &rs.RemoteSecret{}, client.InNamespace(namespace))
}

// GetTargetSecretName gets the target secret name from a given namespace
func (s *SuiteController) GetTargetSecretName(targets []rs.TargetStatus, targetNamespace string) string {
	targetSecretName := ""

	for _, t := range targets {
		if t.Namespace == targetNamespace {
			return t.SecretName
		}
	}

	return targetSecretName
}
