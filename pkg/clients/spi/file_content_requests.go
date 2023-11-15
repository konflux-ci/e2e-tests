package spi

import (
	"context"
	"time"

	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
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
	err := s.KubeRest().Create(context.Background(), &spiFcr)
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
	err := s.KubeRest().Get(context.Background(), namespacedName, &spiFcr)
	if err != nil {
		return nil, err
	}
	return &spiFcr, nil
}

func (s *SPIController) IsSPIFileContentRequestInDeliveredPhase(SPIFcr *spi.SPIFileContentRequest) {
	Eventually(func() spi.SPIFileContentRequestStatus {
		SPIFcr, err := s.GetSPIFileContentRequest(SPIFcr.Name, SPIFcr.Namespace)
		Expect(err).NotTo(HaveOccurred())

		return SPIFcr.Status
	}, 2*time.Minute, 10*time.Second).Should(MatchFields(IgnoreExtras, Fields{
		"Phase":   Equal(spi.SPIFileContentRequestPhaseDelivered),
		"Content": Not(BeEmpty()),
	}), "SPIFileContentRequest %s/%s '.Status' does not contain expected field values", SPIFcr.GetNamespace(), SPIFcr.GetName())
}
