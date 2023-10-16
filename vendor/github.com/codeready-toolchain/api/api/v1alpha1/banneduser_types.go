package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// BannedUserEmailHashLabelKey is used for the banneduser email hash label key
	BannedUserEmailHashLabelKey = LabelKeyPrefix + "email-hash"

	// BannedUserPhoneNumberHashLabelKey is used a label key for the hash of a phone of the banned user
	BannedUserPhoneNumberHashLabelKey = LabelKeyPrefix + "phone-hash"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BannedUserSpec defines the desired state of BannedUser
// +k8s:openapi-gen=true
type BannedUserSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// The e-mail address of the account that has been banned
	Email string `json:"email"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BannedUser is used to maintain a list of banned e-mail addresses
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Email",type="string",JSONPath=`.spec.email`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Banned User"
type BannedUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BannedUserSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// BannedUserList contains a list of BannedUser
type BannedUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BannedUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BannedUser{}, &BannedUserList{})
}
