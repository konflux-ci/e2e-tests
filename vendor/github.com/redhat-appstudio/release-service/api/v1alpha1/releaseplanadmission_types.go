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

// ReleasePlanAdmissionSpec defines the desired state of ReleasePlanAdmission.
type ReleasePlanAdmissionSpec struct {
	// DisplayName is the long name of the ReleasePlanAdmission
	// +optional
	DisplayName string `json:"displayName"`

	// Application is a reference to the application to be released in the managed namespace
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Application string `json:"application"`

	// Origin references where the release requests should come from
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Origin string `json:"origin"`

	// Environment defines which Environment will be used to release the application
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	Environment string `json:"environment,omitempty"`

	// Release Strategy defines which strategy will be used to release the application
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	ReleaseStrategy string `json:"releaseStrategy"`
}

// ReleasePlanAdmissionStatus defines the observed state of ReleasePlanAdmission.
type ReleasePlanAdmissionStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display Name",type=string,priority=1,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.application`
// +kubebuilder:printcolumn:name="Environment",type=string,JSONPath=`.spec.environment`
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Origin",type=string,JSONPath=`.spec.origin`

// ReleasePlanAdmission is the Schema for the ReleasePlanAdmissions API.
type ReleasePlanAdmission struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleasePlanAdmissionSpec   `json:"spec,omitempty"`
	Status ReleasePlanAdmissionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReleasePlanAdmissionList contains a list of ReleasePlanAdmission.
type ReleasePlanAdmissionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReleasePlanAdmission `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReleasePlanAdmission{}, &ReleasePlanAdmissionList{})
}
