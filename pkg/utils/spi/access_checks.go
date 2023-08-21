package spi

import (
	"context"

	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateSPIAccessCheck creates a SPIAccessCheck object
func (s *SPIController) CreateSPIAccessCheck(name, namespace, repoURL string) (*spi.SPIAccessCheck, error) {
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
func (s *SPIController) GetSPIAccessCheck(name, namespace string) (*spi.SPIAccessCheck, error) {
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

// DeleteAllSPIAccessChecksInASpecificNamespace deletes all SPIAccessCheck from a given namespace
func (s *SPIController) DeleteAllAccessChecksInASpecificNamespace(namespace string) error {
	return s.KubeRest().DeleteAllOf(context.TODO(), &spi.SPIAccessCheck{}, client.InNamespace(namespace))
}
