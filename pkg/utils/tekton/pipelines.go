package tekton

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreatePipeline creates a tekton pipeline and returns the pipeline or an error
func (t *TektonController) CreatePipeline(pipeline *v1beta1.Pipeline, ns string) (*v1beta1.Pipeline, error) {
	return t.PipelineClient().TektonV1beta1().Pipelines(ns).Create(context.TODO(), pipeline, metav1.CreateOptions{})
}

// DeletePipeline removes the pipeline from given namespace.
func (t *TektonController) DeletePipeline(name, ns string) error {
	return t.PipelineClient().TektonV1beta1().Pipelines(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
