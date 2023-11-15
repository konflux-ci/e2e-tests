package tekton

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// GetPipelineNameAndBundleRef returns the pipeline name and bundle reference from a pipelineRef
// https://tekton.dev/docs/pipelines/pipelineruns/#tekton-bundles
func GetPipelineNameAndBundleRef(pipelineRef *v1beta1.PipelineRef) (string, string) {
	var name string
	var bundleRef string

	// Prefer the v1 style
	if pipelineRef.Resolver != "" {
		for _, param := range pipelineRef.Params {
			switch param.Name {
			case "name":
				name = param.Value.StringVal
			case "bundle":
				bundleRef = param.Value.StringVal
			}
		}
	} else {
		// Support the v1beta1 style
		name = pipelineRef.Name
		bundleRef = pipelineRef.Bundle //nolint:all
	}

	return name, bundleRef
}
