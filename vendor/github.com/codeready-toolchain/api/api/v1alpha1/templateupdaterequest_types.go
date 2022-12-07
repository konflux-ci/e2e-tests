package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NSTemplateTierNameLabelKey stores the name of NSTemplateTier that this TemplateUpdateRequest is related to
const NSTemplateTierNameLabelKey = LabelKeyPrefix + "nstemplatetier"

// These are valid conditions of a TemplateUpdateRequest
const (
	// TemplateUpdateRequestComplete when the MasterUserRecord has been updated (via the TemplateUpdateRequest)
	// (for the Template Update Request, "complete" makes more sense than the usual "ready" condition type)
	TemplateUpdateRequestComplete ConditionType = "Complete"

	// Status condition reasons
	// TemplateUpdateRequestUpdatedReason when the MasterUserRecord was successfully updated
	TemplateUpdateRequestUpdatedReason = "Updated"
	// TemplateUpdateRequestUpdatingReason when the MasterUserRecord is still being updated
	TemplateUpdateRequestUpdatingReason = updatingReason
	// TemplateUpdateRequestUnableToUpdateReason when an error occurred while updating the MasterUserRecord
	TemplateUpdateRequestUnableToUpdateReason = "UnableToUpdate"
)

// TemplateUpdateRequestSpec defines the desired state of TemplateUpdateRequest
// It contains the new TemplateRefs to use in the MasterUserRecords
// +k8s:openapi-gen=true
type TemplateUpdateRequestSpec struct {
	// The name of the tier to be updated
	// +optional
	TierName string `json:"tierName,omitempty"`

	// The namespace templates
	// +optional
	// +listType=atomic
	Namespaces []NSTemplateTierNamespace `json:"namespaces,omitempty"`

	// the cluster resources template (for cluster-wide quotas, etc.)
	// +optional
	ClusterResources *NSTemplateTierClusterResources `json:"clusterResources,omitempty"`

	// Holds the value from “toolchain.dev.openshift.com/<tiername>-tier-hash” label of the associated Space CR at the time when TemplateUpdateRequest CR is created
	// +optional
	CurrentTierHash string `json:"currentTierHash,omitempty"`
}

// TemplateUpdateRequestStatus defines the observed state of TemplateUpdateRequest
// +k8s:openapi-gen=true
type TemplateUpdateRequestStatus struct {
	// Conditions is an array of current TemplateUpdateRequest conditions
	// Supported condition types: TemplateUpdateRequestComplete
	// +optional
	// +listType=atomic
	Conditions []Condition `json:"conditions,omitempty"`

	// SyncIndexes contains the `syncIndex` for each cluster in the MasterUserRecord.
	// The values here are "captured" before the MasterUserRecord is updated, so we can
	// track the update progress on the member clusters.
	// +optional
	// +patchStrategy=merge
	// +mapType=atomic
	SyncIndexes map[string]string `json:"syncIndexes,omitempty" patchStrategy:"merge"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TemplateUpdateRequest is the Schema for the templateupdaterequests API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Tier",type="string",JSONPath=`.spec.tierName`
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=`.status.conditions[?(@.type=="Complete")].status`
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=`.status.conditions[?(@.type=="Complete")].reason`
// +kubebuilder:resource:path=templateupdaterequests,scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Template Update Request"
type TemplateUpdateRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TemplateUpdateRequestSpec `json:"spec,omitempty"`

	Status TemplateUpdateRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TemplateUpdateRequestList contains a list of TemplateUpdateRequest
type TemplateUpdateRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateUpdateRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TemplateUpdateRequest{}, &TemplateUpdateRequestList{})
}
