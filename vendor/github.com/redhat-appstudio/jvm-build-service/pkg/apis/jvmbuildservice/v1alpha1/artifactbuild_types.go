package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ArtifactBuildSpec struct {
	// GAV is the groupID:artifactID:version tuple seen in maven pom.xml files
	GAV string `json:"gav,omitempty"`
}

type ArtifactBuildStatus struct {
	//TODO: conditions?
	State   string  `json:"state,omitempty"`
	Message string  `json:"message,omitempty"`
	SCMInfo SCMInfo `json:"scm,omitempty"`
}

//type ArtifactBuildState string

const (
	// ArtifactBuildStateNew A new resource that has not been acted on by the operator
	ArtifactBuildStateNew = "ArtifactBuildNew"
	// ArtifactBuildStateDiscovering The discovery pipeline is running to try and figure out how to build this artifact
	ArtifactBuildStateDiscovering = "ArtifactBuildDiscovering"
	// ArtifactBuildStateMissing The discovery pipeline failed to find a way to build this
	ArtifactBuildStateMissing = "ArtifactBuildMissing"
	// ArtifactBuildStateBuilding The build is running
	ArtifactBuildStateBuilding = "ArtifactBuildBuilding"
	// ArtifactBuildStateFailed The build failed
	ArtifactBuildStateFailed = "ArtifactBuildFailed"
	// ArtifactBuildStateComplete The build completed successfully, the resource can be removed
	ArtifactBuildStateComplete = "ArtifactBuildComplete"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=artifactbuilds,scope=Namespaced
// +kubebuilder:printcolumn:name="GAV",type=string,JSONPath=`.spec.gav`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// ArtifactBuild TODO provide godoc description
type ArtifactBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArtifactBuildSpec   `json:"spec"`
	Status ArtifactBuildStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ArtifactBuildList contains a list of ArtifactBuild
type ArtifactBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArtifactBuild `json:"items"`
}
