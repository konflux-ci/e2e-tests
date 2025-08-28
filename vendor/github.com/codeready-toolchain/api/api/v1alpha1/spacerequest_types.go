package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// SpaceRequestLabelKey is a label on the subSpace, and will hold the name of the SpaceRequest that created the subSpace resource.
	SpaceRequestLabelKey = LabelKeyPrefix + "spacerequest"
	// SpaceRequestNamespaceLabelKey is a label on the subSpace, and will hold the namespace of the SpaceRequest that created the subSpace resource.
	SpaceRequestNamespaceLabelKey = LabelKeyPrefix + "spacerequest-namespace"
	// SpaceRequestProvisionedNamespaceLabelKey is a label on the secret that was created to provide access to a specific namespace provisioned by the SpaceRequest.
	SpaceRequestProvisionedNamespaceLabelKey = LabelKeyPrefix + "spacerequest-provisioned-namespace"

	// AdminServiceAccountName is the service account holding the token that grants admin permissions for the namespace provisioned by the SpaceRequest.
	AdminServiceAccountName = "namespace-manager"
)

// SpaceRequestSpec defines the desired state of Space
// +k8s:openapi-gen=true
type SpaceRequestSpec struct {
	// TierName is a required property introduced to retain the name of the tier
	// for which this Space is provisioned.
	TierName string `json:"tierName"`

	// TargetClusterRoles one or more label keys that define a set of clusters
	// where the Space can be provisioned.
	// The target cluster has to match ALL the roles defined in this field in order for the space to be provisioned there.
	// +optional
	// +listType=atomic
	TargetClusterRoles []string `json:"targetClusterRoles,omitempty"`

	// DisableInheritance indicates whether or not SpaceBindings from the parent-spaces are
	// automatically inherited to all sub-spaces in the tree.
	//
	// Set to True to disable SpaceBinding inheritance from the parent-spaces.
	// Default is False.
	// +optional
	DisableInheritance bool `json:"disableInheritance,omitempty"`
}

// SpaceRequestStatus defines the observed state of Space
// +k8s:openapi-gen=true
type SpaceRequestStatus struct {

	// TargetClusterURL The API URL of the cluster where Space is currently provisioned
	// Can be empty if provisioning did not start or failed
	// The URL is just for informative purposes for developers and controllers that are placed in member clusters.
	// +optional
	TargetClusterURL string `json:"targetClusterURL,omitempty"`

	// NamespaceAccess is the list with the provisioned namespace and secret to access it
	// +listType=atomic
	// +optional
	NamespaceAccess []NamespaceAccess `json:"namespaceAccess,omitempty"`

	// Conditions is an array of SpaceRequest conditions
	// Supported condition types:
	// Provisioning, SpaceNotReady and Ready
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// NamespaceAccess defines the name of the namespace and the secret reference to access it
type NamespaceAccess struct {
	// Name is the corresponding name of the provisioned namespace
	Name string `json:"name"`
	// SecretRef is the name of the secret with a SA token that has admin-like
	// (or whatever we set in the tier template) permissions in the namespace
	SecretRef string `json:"secretRef"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SpaceRequest is the Schema for the space request API
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Tier",type="string",JSONPath=`.spec.tierName`
// +kubebuilder:printcolumn:name="TargetClusterURL",type="string",JSONPath=`.status.targetClusterURL`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="SpaceRequest"
type SpaceRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpaceRequestSpec   `json:"spec,omitempty"`
	Status SpaceRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SpaceRequestList contains a list of SpaceRequests
type SpaceRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpaceRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpaceRequest{}, &SpaceRequestList{})
}
