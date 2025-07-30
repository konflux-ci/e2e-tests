package tekton

import (
	"context"
	"os"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRekorHost returns a rekorHost.
func (t *TektonController) GetRekorHost() (rekorHost string, err error) {
	var tektonChainsNs = constants.TEKTON_CHAINS_NS

	if os.Getenv("TEST_ENVIRONMENT") == "upstream" {
		tektonChainsNs = "tekton-pipelines"
	}
	api := t.KubeInterface().CoreV1().ConfigMaps(tektonChainsNs)
	ctx := context.Background()

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
