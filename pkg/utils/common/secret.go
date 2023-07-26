package common

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	. "github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Creates a new secret in a specified namespace
func (s *SuiteController) CreateSecret(ns string, secret *corev1.Secret) (*corev1.Secret, error) {
	return s.KubeInterface().CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
}

// Check if a secret exists, return secret and error
func (s *SuiteController) GetSecret(ns string, name string) (*corev1.Secret, error) {
	return s.KubeInterface().CoreV1().Secrets(ns).Get(context.TODO(), name, metav1.GetOptions{})
}

// Deleted a secret in a specified namespace
func (s *SuiteController) DeleteSecret(ns string, name string) error {
	return s.KubeInterface().CoreV1().Secrets(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// Remove allSecret from a given namespace. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllSecretsInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &corev1.Secret{}, client.InNamespace(namespace))
}

// Links a secret to a specified serviceaccount, if argument addImagePullSecrets is true secret will be added also to ImagePullSecrets of SA.
func (s *SuiteController) LinkSecretToServiceAccount(ns, secret, serviceaccount string, addImagePullSecrets bool) error {
	timeout := 20 * time.Second
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		serviceAccountObject, err := s.KubeInterface().CoreV1().ServiceAccounts(ns).Get(context.TODO(), serviceaccount, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, credentialSecret := range serviceAccountObject.Secrets {
			if credentialSecret.Name == secret {
				// The secret is present in the service account, no updates needed
				return true, nil
			}
		}
		serviceAccountObject.Secrets = append(serviceAccountObject.Secrets, corev1.ObjectReference{Name: secret})
		if addImagePullSecrets {
			serviceAccountObject.ImagePullSecrets = append(serviceAccountObject.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
		}
		_, err = s.KubeInterface().CoreV1().ServiceAccounts(ns).Update(context.TODO(), serviceAccountObject, metav1.UpdateOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
}

// UnlinkSecretFromServiceAcocount unlinks secret from service account
func (s *SuiteController) UnlinkSecretFromServiceAccount(namespace, secretName, serviceAccount string, rmImagePullSecrets bool) error {
	serviceAccountObject, err := s.KubeInterface().CoreV1().ServiceAccounts(namespace).Get(context.TODO(), serviceAccount, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for index, secret := range serviceAccountObject.Secrets {
		if secret.Name == secretName {
			serviceAccountObject.Secrets = append(serviceAccountObject.Secrets[:index], serviceAccountObject.Secrets[index+1:]...)
			break
		}
	}

	if rmImagePullSecrets {
		for index, secret := range serviceAccountObject.ImagePullSecrets {
			if secret.Name == secretName {
				serviceAccountObject.ImagePullSecrets = append(serviceAccountObject.ImagePullSecrets[:index], serviceAccountObject.ImagePullSecrets[index+1:]...)
				break
			}
		}
	}
	_, err = s.KubeInterface().CoreV1().ServiceAccounts(namespace).Update(context.TODO(), serviceAccountObject, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// CreateRegistryAuthSecret create a docker registry secret in a given ns
func (s *SuiteController) CreateRegistryAuthSecret(secretName, namespace, secretStringData string) (*corev1.Secret, error) {
	rawDecodedTextStringData, err := base64.StdEncoding.DecodeString(secretStringData)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type:       corev1.SecretTypeDockerConfigJson,
		StringData: map[string]string{corev1.DockerConfigJsonKey: string(rawDecodedTextStringData)},
	}
	er := s.KubeRest().Create(context.TODO(), secret)
	if er != nil {
		return nil, er
	}
	return secret, nil
}

// CreateRegistryJsonSecret creates a secret for registry repository in namespace given with key passed.
func (s *SuiteController) CreateRegistryJsonSecret(name, namespace, authKey, keyName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{".dockerconfigjson": []byte(fmt.Sprintf("{\"auths\":{\"quay.io\":{\"username\":\"%s\",\"password\":\"%s\",\"auth\":\"dGVzdDp0ZXN0\",\"email\":\"\"}}}", keyName, authKey))},
	}
	err := s.KubeRest().Create(context.TODO(), secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// AddRegistryAuthSecretToSA adds registry auth secret to service account
func (s *SuiteController) AddRegistryAuthSecretToSA(registryAuth, namespace string) error {
	quayToken := utils.GetEnv(registryAuth, "")
	if quayToken == "" {
		return errors.New("failed to get registry auth secret")
	}

	_, err := s.CreateRegistryAuthSecret(RegistryAuthSecretName, namespace, quayToken)
	if err != nil {
		return err
	}

	err = s.LinkSecretToServiceAccount(namespace, RegistryAuthSecretName, DefaultPipelineServiceAccount, true)
	if err != nil {
		return err
	}

	return nil
}
