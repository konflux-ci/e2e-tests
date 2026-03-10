package tekton

import (
	"context"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Candidate namespaces where Tekton Chains may be deployed.
var tektonChainsNamespaceCandidates = []string{
	"openshift-pipelines",
	"tekton-pipelines",
}

var (
	resolvedChainsNs   string
	resolvedChainsOnce sync.Once
	resolvedChainsErr  error
)

// GetTektonChainsNamespace detects which namespace contains the Tekton Chains
// controller by looking for pods with the app=tekton-chains-controller label.
// The result is cached for the lifetime of the process.
func (t *TektonController) GetTektonChainsNamespace() (string, error) {
	resolvedChainsOnce.Do(func() {
		for _, ns := range tektonChainsNamespaceCandidates {
			pods, err := t.KubeInterface().CoreV1().Pods(ns).List(
				context.Background(), metav1.ListOptions{
					LabelSelector: "app=tekton-chains-controller",
				})
			if err != nil {
				continue
			}
			if len(pods.Items) > 0 {
				resolvedChainsNs = ns
				return
			}
		}
		resolvedChainsErr = fmt.Errorf(
			"could not find tekton-chains-controller pods in any of: %v",
			tektonChainsNamespaceCandidates)
	})
	return resolvedChainsNs, resolvedChainsErr
}
