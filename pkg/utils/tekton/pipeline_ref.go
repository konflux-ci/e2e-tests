package tekton

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// GetPipelineNameAndBundleRef returns the pipeline name and bundle reference from a pipelineRef
// https://tekton.dev/docs/pipelines/pipelineruns/#tekton-bundles
func GetPipelineNameAndBundleRef(pipelineRef *v1.PipelineRef) (string, string) {
	var name string
	var bundleRef string

	for _, param := range pipelineRef.Params {
		switch param.Name {
		case "name":
			name = param.Value.StringVal
		case "bundle":
			bundleRef = param.Value.StringVal
		}
	}

	return name, bundleRef
}

func NewBundleResolverPipelineRef(name string, bundleRef string) *v1.PipelineRef {
	return &v1.PipelineRef{
		ResolverRef: v1.ResolverRef{
			Resolver: "bundles",
			Params: []v1.Param{
				{Name: "name", Value: v1.ParamValue{StringVal: name, Type: v1.ParamTypeString}},
				{Name: "bundle", Value: v1.ParamValue{StringVal: bundleRef, Type: v1.ParamTypeString}},
				{Name: "kind", Value: v1.ParamValue{StringVal: "pipeline", Type: v1.ParamTypeString}},
			},
		},
	}
}
