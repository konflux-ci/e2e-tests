package spi

import (
	"context"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {
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
func (s *SuiteController) CreateSPIAccessTokenBinding(name, namespace, repoURL, secretName string) (*spi.SPIAccessTokenBinding, error) {
	spiAccessTokenBinding := spi.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
				Type: v1.SecretTypeBasicAuth,
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
