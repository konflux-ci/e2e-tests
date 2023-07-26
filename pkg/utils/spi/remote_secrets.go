package spi

import (
	"context"

	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateRemoteSecret creates a RemoteSecret object
func (s *SPIController) CreateRemoteSecret(name, namespace string, targetNamespaces []string) (*rs.RemoteSecret, error) {
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
func (s *SPIController) GetRemoteSecret(name, namespace string) (*rs.RemoteSecret, error) {
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

// GetTargetSecretName gets the target secret name from a given namespace
func (s *SPIController) GetTargetSecretName(targets []rs.TargetStatus, targetNamespace string) string {
	targetSecretName := ""

	for _, t := range targets {
		if t.Namespace == targetNamespace {
			return t.SecretName
		}
	}

	return targetSecretName
}

// Remove all RemoteSecret from a given namespace. Useful when creating a lot of resources and wanting to remove all of them
func (h *SPIController) DeleteAllRemoteSecretsInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &rs.RemoteSecret{}, client.InNamespace(namespace))
}
