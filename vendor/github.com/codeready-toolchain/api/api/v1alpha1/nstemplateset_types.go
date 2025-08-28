package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	OwnerLabelKey       = LabelKeyPrefix + "owner"
	SpaceLabelKey       = LabelKeyPrefix + "space"
	TypeLabelKey        = LabelKeyPrefix + "type"
	TemplateRefLabelKey = LabelKeyPrefix + "templateref"
	TierLabelKey        = LabelKeyPrefix + "tier"
	ProviderLabelKey    = LabelKeyPrefix + "provider"
	ProviderLabelValue  = "codeready-toolchain"

	LastAppliedSpaceRolesAnnotationKey = LabelKeyPrefix + "last-applied-space-roles"
)

// These are valid status condition reasons of a NSTemplateSet
const (
	NSTemplateSetProvisionedReason                       = provisionedReason
	NSTemplateSetProvisioningReason                      = provisioningReason
	NSTemplateSetUnableToProvisionReason                 = "UnableToProvision"
	NSTemplateSetUnableToProvisionNamespaceReason        = "UnableToProvisionNamespace"
	NSTemplateSetUnableToProvisionClusterResourcesReason = "UnableToProvisionClusteResources"
	NSTemplateSetUnableToProvisionSpaceRolesReason       = "UnableToProvisionSpaceRoles"
	NSTemplateSetTerminatingReason                       = terminatingReason
	NSTemplateSetTerminatingFailedReason                 = terminatingFailedReason
	NSTemplateSetUpdatingReason                          = updatingReason
	NSTemplateSetUpdateFailedReason                      = "UpdateFailed"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NSTemplateSetSpec defines the desired state of NSTemplateSet
// +k8s:openapi-gen=true
type NSTemplateSetSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// The name of the tier represented by this template set
	TierName string `json:"tierName"`

	// The namespace templates
	// +listType=atomic
	Namespaces []NSTemplateSetNamespace `json:"namespaces"`

	// the cluster resources template (for cluster-wide quotas, etc.)
	// +optional
	ClusterResources *NSTemplateSetClusterResources `json:"clusterResources,omitempty"`

	// the role template and the users to whom the templates should be applied to
	// +optional
	// +listType=atomic
	SpaceRoles []NSTemplateSetSpaceRole `json:"spaceRoles,omitempty"`
}

// NSTemplateSetNamespace the namespace definition in an NSTemplateSet resource
// +k8s:openapi-gen=true
type NSTemplateSetNamespace struct {

	// TemplateRef The name of the TierTemplate resource which exists in the host cluster and which contains the template to use
	TemplateRef string `json:"templateRef"`
}

// NSTemplateSetClusterResources defines the cluster-scoped resources associated with a given user
// +k8s:openapi-gen=true
type NSTemplateSetClusterResources struct {

	// TemplateRef The name of the TierTemplate resource which exists in the host cluster and which contains the template to use
	TemplateRef string `json:"templateRef"`
}

// NSTemplateSetSpaceRole the role template and the users to whom the templates should be applied to
// +k8s:openapi-gen=true
type NSTemplateSetSpaceRole struct {

	// TemplateRef The name of the TierTemplate resource which exists in the host cluster and which contains the template to use
	TemplateRef string `json:"templateRef"`

	// Usernames the usernames to which the template applies
	// +listType=atomic
	Usernames []string `json:"usernames"`
}

// NSTemplateSetStatus defines the observed state of NSTemplateSet
// +k8s:openapi-gen=true
type NSTemplateSetStatus struct {
	// ProvisionedNamespaces is a list of Namespaces that were provisioned by the NSTemplateSet.
	// +optional
	// +listType=atomic
	ProvisionedNamespaces []SpaceNamespace `json:"provisionedNamespaces,omitempty"`

	// Conditions is an array of current NSTemplateSet conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NSTemplateSet defines user environment via templates that are used for namespace provisioning
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Tier",type="string",JSONPath=`.spec.tierName`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Namespace Template Set"
type NSTemplateSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NSTemplateSetSpec   `json:"spec,omitempty"`
	Status NSTemplateSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NSTemplateSetList contains a list of NSTemplateSet
type NSTemplateSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NSTemplateSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NSTemplateSet{}, &NSTemplateSetList{})
}
