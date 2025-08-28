package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These are valid conditions of a MasterUserRecord
const (

	// #### CONDITION TYPES ####

	// MasterUserRecordProvisioning means the Master User Record is being provisioned
	MasterUserRecordProvisioning ConditionType = "Provisioning"
	// MasterUserRecordUserAccountNotReady means the User Account failed to be provisioned
	MasterUserRecordUserAccountNotReady ConditionType = "UserAccountNotReady"
	// MasterUserRecordReady means the Master User Record provisioning succeeded
	MasterUserRecordReady ConditionType = "Ready"
	// MasterUserRecordUserProvisionedNotificationCreated means that the Notification CR was created so the user should be notified about the successful provisioning
	MasterUserRecordUserProvisionedNotificationCreated ConditionType = "UserProvisionedNotificationCreated"

	// #### CONDITION REASONS ####

	// Status condition reasons
	MasterUserRecordUnableToGetUserAccountReason             = "UnableToGetUserAccount"
	MasterUserRecordUnableToCreateUserAccountReason          = "UnableToCreateUserAccount"
	MasterUserRecordUnableToSynchronizeUserAccountSpecReason = "UnableToSynchronizeUserAccountSpecAccount"
	MasterUserRecordTargetClusterNotReadyReason              = "TargetClusterNotReady"
	MasterUserRecordProvisioningReason                       = provisioningReason
	MasterUserRecordProvisionedReason                        = provisionedReason
	MasterUserRecordUpdatingReason                           = updatingReason
	MasterUserRecordUnableToAddFinalizerReason               = "UnableToAddFinalizer"
	MasterUserRecordUnableToDeleteUserAccountsReason         = "UnableToDeleteUserAccounts"
	MasterUserRecordUnableToRemoveFinalizerReason            = "UnableToRemoveFinalizer"
	MasterUserRecordUnableToCheckLabelsReason                = "UnableToCheckLabels"
	MasterUserRecordDisabledReason                           = disabledReason
	MasterUserRecordNotificationCRCreatedReason              = "NotificationCRCreated"
	MasterUserRecordNotificationCRCreationFailedReason       = "NotificationCRCreationFailed"

	// #### LABELS ####

	// MasterUserRecordOwnerLabelKey indicates the label value that contains the owner reference for this resource,
	// which will be the UserSignup instance with the corresponding resource name
	MasterUserRecordOwnerLabelKey = OwnerLabelKey

	// #### ANNOTATIONS ####
	// MasterUserRecordEmailAnnotationKey is used to store the user's email in an annotation
	// (Note: key is the same as for the UserSignup annotation)
	MasterUserRecordEmailAnnotationKey = UserSignupUserEmailAnnotationKey
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MasterUserRecordSpec defines the desired state of MasterUserRecord
// +k8s:openapi-gen=true
type MasterUserRecordSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// UserID is the user ID from RHD Identity Provider token (“sub” claim)
	UserID string `json:"userID"`

	// If set to true then the corresponding user should not be able to login (but the underlying UserAccounts still exists)
	// "false" is assumed by default
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// The list of user accounts in the member clusters which belong to this MasterUserRecord
	// +listType=map
	// +listMapKey=targetCluster
	UserAccounts []UserAccountEmbedded `json:"userAccounts,omitempty"`

	// OriginalSub is an optional property temporarily introduced for the purpose of migrating the users to
	// a new IdP provider client, and contains the user's "original-sub" claim
	// +optional
	OriginalSub string `json:"originalSub,omitempty"`

	// TierName is an optional property introduced to retain the name of the tier
	// for which the Dev Sandbox user is provisioned, so we can still deal with deactivation
	// once the NSTemplateSet field has been removed from `[]spec.UserAccounts`
	// temporarily marked as optional until the migration took place (CRT-1321)
	// +optional
	TierName string `json:"tierName,omitempty"`

	// PropagatedClaims contains a selection of claim values from the SSO Identity Provider which are intended to
	// be "propagated" down the resource dependency chain
	// +optional
	PropagatedClaims PropagatedClaims `json:"propagatedClaims,omitempty"`
}

type UserAccountEmbedded struct {

	// The cluster in which the user exists
	TargetCluster string `json:"targetCluster"`
}

// MasterUserRecordStatus defines the observed state of MasterUserRecord
// +k8s:openapi-gen=true
type MasterUserRecordStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Conditions is an array of current Master User Record conditions
	// Supported condition types:
	// Provisioning, UserAccountNotReady and Ready
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// The status of user accounts in the member clusters which belong to this MasterUserRecord
	// +listType=atomic
	UserAccounts []UserAccountStatusEmbedded `json:"userAccounts,omitempty"`

	// The timestamp when the user was provisioned
	// +optional
	ProvisionedTime *metav1.Time `json:"provisionedTime,omitempty"`
}

type UserAccountStatusEmbedded struct {

	// Cluster is the cluster in which the user exists
	Cluster Cluster `json:"cluster"`

	// Inherits the status from the corresponding UserAccount status
	UserAccountStatus `json:",inline"`
}

type Cluster struct {
	// Name is the name of the corresponding ToolchainCluster resource
	Name string `json:"name"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MasterUserRecord keeps all information about user, user accounts and namespaces provisioned in CodeReady Toolchain
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:resource:shortName=mur
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=`.spec.userAccounts[].targetCluster`
// +kubebuilder:printcolumn:name="Tier",type="string",JSONPath=`.spec.tierName`
// +kubebuilder:printcolumn:name="Banned",type="string",JSONPath=`.spec.banned`,priority=1
// +kubebuilder:printcolumn:name="Disabled",type="string",JSONPath=`.spec.disabled`,priority=1
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Master User Record"
type MasterUserRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MasterUserRecordSpec   `json:"spec,omitempty"`
	Status MasterUserRecordStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MasterUserRecordList contains a list of MasterUserRecord
type MasterUserRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MasterUserRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MasterUserRecord{}, &MasterUserRecordList{})
}
