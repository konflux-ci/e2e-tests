package tekton

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRekorHost returns a rekorHost.
func (t *TektonController) GetRekorHost() (rekorHost string, err error) {
	namespace, err := t.GetTektonChainsNamespace()
	if err != nil {
		return "", err
	}

	api := t.KubeInterface().CoreV1().ConfigMaps(namespace)
	ctx := context.Background()

	cm, err := api.Get(ctx, "chains-config", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	rekorHost, ok := cm.Data["transparency.url"]
	if !ok || rekorHost == "" {
		rekorHost = "https://rekor.sigstore.dev"
	}
	return rekorHost, nil
}
