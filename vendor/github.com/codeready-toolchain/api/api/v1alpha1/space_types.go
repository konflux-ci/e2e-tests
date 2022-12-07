package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// SpaceCreatorLabelKey is used to label the Space with the ID of its creator (Dev Sandbox UserSignup or AppStudio Workspace)
	SpaceCreatorLabelKey = LabelKeyPrefix + "creator"

	// WorkspaceLabelKey is used to label the Space with the name of the associated AppStudio Workspace
	WorkspaceLabelKey = LabelKeyPrefix + "workspace"
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

	// TierName is a required property introduced to retain the name of the tier
	// for which this Space is provisioned
	// If not set then the tier name will be set automatically
	// +optional
	TierName string `json:"tierName,omitempty"`
}

// SpaceStatus defines the observed state of Space
// +k8s:openapi-gen=true
type SpaceStatus struct {

	// TargetCluster The cluster in which this Space is currently provisioned
	// Can be empty if provisioning did not start or failed
	// To be used to de-provision the NSTemplateSet if the Spec.TargetCluster is either changed or removed
	// +optional
	TargetCluster string `json:"targetCluster,omitempty"`

	// Conditions is an array of current Space conditions
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
