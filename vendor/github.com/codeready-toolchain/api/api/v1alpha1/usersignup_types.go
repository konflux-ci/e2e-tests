package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// These are valid conditions of a UserSignup

	// UserSignupApproved reflects whether the signup request has been approved or not
	UserSignupApproved ConditionType = "Approved"
	// UserSignupComplete means provisioning is complete
	UserSignupComplete ConditionType = "Complete"
	// UserSignupUserDeactivatingNotificationCreated is used to track the status of the notification send to a user
	// shortly before their account is due for deactivation
	UserSignupUserDeactivatingNotificationCreated ConditionType = "UserDeactivatingNotificationCreated"
	// UserSignupUserDeactivatedNotificationCreated means that the Notification CR was created so the user should be notified about their deactivated account
	UserSignupUserDeactivatedNotificationCreated ConditionType = "UserDeactivatedNotificationCreated"

	// UserSignupLastTargetClusterAnnotationKey is used for tracking the cluster for returning users
	UserSignupLastTargetClusterAnnotationKey = LabelKeyPrefix + "last-target-cluster"
	// UserSignupUserEmailAnnotationKey is used for the usersignup email annotations key
	UserSignupUserEmailAnnotationKey = LabelKeyPrefix + "user-email"
	// UserSignupVerificationCodeAnnotationKey is used for the usersignup verification code annotation key
	UserSignupVerificationCodeAnnotationKey = LabelKeyPrefix + "verification-code"
	// UserSignupVerificationTimestampAnnotationKey is used for the usersignup verification timestamp annotation key
	UserSignupVerificationTimestampAnnotationKey = LabelKeyPrefix + "verification-timestamp"
	// UserSignupVerificationInitTimestampAnnotationKey is used for the usersignup verification code generated timestamp annotation key
	UserSignupVerificationInitTimestampAnnotationKey = LabelKeyPrefix + "verification-init-timestamp"
	// UserSignupVerificationCounterAnnotationKey is used for the usersignup verification counter annotation key
	UserSignupVerificationCounterAnnotationKey = LabelKeyPrefix + "verification-counter"
	// UserVerificationAttemptsAnnotationKey is used for the usersignup verification attempts annotation key
	UserVerificationAttemptsAnnotationKey = LabelKeyPrefix + "verification-attempts"
	// UserVerificationExpiryAnnotationKey is used for the usersignup verification expiry annotation key
	UserVerificationExpiryAnnotationKey = LabelKeyPrefix + "verification-expiry"
	// SkipAutoCreateSpaceAnnotationKey when true signals the usersignup controller to skip Space creation, otherwise a Space will be created by default
	SkipAutoCreateSpaceAnnotationKey = LabelKeyPrefix + "skip-auto-create-space"
	// UserSignupActivationCounterAnnotationKey is used for the usersignup activation counter annotation key
	// Activations are counted after phone verification succeeded
	UserSignupActivationCounterAnnotationKey = LabelKeyPrefix + "activation-counter"
	// UserSignupCaptchaScoreAnnotationKey is set if captcha verification was used, and contains the last captcha assessment score for the user
	UserSignupCaptchaScoreAnnotationKey = LabelKeyPrefix + "captcha-score"

	// UserSignupUserEmailHashLabelKey is used for the usersignup email hash label key
	UserSignupUserEmailHashLabelKey = LabelKeyPrefix + "email-hash"
	// UserSignupUserPhoneHashLabelKey is used for the usersignup phone hash label key
	UserSignupUserPhoneHashLabelKey = LabelKeyPrefix + "phone-hash"

	// UserSignupSocialEventLabelKey is used to indicate that the user registered via an activation code, and contains
	// the name of the SocialEvent that they signed up for
	UserSignupSocialEventLabelKey = LabelKeyPrefix + "social-event"

	// UserSignupStateLabelKey is used for setting the required/expected state of UserSignups (not-ready, pending, approved, banned, deactivated).
	// The main purpose of the label is easy selecting the UserSignups based on the state - eg. get all UserSignup on the waiting list (state=pending).
	// Another usage of the label is counting the UserSingups for and exposing it through metrics or ToolchainStatus CR.
	// Every value is set before doing the action - approving/deactivating/banning. The only exception is the "not-ready" state which is used as an initial state
	// for all UserSignups that were just created and are still not fully ready - eg. requires verification.
	UserSignupStateLabelKey = StateLabelKey
	// UserSignupStateLabelValueNotReady is used for identifying that the UserSignup is not ready for approval yet (eg. requires verification)
	UserSignupStateLabelValueNotReady = "not-ready"
	// UserSignupStateLabelValuePending is used for identifying that the UserSignup is pending approval
	UserSignupStateLabelValuePending = StateLabelValuePending
	// UserSignupStateLabelValueApproved is used for identifying that the UserSignup is approved
	UserSignupStateLabelValueApproved = "approved"
	// UserSignupStateLabelValueDeactivated is used for identifying that the UserSignup is deactivated
	UserSignupStateLabelValueDeactivated = "deactivated"
	// UserSignupStateLabelValueBanned is used for identifying that the UserSignup is banned
	UserSignupStateLabelValueBanned = "banned"

	// Status condition reasons
	UnableToCreateSpaceBinding                     = "UnableToCreateSpaceBinding"
	UserSignupNoClusterAvailableReason             = "NoClusterAvailable"
	UserSignupNoUserTierAvailableReason            = "NoUserTierAvailable"
	UserSignupNoTemplateTierAvailableReason        = "NoTemplateTierAvailable"
	UserSignupFailedToReadUserApprovalPolicyReason = "FailedToReadUserApprovalPolicy"
	UserSignupUnableToCreateMURReason              = "UnableToCreateMUR"
	UserSignupUnableToUpdateAnnotationReason       = "UnableToUpdateAnnotation"
	UserSignupUnableToUpdateStateLabelReason       = "UnableToUpdateStateLabel"
	UserSignupUnableToDeleteMURReason              = "UnableToDeleteMUR"
	UserSignupUnableToCreateSpaceReason            = "UnableToCreateSpace"
	UserSignupUnableToCreateSpaceBindingReason     = UnableToCreateSpaceBinding
	UserSignupProvisioningSpaceReason              = "ProvisioningSpace"

	// The UserSignupUserDeactivatingReason constant will be replaced with UserSignupDeactivationInProgressReason
	// in order to reduce ambiguity.  The "Deactivating" state should only refer to the period of time before the
	// user is deactivated (by default 3 days), not when the user is in the actual process of deactivation
	UserSignupUserDeactivatingReason       = "Deactivating"
	UserSignupDeactivationInProgressReason = "DeactivationInProgress"

	UserSignupUserDeactivatedReason            = "Deactivated"
	UserSignupInvalidMURStateReason            = "InvalidMURState"
	UserSignupApprovedAutomaticallyReason      = "ApprovedAutomatically"
	UserSignupApprovedByAdminReason            = "ApprovedByAdmin"
	UserSignupPendingApprovalReason            = "PendingApproval"
	UserSignupUserBanningReason                = "Banning"
	UserSignupUserBannedReason                 = "Banned"
	UserSignupFailedToReadBannedUsersReason    = "FailedToReadBannedUsers"
	UserSignupMissingUserEmailReason           = "MissingUserEmail"
	UserSignupMissingUserEmailAnnotationReason = "MissingUserEmailAnnotation"
	UserSignupMissingEmailHashLabelReason      = "MissingEmailHashLabel"
	UserSignupInvalidEmailHashLabelReason      = "InvalidEmailHashLabel"
	UserSignupVerificationRequiredReason       = "VerificationRequired"

	notificationCRCreated        = "NotificationCRCreated"
	userIsActive                 = "UserIsActive"
	userNotInPreDeactivation     = "UserNotInPreDeactivation"
	notificationCRCreationFailed = "NotificationCRCreationFailed"

	// ###############################################################################
	//    Deactivation Notification Status Reasons
	// ###############################################################################

	// UserSignupDeactivatedNotificationUserIsActiveReason is the value that the condition reason is set to when
	// a previously deactivated user has been reactivated again (for example when a user signs up again after their
	// sandbox has been deactivated)
	UserSignupDeactivatedNotificationUserIsActiveReason = userIsActive

	UserSignupDeactivatedNotificationCRCreatedReason = notificationCRCreated

	UserSignupDeactivatedNotificationCRCreationFailedReason = notificationCRCreationFailed

	// ###############################################################################
	//    Pre-Deactivation Notification Status Reasons
	// ###############################################################################

	// UserSignupDeactivatingNotificationUserNotInPreDeactivationReason is the value that the condition reason is set to
	// for an active user, before entering the pre-deactivation period
	UserSignupDeactivatingNotificationUserNotInPreDeactivationReason = userNotInPreDeactivation

	UserSignupDeactivatingNotificationCRCreatedReason = notificationCRCreated

	UserSignupDeactivatingNotificationCRCreationFailedReason = notificationCRCreationFailed

	// ###############################################################################
	//    UserSignup States
	// ###############################################################################

	// UserSignupStateApproved - If set then the user has been manually approved.  Otherwise, if not set then
	// the user is subject of auto-approval (if enabled)
	UserSignupStateApproved = UserSignupState("approved")

	// UserSignupStateVerificationRequired - If set then the user must complete the phone verification process
	UserSignupStateVerificationRequired = UserSignupState("verification-required")

	// UserSignupStateDeactivating - If this state is set, it indicates that the user has entered the "pre-deactivation"
	// phase and their account will be deactivated shortly.  Setting this state triggers the sending of a notification
	// to the user to warn them of their pending account deactivation.
	UserSignupStateDeactivating = UserSignupState("deactivating")

	// UserSignupStateDeactivated - If this state is set, it means the user has been deactivated and they may no
	// longer use their account
	UserSignupStateDeactivated = UserSignupState("deactivated")

	// UserSignupStateBanned - If this state is set by an admin then the user's account will be banned.
	UserSignupStateBanned = UserSignupState("banned")
)

