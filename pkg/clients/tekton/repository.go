package tekton

import (
	"context"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

// GetRepositoryParams returns a repository params list
func (t *TektonController) GetRepositoryParams(name, namespace string) ([]pacv1alpha1.Params, error) {
	ctx := context.Background()
	repositoryObj := &pacv1alpha1.Repository{}
	err := t.KubeRest().Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, repositoryObj)
	if err != nil {
		return nil, err
	}

	// Check if Params is nil to avoid panic
	if repositoryObj.Spec.Params == nil {
		return []pacv1alpha1.Params{}, nil
	}

	return *repositoryObj.Spec.Params, nil
}
