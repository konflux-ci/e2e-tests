package release

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Contains all methods related with secret objects CRUD operations.
type SecretsInterface interface {
	//Creates a registry secret.
	CreateRegistryJsonSecret(name, namespace, authKey, keyName string) (*corev1.Secret, error)
}

// CreateRegistryJsonSecret creates a secret for registry repository in namespace given with key passed.
func (r *releaseFactory) CreateRegistryJsonSecret(name, namespace, authKey, keyName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{".dockerconfigjson": []byte(fmt.Sprintf("{\"auths\":{\"quay.io\":{\"username\":\"%s\",\"password\":\"%s\",\"auth\":\"dGVzdDp0ZXN0\",\"email\":\"\"}}}", keyName, authKey))},
	}
	err := r.KubeRest().Create(context.TODO(), secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}
