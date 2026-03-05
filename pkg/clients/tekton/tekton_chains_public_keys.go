package tekton

import (
	"context"
	"fmt"
	"os"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTektonChainsPublicKey returns a TektonChains public key.
// It checks the OpenShift Pipelines namespace first, then falls back to the
// upstream Tekton namespace for clusters using upstream Tekton instead of
// OpenShift Pipelines.
func (t *TektonController) GetTektonChainsPublicKey() ([]byte, error) {
	secretName := "public-key"
	dataKey := "cosign.pub"

	namespaces := []string{constants.TEKTON_CHAINS_NS}
	if os.Getenv(constants.TEST_ENVIRONMENT_ENV) == constants.UpstreamTestEnvironment {
		namespaces = append(namespaces, "tekton-pipelines")
	}

	for _, namespace := range namespaces {
		secret, err := t.KubeInterface().CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		publicKey := secret.Data[dataKey]
		if len(publicKey) < 1 {
			return nil, fmt.Errorf("the content of the public key '%s' in secret %s in %s namespace is empty", dataKey, secretName, namespace)
		}
		return publicKey, nil
	}

	return nil, fmt.Errorf("couldn't find the secret %s in any of these namespaces: %v", secretName, namespaces)
}