type UserSignupState string

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UserSignupSpec defines the desired state of UserSignup
// +k8s:openapi-gen=true
type UserSignupSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// The cluster in which the user is provisioned in
	// If not set then the target cluster will be picked automatically
	// +optional
	TargetCluster string `json:"targetCluster,omitempty"`

	// The user's user ID, obtained from the identity provider from the 'sub' (subject) claim
	Userid string `json:"userid"`

	// The user's username, obtained from the identity provider.
	Username string `json:"username"`

	// The user's first name, obtained from the identity provider.
	// +optional
	GivenName string `json:"givenName,omitempty"`

	// The user's last name, obtained from the identity provider.
	// +optional
	FamilyName string `json:"familyName,omitempty"`

	// The user's company name, obtained from the identity provider.
	// +optional
	Company string `json:"company,omitempty"`

	// States contains a number of values that reflect the desired state of the UserSignup.
	// +optional
	// +listType=atomic
	States []UserSignupState `json:"states,omitempty"`

	// OriginalSub is an optional property temporarily introduced for the purpose of migrating the users to
	// a new IdP provider client, and contains the user's "original-sub" claim
	// +optional
	OriginalSub string `json:"originalSub,omitempty"`

	// IdentityClaims contains as-is claim values extracted from the user's access token
	// +optional
	IdentityClaims IdentityClaimsEmbedded `json:"identityClaims,omitempty"`
}

