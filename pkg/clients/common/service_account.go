package common

import (
	"context"
	"fmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *SuiteController) GetServiceAccount(saName, namespace string) (*corev1.ServiceAccount, error) {
	return s.KubeInterface().CoreV1().ServiceAccounts(namespace).Get(context.Background(), saName, metav1.GetOptions{})
}

func (s *SuiteController) ServiceAccountPresent(saName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := s.GetServiceAccount(saName, namespace)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("failed to get service account %s in namespace %s: %+v\n", saName, namespace, err)
			return false, nil
		}
		return true, nil
	}
}

// CreateServiceAccount creates a service account with the provided name and namespace using the given list of secrets.
func (s *SuiteController) CreateServiceAccount(name, namespace string, serviceAccountSecretList []corev1.ObjectReference, labels map[string]string) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Secrets: serviceAccountSecretList,
	}
	return s.KubeInterface().CoreV1().ServiceAccounts(namespace).Create(context.Background(), serviceAccount, metav1.CreateOptions{})
}

// IsSecretLinkedToServiceAccount checks whether a secret is linked to a service account
func (s *SuiteController) IsSecretLinkedToServiceAccount(serviceAccountName, namespace, secretName string) (bool, error) {
	serviceAccount, err := s.GetServiceAccount(serviceAccountName, namespace)
	if err != nil {
		return false, err
	}
	secretLinked := false
	for _, secretRef := range serviceAccount.Secrets {
		if secretRef.Name == secretName {
			fmt.Printf("found secret %s is linked to service account %s\n", secretName, serviceAccountName)
			secretLinked = true
			break
		}
	}
	return secretLinked, nil
}

// DeleteAllServiceAccountsInASpecificNamespace deletes all ServiceAccount from a given namespace
func (h *SuiteController) DeleteAllServiceAccountsInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.Background(), &corev1.ServiceAccount{}, client.InNamespace(namespace))
}
