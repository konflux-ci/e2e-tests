package spi

import (
	"context"

	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateSPIAccessTokenBinding creates an SPIAccessTokenBinding object
func (s *SPIController) CreateSPIAccessTokenBinding(name, namespace, repoURL, secretName string, secretType v1.SecretType) (*spi.SPIAccessTokenBinding, error) {
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

// CreateSPIAccessTokenBindingWithSA creates SPIAccessTokenBinding with secret linked to a service account
// There are three ways of linking a secret to a service account:
// - Linking a secret to an existing service account
// - Linking a secret to an existing service account as image pull secret
// - Using a managed service account
func (s *SPIController) CreateSPIAccessTokenBindingWithSA(name, namespace, serviceAccountName, repoURL, secretName string, isImagePullSecret, isManagedServiceAccount bool) (*spi.SPIAccessTokenBinding, error) {
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

// GetSPIAccessTokenBinding returns the requested SPIAccessTokenBinding object
func (s *SPIController) GetSPIAccessTokenBinding(name, namespace string) (*spi.SPIAccessTokenBinding, error) {
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

// Remove all SPIAccessTokenBinding from a given namespace. Useful when creating a lot of resources and wanting to remove all of them
func (s *SPIController) DeleteAllBindingTokensInASpecificNamespace(namespace string) error {
	return s.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessTokenBinding{}, client.InNamespace(namespace))
}
