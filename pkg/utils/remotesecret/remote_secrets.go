package remotesecret

import (
	"context"

	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CreateRemoteSecret creates a RemoteSecret object
func (s *RemoteSecretController) CreateRemoteSecret(name, namespace string, targets []rs.RemoteSecretTarget) (*rs.RemoteSecret, error) {
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

	remoteSecret.Spec.Targets = targets

	err := s.KubeRest().Create(context.TODO(), &remoteSecret)
	if err != nil {
		return nil, err
	}
	return &remoteSecret, nil
}

// CreateRemoteSecretWithLabels creates a RemoteSecret object with specified labels
func (s *RemoteSecretController) CreateRemoteSecretWithLabels(name, namespace string, targetSecretName string, labels map[string]string) (*rs.RemoteSecret, error) {
	remoteSecret := rs.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: rs.RemoteSecretSpec{
			Secret: rs.LinkableSecretSpec{
				Name: targetSecretName,
			},
		},
	}

	remoteSecret.ObjectMeta.Labels = labels

	err := s.KubeRest().Create(context.TODO(), &remoteSecret)
	if err != nil {
		return nil, err
	}
	return &remoteSecret, nil
}

// GetRemoteSecret returns the requested RemoteSecret object
func (s *RemoteSecretController) GetRemoteSecret(name, namespace string) (*rs.RemoteSecret, error) {
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
func (s *RemoteSecretController) GetTargetSecretName(targets []rs.TargetStatus, targetNamespace string) string {
	targetSecretName := ""

	for _, t := range targets {
		if t.Namespace == targetNamespace {
			return t.SecretName
		}
	}

	return targetSecretName
}

// CreateUploadSecret creates an Upload secret object to inject data in a Remote Secret
func (s *RemoteSecretController) CreateUploadSecret(name, namespace string, remoteSecretName string, stringData map[string]string) (*corev1.Secret, error) {
	uploadSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				rs.UploadSecretLabel: "remotesecret",
			},
			Annotations: map[string]string{
				rs.RemoteSecretNameAnnotation: remoteSecretName,
			},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: stringData,
	}

	err := s.KubeRest().Create(context.TODO(), &uploadSecret)
	if err != nil {
		return nil, err
	}
	return &uploadSecret, nil
}
