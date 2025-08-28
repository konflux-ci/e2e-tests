package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RebuiltArtifactSpec struct {
	// The GAV of the rebuilt artifact
	GAV    string `json:"gav,omitempty"`
	Image  string `json:"image,omitempty"`
	Digest string `json:"digest,omitempty"`
}

type RebuiltArtifactStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=rebuiltartifacts,scope=Namespaced
// +kubebuilder:printcolumn:name="GAV",type=string,JSONPath=`.spec.gav`
// RebuiltArtifact An artifact that has been rebuilt and deployed to S3 or a Container registry
type RebuiltArtifact struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RebuiltArtifactSpec   `json:"spec"`
	Status RebuiltArtifactStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RebuiltArtifactList contains a list of RebuiltArtifact
type RebuiltArtifactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RebuiltArtifact `json:"items"`
}
