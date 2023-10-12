package gitops

import (
	"context"
	"fmt"

	codereadytoolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSpaces returns a list of spaces in given namespace.
func (g *GitopsController) GetSpaces(namespace string) (*codereadytoolchainv1alpha1.SpaceList, error) {
	spaceList := &codereadytoolchainv1alpha1.SpaceList{}

	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := g.KubeRest().List(context.Background(), spaceList, opts...)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error occurred while trying to list spaces in %s namespace: %w", namespace, err)
	}

	return spaceList, nil
}
