package controller

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

// GetPipeline return a pipeline from cluster and if don't exist returns an error
func (h *CommonSuiteController) GetPipeline(name string, namespace string) (*v1beta1.Pipeline, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	pipeline := &v1beta1.Pipeline{}

	if err := h.KubeRest().Get(context.TODO(), namespacedName, pipeline); err != nil {
		return nil, err
	}
	return pipeline, nil
}
