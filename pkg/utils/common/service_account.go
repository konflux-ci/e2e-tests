package common

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *SuiteController) GetServiceAccount(saName, namespace string) (*corev1.ServiceAccount, error) {
	return s.KubeInterface().CoreV1().ServiceAccounts(namespace).Get(context.TODO(), saName, metav1.GetOptions{})
}

func (s *SuiteController) ServiceaccountPresent(saName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := s.GetServiceAccount(saName, namespace)
		if err != nil {
			return false, nil
		}
		return true, nil
	}
}

// CreateServiceAccount creates a service account with the provided name and namespace using the given list of secrets.
func (s *SuiteController) CreateServiceAccount(name, namespace string, serviceAccountSecretList []corev1.ObjectReference) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Secrets: serviceAccountSecretList,
	}
	return s.KubeInterface().CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
}

// DeleteAllServiceAccountsInASpecificNamespace deletes all ServiceAccount from a given namespace
func (h *SuiteController) DeleteAllServiceAccountsInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &corev1.ServiceAccount{}, client.InNamespace(namespace))
}
