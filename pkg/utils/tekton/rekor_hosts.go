package tekton

import (
	"context"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRekorHost returns a rekorHost.
func (t *TektonController) GetRekorHost() (rekorHost string, err error) {
	api := t.KubeInterface().CoreV1().ConfigMaps(constants.TEKTON_CHAINS_DEPLOYMENT_NS)
	ctx := context.TODO()

	cm, err := api.Get(ctx, "chains-config", metav1.GetOptions{})
	if err != nil {
		return
	}

	rekorHost, ok := cm.Data["transparency.url"]
	if !ok || rekorHost == "" {
		rekorHost = "https://rekor.sigstore.dev"
	}
	return
}
