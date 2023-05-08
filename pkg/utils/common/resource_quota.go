package common

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetResourceQuota returns the ResourceQuota object from a given namespace and ResourceQuota name
func (s *SuiteController) GetResourceQuota(namespace, ResourceQuotaName string) (*corev1.ResourceQuota, error) {
	return s.KubeInterface().CoreV1().ResourceQuotas(namespace).Get(context.TODO(), ResourceQuotaName, metav1.GetOptions{})
}

// GetSpiResourceQuotaInfo returns the available spi resources and its usage in a given test, namespace, and ResourceQuota name
func (s *SuiteController) GetSpiResourceQuotaInfo(test, namespace, resourceQuotaName string) error {
	rq, err := s.GetResourceQuota(namespace, resourceQuotaName)
	if err != nil {
		return err
	}

	available := rq.Status.Hard
	used := rq.Status.Used

	for resourceName, availableQuantity := range available {
		if usedQuantity, ok := used[resourceName]; ok {
			GinkgoWriter.Printf("test: %s, namespace: %s, resource: %s, available: %s, used: %s\n", test, namespace, resourceName, availableQuantity.String(), usedQuantity.String())
		}
	}

	return nil
}
