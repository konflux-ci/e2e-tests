package tekton

import (
	"context"
	"fmt"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k KubeController) GetTektonChainsPublicKey() ([]byte, error) {
	namespace := constants.TEKTON_CHAINS_KEY_NS
	secretName := "public-key"
	dataKey := "cosign.pub"

	secret, err := k.Tektonctrl.KubeInterface().CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("couldn't get the secret %s from %s namespace: %+v", secretName, namespace, err)
	}
	publicKey := secret.Data[dataKey]
	if len(publicKey) < 1 {
		return nil, fmt.Errorf("the content of the public key '%s' in secret %s in %s namespace is empty", dataKey, secretName, namespace)
	}
	return publicKey, err
}
