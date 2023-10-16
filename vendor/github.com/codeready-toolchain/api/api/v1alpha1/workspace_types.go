package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkspaceStatus defines the observed state of a Workspace
// +k8s:openapi-gen=true
type WorkspaceStatus struct {
	// The list of namespaces belonging to the Workspace.
	// +optional
	// +listType=atomic
	Namespaces []SpaceNamespace `json:"namespaces,omitempty"`

	// Owner the name of the UserSignup that owns the workspace. It’s the user who is being charged
	// for the usage and whose quota is used for the workspace. There is only one user for this kind
	// of relationship and it can be transferred to someone else during the lifetime of the workspace.
	// By default, it’s the creator who becomes the owner as well.
	// +optional
	Owner string `json:"owner,omitempty"`

	// Role defines what kind of permissions the user has in the given workspace.
	// +optional
	Role string `json:"role,omitempty"`

	// Type defines the type of workspace. For example, "home" for a user's given workspace upon first
	// signing up. It is currently valid for this value to be empty.
	// +optional
	Type string `json:"type,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// Workspace is the Schema for the workspaces API but it is only for use by the Proxy. There will be
// no actual Workspace CRs in the host/member clusters. The CRD will be installed in member clusters
// for API discovery purposes only. The schema will be used by the proxy's workspace lister API.
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Owner",type="string",JSONPath=`.status.owner`
// +kubebuilder:printcolumn:name="Role",type="string",JSONPath=`.status.role`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Workspace"
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status WorkspaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkspaceList contains a list of Workspaces
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}
