/*
Copyright 2022.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// WhenConditions defines requirements when specified build pipeline must be used.
// All conditions are connected via AND, whereas cases within any condition connected via OR.
// Example:
//
//	language: java
//	projectType: spring,quarkus
//	annotations:
//	   builder: gradle,maven
//
// which means that language is 'java' AND (project type is 'spring' OR 'quarkus') AND
// annotation 'builder' is present with value 'gradle' OR 'maven'.
type WhenCondition struct {
	// Defines component language to match, e.g. 'java'.
	// The value to compare with is taken from devfile.metadata.language field.
	// +kubebuilder:validation:Optional
	Language string `json:"language,omitempty"`

	// Defines type of project of the component to match, e.g. 'quarkus'.
	// The value to compare with is taken from devfile.metadata.projectType field.
	// +kubebuilder:validation:Optional
	ProjectType string `json:"projectType,omitempty"`

	// Defines if a Dockerfile should be present in the component.
	// Note, unset (nil) value is not the same as false (unset means skip the dockerfile check).
	// The value to compare with is taken from devfile components of image type.
	// +kubebuilder:validation:Optional
	DockerfileRequired *bool `json:"dockerfile,omitempty"`

	// Defines list of allowed component names to match, e.g. 'my-component'.
	// The value to compare with is taken from component.metadata.name field.
	// +kubebuilder:validation:Optional
	ComponentName string `json:"componentName,omitempty"`

	// Defines annotations to match.
	// The values to compare with are taken from component.metadata.annotations field.
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Defines labels to match.
	// The values to compare with are taken from component.metadata.labels field.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
}

// PipelineParam is a type to describe pipeline parameters.
// tektonapi.Param type is not used due to validation issues.
type PipelineParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PipelineSelector defines allowed build pipeline and conditions when it should be used.
type PipelineSelector struct {
	// Name of the selector item. Optional.
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// Build Pipeline to use if the selector conditions are met.
	// +kubebuilder:validation:Required
	PipelineRef tektonapi.PipelineRef `json:"pipelineRef"`

	// Extra arguments to add to the specified pipeline run.
	// +kubebuilder:validation:Optional
	// +listType=atomic
	PipelineParams []PipelineParam `json:"pipelineParams,omitempty"`

	// Defines the selector conditions when given build pipeline should be used.
	// All conditions are connected via AND, whereas cases within any condition connected via OR.
	// If the section is omitted, then the condition is considered true (usually used for fallback condition).
	// +kubebuilder:validation:Optional
	WhenConditions WhenCondition `json:"when,omitempty"`
}

// BuildPipelineSelectorSpec defines the desired state of BuildPipelineSelector
type BuildPipelineSelectorSpec struct {
	// Defines chain of pipeline selectors.
	// The first matching item is used.
	// +kubebuilder:validation:Required
	Selectors []PipelineSelector `json:"selectors"`
}

//+kubebuilder:object:root=true

// BuildPipelineSelector is the Schema for the BuildPipelineSelectors API
type BuildPipelineSelector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BuildPipelineSelectorSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// BuildPipelineSelectorList contains a list of BuildPipelineSelector
type BuildPipelineSelectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BuildPipelineSelector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BuildPipelineSelector{}, &BuildPipelineSelectorList{})
}
