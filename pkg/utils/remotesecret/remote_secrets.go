package remotesecret

import (
	"context"

	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CreateRemoteSecret creates a RemoteSecret object
func (s *RemoteSecretController) CreateRemoteSecret(name, namespace string, targets []rs.RemoteSecretTarget, secretType v1.SecretType, labels map[string]string) (*rs.RemoteSecret, error) {
	remoteSecret := rs.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: rs.RemoteSecretSpec{
			Secret: rs.LinkableSecretSpec{
				GenerateName: "some-secret-",
				Type:         secretType,
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

// BuildSecret returns a specific secret for remote secret usage
func (s *RemoteSecretController) BuildSecret(remoteSecretName string, secretType v1.SecretType, data map[string]string) *v1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
			Labels: map[string]string{
				"appstudio.redhat.com/upload-secret": "remotesecret",
			},
			Annotations: map[string]string{
				"appstudio.redhat.com/remotesecret-name": remoteSecretName,
			},
		},
		Type:       secretType,
		StringData: data,
	}

	return secret
}
