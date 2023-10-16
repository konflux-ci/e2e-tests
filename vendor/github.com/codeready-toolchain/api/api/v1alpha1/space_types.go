package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// SpaceCreatorLabelKey is used to label the Space with the ID of its creator (Dev Sandbox UserSignup or AppStudio Workspace)
	SpaceCreatorLabelKey = LabelKeyPrefix + "creator"

	// WorkspaceLabelKey is used to label the Space with the name of the associated AppStudio Workspace
	WorkspaceLabelKey = LabelKeyPrefix + "workspace"

	// ParentSpaceLabelKey is used to label the Space with the name of the parent space
	// from which the creation was requested
	ParentSpaceLabelKey = LabelKeyPrefix + "parent-space"
)

// These are valid status condition reasons of a Space
const (
	// Status condition reasons
	SpaceUnableToCreateNSTemplateSetReason = "UnableToCreateNSTemplateSet"
	SpaceUnableToUpdateNSTemplateSetReason = "UnableToUpdateNSTemplateSet"
	SpaceProvisioningReason                = provisioningReason
	SpaceProvisioningPendingReason         = "ProvisioningPending"
	SpaceProvisioningFailedReason          = "UnableToProvision"
	SpaceProvisionedReason                 = provisionedReason
	SpaceTerminatingReason                 = terminatingReason
	SpaceTerminatingFailedReason           = terminatingFailedReason
	SpaceUpdatingReason                    = updatingReason
	SpaceRetargetingReason                 = "Retargeting"
	SpaceRetargetingFailedReason           = "UnableToRetarget"

	// SpaceStateLabelKey is used for setting the actual/expected state of Spaces (pending, or empty).
	// The main purpose of the label is easy selecting the Spaces based on the state - eg. get all Spaces on the waiting list (state=pending).
	SpaceStateLabelKey = StateLabelKey
	// SpaceStateLabelValuePending is used for identifying that the Space is waiting for assigning an available cluster
	SpaceStateLabelValuePending = StateLabelValuePending
	// SpaceStateLabelValueClusterAssigned is used for identifying that the Space has an assigned cluster
	SpaceStateLabelValueClusterAssigned = "cluster-assigned"
)

// SpaceSpec defines the desired state of Space
// +k8s:openapi-gen=true
type SpaceSpec struct {

	// TargetCluster The cluster in which this Space is going to be provisioned
	// If not set then the target cluster will be picked automatically
	// +optional
	TargetCluster string `json:"targetCluster,omitempty"`

	// TargetClusterRoles one or more label keys that define a set of clusters
	// where the Space can be provisioned.
	// The target cluster has to match ALL the roles defined in this field in order for the space to be provisioned there.
	// It can be used as an alternative to targetCluster field, which has precedence in case both roles and name are provided.
	// +optional
	// +listType=atomic
	TargetClusterRoles []string `json:"targetClusterRoles,omitempty"`

	// TierName is introduced to retain the name of the tier
	// for which this Space is provisioned
	// If not set then the tier name will be set automatically
	// +optional
	TierName string `json:"tierName,omitempty"`

	// ParentSpace holds the name of the context (Space) from which this space was created (requested),
	// enabling hierarchy relationships between different Spaces.
	//
	// Keeping this association brings two main benefits:
	// 1. SpaceBindings are inherited from the parent Space
	// 2. Ability to easily monitor quota for the requested sub-spaces
	// +optional
	ParentSpace string `json:"parentSpace,omitempty"`
}

// SpaceStatus defines the observed state of Space
// +k8s:openapi-gen=true
type SpaceStatus struct {

	// TargetCluster The cluster in which this Space is currently provisioned
	// Can be empty if provisioning did not start or failed
	// To be used to de-provision the NSTemplateSet if the Spec.TargetCluster is either changed or removed
	// +optional
	TargetCluster string `json:"targetCluster,omitempty"`

	// ProvisionedNamespaces is a list of Namespaces that were provisioned for the Space.
	// +optional
	// +listType=atomic
	ProvisionedNamespaces []SpaceNamespace `json:"provisionedNamespaces,omitempty"`

	// Conditions is an array of current Space conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// Space is the Schema for the spaces API
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=`.spec.targetCluster`
// +kubebuilder:printcolumn:name="Tier",type="string",JSONPath=`.spec.tierName`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Space"
type Space struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpaceSpec   `json:"spec,omitempty"`
	Status SpaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SpaceList contains a list of Space
type SpaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Space `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Space{}, &SpaceList{})
}
