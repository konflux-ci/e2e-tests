package tekton

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTektonChainsPublicKey returns a TektonChains public key.
func (t *TektonController) GetTektonChainsPublicKey() ([]byte, error) {
	namespace, err := t.GetTektonChainsNamespace()
	if err != nil {
		return nil, err
	}

	secretName := "public-key"
	dataKey := "cosign.pub"

	secret, err := t.KubeInterface().CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("couldn't get the secret %s from %s namespace: %+v", secretName, namespace, err)
	}
	publicKey := secret.Data[dataKey]
	if len(publicKey) < 1 {
		return nil, fmt.Errorf("the content of the public key '%s' in secret %s in %s namespace is empty", dataKey, secretName, namespace)
	}
	return publicKey, err
}
