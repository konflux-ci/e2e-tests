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
)

// ReleasePlanSpec defines the desired state of ReleasePlan.
type ReleasePlanSpec struct {
	// DisplayName is the long name of the ReleasePlan
	// +optional
	DisplayName string `json:"displayName"`

	// Application is a reference to the application to be released in the managed namespace
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Application string `json:"application"`

	// Target references where to send the release requests
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Target string `json:"target"`
}

// ReleasePlanStatus defines the observed state of ReleasePlan.
type ReleasePlanStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display Name",type=string,priority=1,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.application`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target`

// ReleasePlan is the Schema for the ReleasePlans API.
type ReleasePlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleasePlanSpec   `json:"spec,omitempty"`
	Status ReleasePlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReleasePlanList contains a list of ReleasePlan.
type ReleasePlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReleasePlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReleasePlan{}, &ReleasePlanList{})
}
