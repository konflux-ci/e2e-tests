package common

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetResourceQuota returns the ResourceQuota object from a given namespace and ResourceQuota name
func (s *SuiteController) GetResourceQuota(namespace, ResourceQuotaName string) (*corev1.ResourceQuota, error) {
	return s.KubeInterface().CoreV1().ResourceQuotas(namespace).Get(context.Background(), ResourceQuotaName, metav1.GetOptions{})
}

// GetResourceQuotaInfo returns the available resources and its usage in a given test, namespace, and ResourceQuota name
func (s *SuiteController) GetResourceQuotaInfo(test, namespace, resourceQuotaName string) error {
	rq, err := s.GetResourceQuota(namespace, resourceQuotaName)
	if err != nil {
		GinkgoWriter.Printf("failed to get ResourceQuota %s in namespace %s: %v\n", resourceQuotaName, namespace, err)
		return err
	}

	notFound := false
	available := rq.Status.Hard
	used := rq.Status.Used

	for resourceName, availableQuantity := range available {
		if usedQuantity, ok := used[resourceName]; ok {
			GinkgoWriter.Printf("test: %s, namespace: %s, resourceQuota: %s, resource: %s, available: %s, used: %s\n", test, namespace, resourceQuotaName, resourceName, availableQuantity.String(), usedQuantity.String())
		} else {
			notFound = true
		}
	}

	// something went wrong
	if len(available) == 0 || len(used) == 0 || notFound {
		GinkgoWriter.Printf("test: %s, namespace: %s, resourceQuota: %s, available resources: %s, used resources: %s\n", test, namespace, resourceQuotaName, available, used)
	}

	return nil
}