// IdentityClaimsEmbedded is used to define a set of SSO claim values that we are interested in storing
// +k8s:openapi-gen=true
type IdentityClaimsEmbedded struct {

	// PropagatedClaims
	PropagatedClaims `json:",inline"`

	// PreferredUsername contains the user's username
	PreferredUsername string `json:"preferredUsername"`

	// GivenName contains the value of the 'given_name' claim
	// +optional
	GivenName string `json:"givenName,omitempty"`

	// FamilyName contains the value of the 'family_name' claim
	// +optional
	FamilyName string `json:"familyName,omitempty"`

	// Company contains the value of the 'company' claim
	// +optional
	Company string `json:"company,omitempty"`
}

type PropagatedClaims struct {
	// Sub contains the value of the 'sub' claim
	Sub string `json:"sub"`

	// UserID contains the value of the 'user_id' claim
	// +optional
	UserID string `json:"userID,omitempty"`

	// AccountID contains the value of the 'account_id' claim
	// +optional
	AccountID string `json:"accountID,omitempty"`

	// OriginalSub is an optional property temporarily introduced for the purpose of migrating the users to
	// a new IdP provider client, and contains the user's "original-sub" claim
	// +optional
	OriginalSub string `json:"originalSub,omitempty"`

	// Email contains the user's email address
	Email string `json:"email"`
}

// UserSignupStatus defines the observed state of UserSignup
// +k8s:openapi-gen=true
type UserSignupStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Conditions is an array of current UserSignup conditions
	// Supported condition types:
	// PendingApproval, Provisioning, Complete
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// CompliantUsername is used to store the transformed, DNS-1123 compliant username
	// +optional
	CompliantUsername string `json:"compliantUsername,omitempty"`

	// HomeSpace is the name of the Space that is created for the user
	// immediately after their account is approved.
	// This is used by the proxy when no workspace context is provided.
	// +optional
	HomeSpace string `json:"homeSpace,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// UserSignup registers a user in the CodeReady Toolchain
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Username",type="string",JSONPath=`.spec.username`
// +kubebuilder:printcolumn:name="First Name",type="string",JSONPath=`.spec.givenName`,priority=1
// +kubebuilder:printcolumn:name="Last Name",type="string",JSONPath=`.spec.familyName`,priority=1
// +kubebuilder:printcolumn:name="Company",type="string",JSONPath=`.spec.company`,priority=1
// +kubebuilder:printcolumn:name="TargetCluster",type="string",JSONPath=`.spec.targetCluster`,priority=1
// +kubebuilder:printcolumn:name="Complete",type="string",JSONPath=`.status.conditions[?(@.type=="Complete")].status`
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=`.status.conditions[?(@.type=="Complete")].reason`
// +kubebuilder:printcolumn:name="Approved",type="string",JSONPath=`.status.conditions[?(@.type=="Approved")].status`,priority=1
// +kubebuilder:printcolumn:name="ApprovedBy",type="string",JSONPath=`.status.conditions[?(@.type=="Approved")].reason`,priority=1
// +kubebuilder:printcolumn:name="States",type="string",JSONPath=`.spec.states`,priority=1
// +kubebuilder:printcolumn:name="CompliantUsername",type="string",JSONPath=`.status.compliantUsername`
// +kubebuilder:printcolumn:name="Email",type="string",JSONPath=`.metadata.annotations.toolchain\.dev\.openshift\.com/user-email`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="User Signup"
type UserSignup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSignupSpec   `json:"spec,omitempty"`
	Status UserSignupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UserSignupList contains a list of UserSignup
type UserSignupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserSignup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserSignup{}, &UserSignupList{})
}
