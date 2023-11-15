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

func NewBundleResolverPipelineRef(name string, bundleRef string) *v1beta1.PipelineRef {
	return &v1beta1.PipelineRef{
		ResolverRef: v1beta1.ResolverRef{
			Resolver: "bundles",
			Params: []v1beta1.Param{
				{Name: "name", Value: v1beta1.ParamValue{StringVal: name, Type: v1beta1.ParamTypeString}},
				{Name: "bundle", Value: v1beta1.ParamValue{StringVal: bundleRef, Type: v1beta1.ParamTypeString}},
				{Name: "kind", Value: v1beta1.ParamValue{StringVal: "pipeline", Type: v1beta1.ParamTypeString}},
			},
		},
	}
}
