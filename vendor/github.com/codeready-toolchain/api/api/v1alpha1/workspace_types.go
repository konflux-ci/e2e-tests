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

	// AvailableRoles contains the roles for this tier. For example, "admin|contributor|maintainer".
	// +listType=atomic
	// +optional
	AvailableRoles []string `json:"availableRoles,omitempty"`

	// Bindings enumerates the permissions that have been granted to users within the current workspace, and actions that can be applied to those permissions.
	// +listType=atomic
	// +optional
	Bindings []Binding `json:"bindings,omitempty"`
}

// Binding defines a user role in a given workspace,
// and available actions that can be performed on the role
// +k8s:openapi-gen=true
type Binding struct {
	// MasterUserRecord is the name of the user that has access to the workspace.
	// This field is immutable via a validating webhook.
	MasterUserRecord string `json:"masterUserRecord,omitempty"`

	// Role is the role of the user in the current workspace. For example "admin" for the user that has all permissions on the current workspace.
	Role string `json:"role,omitempty"`

	// AvailableActions is a list of actions that can be performed on the binding.
	// Available values:
	// - "update" when the role in the current binding can be changed
	// - "delete" when the current binding can be deleted
	// - "override" when the current binding is inherited from a parent workspace, it cannot be updated, but it can be overridden by creating a new binding containing the same MasterUserRecord but different role in the subworkspace.
	// +listType=atomic
	// +optional
	AvailableActions []string `json:"availableActions,omitempty"`

	// BindingRequest provides the name and namespace of the SpaceBindingRequest that generated the SpaceBinding resource.
	// It's available only if the binding was generated using the SpaceBindingRequest mechanism.
	// +optional
	BindingRequest *BindingRequest `json:"bindingRequest,omitempty"`
}

// BindingRequest contains the name and the namespace where of the associated SpaceBindingRequest.
// +k8s:openapi-gen=true
type BindingRequest struct {
	// Name of the SpaceBindingRequest that generated the SpaceBinding resource.
	Name string `json:"name"`
	// Namespace of the SpaceBindingRequest that generated the SpaceBinding resource.
	Namespace string `json:"namespace"`
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
