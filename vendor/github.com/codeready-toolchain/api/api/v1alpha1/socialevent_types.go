package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// SocialEventReady means the event has been setup successfully and passes validation requirements
	SocialEventReady ConditionType = "Ready"

	// Status condition reasons
	SocialEventInvalidUserTierReason      = "InvalidUserTier"
	SocialEventUnableToGetUserTierReason  = "UnableToGetUserTier"
	SocialEventInvalidSpaceTierReason     = "InvalidSpaceTier"
	SocialEventUnableToGetSpaceTierReason = "UnableToGetSpaceTier"

	// SocialEventUserSignupLabelKey the key of the label set on the UserSignups who registered with an activation code.
	// The label value is the name of the SocialEvent resource
	SocialEventUserSignupLabelKey = LabelKeyPrefix + "social-event"
)

// SocialEventSpec defines the parameters for a Social event, such as a training session or workshop. Users
// may register for the event by using the event's unique activation code
//
// +k8s:openapi-gen=true
type SocialEventSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// The timestamp from which users may register via this event's activation code
	StartTime metav1.Time `json:"startTime"`

	// The timestamp after which users may no longer register via this event's activation code
	EndTime metav1.Time `json:"endTime"`

	// An optional description that may be provided describing the purpose of the event
	// +optional
	Description string `json:"description,omitempty"`

	// The maximum number of attendees
	MaxAttendees int `json:"maxAttendees"`

	// The tier to assign to users registering for the event.
	// This must be the valid name of an nstemplatetier resource.
	UserTier string `json:"userTier"`

	// The tier to assign to spaces created for users who registered for the event.
	// This must be the valid name of an nstemplatetier resource.
	SpaceTier string `json:"spaceTier"`

	// If true, best effort is made to provision all attendees of the event on the same cluster
	// +optional
	PreferSameCluster bool `json:"preferSameCluster,omitempty"`

	// If true, the user will also be required to complete standard phone verification
	// +optional
	VerificationRequired bool `json:"verificationRequired,omitempty"`
}

// SocialEventStatus defines the observed state of SocialEvent
// +k8s:openapi-gen=true
type SocialEventStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Conditions is an array of current SocialEventStatus conditions
	// Supported condition types:
	// Ready
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	ActivationCount int `json:"activationCount"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SocialEvent registers a social event in Dev Sandbox
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="StartTime",type="string",JSONPath=`.spec.startTime`
// +kubebuilder:printcolumn:name="EndTime",type="string",JSONPath=`.spec.endTime`
// +kubebuilder:printcolumn:name="Description",type="string",JSONPath=`.spec.description`
// +kubebuilder:printcolumn:name="MaxAttendees",type="string",JSONPath=`.spec.maxAttendees`
// +kubebuilder:printcolumn:name="CurrentAttendees",type="string",JSONPath=`.status.activationCount`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Toolchain Event"
type SocialEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SocialEventSpec   `json:"spec,omitempty"`
	Status SocialEventStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SocialEventList contains a list of SocialEvent
type SocialEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SocialEvent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SocialEvent{}, &SocialEventList{})
}
