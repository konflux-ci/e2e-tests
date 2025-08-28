/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

// Param defines the parameters for a given resolver in PipelineRef
type Param struct {
	// Name is the name of the parameter
	Name string `json:"name"`

	// Value is the value of the parameter
	Value string `json:"value"`
}

// PipelineRef represents a reference to a Pipeline using a resolver.
// +kubebuilder:object:generate=true
type PipelineRef struct {
	// Resolver is the name of a Tekton resolver to be used (e.g. git)
	Resolver string `json:"resolver"`

	// Params is a slice of parameters for a given resolver
	Params []Param `json:"params"`
}

// Pipeline contains a reference to a Pipeline and the name of the service account to use while executing it.
// +kubebuilder:object:generate=true
type Pipeline struct {
	// PipelineRef is the reference to the Pipeline
	PipelineRef PipelineRef `json:"pipelineRef"`

	// ServiceAccountName is the ServiceAccount to use during the execution of the Pipeline
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Timeouts defines the different Timeouts to use in the PipelineRun execution
	// +optional
	Timeouts tektonv1.TimeoutFields `json:"timeouts,omitempty"`
}

// ParameterizedPipeline is an extension of the Pipeline struct, adding an array of parameters that will be passed to
// the Pipeline.
// +kubebuilder:object:generate=true
type ParameterizedPipeline struct {
	Pipeline `json:",inline"`

	// Params is a slice of parameters for a given resolver
	// +optional
	Params []Param `json:"params,omitempty"`
}

// ToTektonPipelineRef converts a PipelineRef object to Tekton's own PipelineRef type and returns it.
func (pr *PipelineRef) ToTektonPipelineRef() *tektonv1.PipelineRef {
	params := tektonv1.Params{}

	for _, p := range pr.Params {
		params = append(params, tektonv1.Param{
			Name: p.Name,
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: p.Value,
			},
		})
	}

	tektonPipelineRef := &tektonv1.PipelineRef{
		ResolverRef: tektonv1.ResolverRef{
			Resolver: tektonv1.ResolverName(pr.Resolver),
			Params:   params,
		},
	}

	return tektonPipelineRef
}

// GetTektonParams returns the ParameterizedPipeline []Param as []tektonv1.Param.
func (prp *ParameterizedPipeline) GetTektonParams() []tektonv1.Param {
	params := []tektonv1.Param{}

	for _, param := range prp.Params {
		params = append(params, tektonv1.Param{
			Name: param.Name,
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: param.Value,
			},
		})
	}

	return params
}

// IsClusterScoped returns whether the PipelineRef uses a cluster resolver or not.
func (pr *PipelineRef) IsClusterScoped() bool {
	return pr.Resolver == "cluster"
}
