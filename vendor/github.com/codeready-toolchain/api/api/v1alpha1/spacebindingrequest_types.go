package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// SpaceBindingRequestLabelKey is a label on the SpaceBinding, and will hold the name of the SpaceBindingRequest that created the SpaceBinding resource.
	SpaceBindingRequestLabelKey = LabelKeyPrefix + "spacebindingrequest"
	// SpaceBindingRequestNamespaceLabelKey is a label on the SpaceBinding, and will hold the namespace of the SpaceBindingRequest that created the SpaceBinding resource.
	SpaceBindingRequestNamespaceLabelKey = LabelKeyPrefix + "spacebindingrequest-namespace"

	// --- Status condition reasons ---

	// SpaceBindingRequestTerminatingReason represents the reason for space binding request termination.
	SpaceBindingRequestTerminatingReason = terminatingReason

	// SpaceBindingRequestTerminatingFailedReason represents the reason for a failed space binding request termination.
	SpaceBindingRequestTerminatingFailedReason = terminatingFailedReason

	// SpaceBindingRequestUnableToCreateSpaceBindingReason represents the reason for a failed space binding creation.
	SpaceBindingRequestUnableToCreateSpaceBindingReason = UnableToCreateSpaceBinding

	// SpaceBindingRequestProvisioningReason represents the reason for space binding request provisioning.
	SpaceBindingRequestProvisioningReason = provisioningReason

	// SpaceBindingRequestProvisionedReason represents the reason for a successfully provisioned space binding request.
	SpaceBindingRequestProvisionedReason = provisionedReason
)

// SpaceBindingRequestSpec defines the desired state of SpaceBindingRequest
// +k8s:openapi-gen=true
type SpaceBindingRequestSpec struct {
	// MasterUserRecord is a required property introduced to retain the name of the MUR
	// for which this SpaceBinding is provisioned.
	MasterUserRecord string `json:"masterUserRecord"`

	// SpaceRole is a required property which defines the role that will be granted to the MUR in the current Space by the SpaceBinding resource.
	SpaceRole string `json:"spaceRole"`
}

// SpaceBindingRequestStatus defines the observed state of SpaceBinding
// +k8s:openapi-gen=true
type SpaceBindingRequestStatus struct {
	// Conditions is an array of SpaceBindingRequest conditions
	// Supported condition types:
	// Provisioning, SpaceBindingNotReady and Ready
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SpaceBindingRequest is the Schema for the SpaceBindingRequest API
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="MUR",type="string",JSONPath=`.spec.masterUserRecord`
// +kubebuilder:printcolumn:name="SpaceRole",type="string",JSONPath=`.spec.spaceRole`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="SpaceBindingRequest"
type SpaceBindingRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpaceBindingRequestSpec   `json:"spec,omitempty"`
	Status SpaceBindingRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SpaceBindingRequestList contains a list of SpaceBindingRequests
type SpaceBindingRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpaceBindingRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpaceBindingRequest{}, &SpaceBindingRequestList{})
}
