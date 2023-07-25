package spi

import (
	"context"

	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CreateSPIFileContentRequest creates an SPIFileContentRequest Object
func (s *SPIController) CreateSPIFileContentRequest(name, namespace, repoURL, filePath string) (*spi.SPIFileContentRequest, error) {
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
func (s *SPIController) GetSPIFileContentRequest(name, namespace string) (*spi.SPIFileContentRequest, error) {
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
