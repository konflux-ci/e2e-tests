package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true

// UserTier contains user-specific configuration
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="User Tier"
type UserTier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec UserTierSpec `json:"spec,omitempty"`
}

// UserTierSpec defines the desired state of UserTier
// +k8s:openapi-gen=true
type UserTierSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// the period (in days) after which users within the tier will be deactivated
	// +optional
	DeactivationTimeoutDays int `json:"deactivationTimeoutDays,omitempty"`
}

//+kubebuilder:object:root=true

// UserTierList contains a list of UserTier
type UserTierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserTier `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserTier{}, &UserTierList{})
}
