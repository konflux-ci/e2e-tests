package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These are valid conditions of a ToolchainConfig
const (
	ToolchainConfigSyncComplete     ConditionType = "SyncComplete"
	ToolchainConfigRegServiceDeploy ConditionType = "RegServiceDeploy"

	// Status condition reasons
	// ToolchainConfigSyncedReason when the MemberOperatorConfigs were successfully synced to the member clusters
	ToolchainConfigSyncedReason = "Synced"
	// ToolchainConfigSyncFailedReason when there were failures while syncing MemberOperatorConfigs to the member clusters
	ToolchainConfigSyncFailedReason = "SyncFailed"
	// ToolchainConfigRegServiceDeployingReason when the registration service is being deployed
	ToolchainConfigRegServiceDeployingReason = "Deploying"
	// ToolchainConfigRegServiceDeployedReason when the registration service has deployed successfully
	ToolchainConfigRegServiceDeployedReason = "Deployed"
	// ToolchainConfigRegServiceDeployFailedReason when there were failures while deploying the registration service
	ToolchainConfigRegServiceDeployFailedReason = "DeployFailed"
)

// ToolchainConfigSpec contains all configuration for host and member operators
// +k8s:openapi-gen=true
type ToolchainConfigSpec struct {
	// Contains all host operator configuration
	// +optional
	Host HostConfig `json:"host,omitempty"`

	// Contains all member operator configurations for all member clusters
	// +optional
	Members Members `json:"members,omitempty"`
}

// HostConfig contains all configuration parameters of the host operator
// +k8s:openapi-gen=true
type HostConfig struct {

	// Environment specifies the host-operator environment such as prod, stage, unit-tests, e2e-tests, dev, etc
	// +optional
	Environment *string `json:"environment,omitempty"`

	// Keeps parameters necessary for automatic approval
	// +optional
	AutomaticApproval AutomaticApprovalConfig `json:"automaticApproval,omitempty"`

	// Keeps parameters concerned with user deactivation
	// +optional
	Deactivation DeactivationConfig `json:"deactivation,omitempty"`

	// Keeps parameters concerned with metrics
	// +optional
	Metrics MetricsConfig `json:"metrics,omitempty"`

	// Keeps parameters concerned with notifications
	// +optional
	Notifications NotificationsConfig `json:"notifications,omitempty"`

	// Keeps parameters necessary for the registration service
	// +optional
	RegistrationService RegistrationServiceConfig `json:"registrationService,omitempty"`

	// Keeps parameters concerned with tiers
	// +optional
	Tiers TiersConfig `json:"tiers,omitempty"`

	// Keeps parameters concerned with the toolchainstatus
	// +optional
	ToolchainStatus ToolchainStatusConfig `json:"toolchainStatus,omitempty"`

	// Keeps parameters concerned with user management
	// +optional
	Users UsersConfig `json:"users,omitempty"`

	// Keeps parameters necessary for configuring capacity limits
	// +optional
	CapacityThresholds CapacityThresholds `json:"capacityThresholds,omitempty"`

	// Keeps parameters necessary for configuring Space provisioning functionality
	// +optional
	SpaceConfig SpaceConfig `json:"spaceConfig,omitempty"`
}

// Members contains all configuration for member operators
// +k8s:openapi-gen=true
type Members struct {
	// Defines default configuration to be applied to all member clusters
	// +optional
	Default MemberOperatorConfigSpec `json:"default,omitempty"`

	// A map of cluster-specific member operator configurations indexed by member toolchaincluster name
	// +optional
	// +mapType=atomic
	SpecificPerMemberCluster map[string]MemberOperatorConfigSpec `json:"specificPerMemberCluster,omitempty"`
}

// Defines all parameters necessary for automatic approval
// +k8s:openapi-gen=true
type AutomaticApprovalConfig struct {
	// Defines if the automatic approval is enabled or not
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// Contains default capacity threshold as well as specific ones for particular member clusters
// +k8s:openapi-gen=true
type ResourceCapacityThreshold struct {
	// It is the default capacity threshold (in percentage of usage) to be used for all member clusters if no special threshold is defined
	// +optional
	DefaultThreshold *int `json:"defaultThreshold,omitempty"`

	// Contains a map of specific capacity thresholds (in percentage of usage) for particular member clusters mapped by their names
	// +optional
	// +mapType=atomic
	SpecificPerMemberCluster map[string]int `json:"specificPerMemberCluster,omitempty"`
}

// CapacityThresholds allows to configure the capacity limits in the clusters
// +k8s:openapi-gen=true
type CapacityThresholds struct {

	// Contains capacity threshold configuration
	// +optional
	ResourceCapacityThreshold ResourceCapacityThreshold `json:"resourceCapacityThreshold,omitempty"`

	// Contains a map of maximal number of spaces that can be provisioned per member cluster mapped by the cluster name
	// +optional
	// +mapType=atomic
	MaxNumberOfSpacesPerMemberCluster map[string]int `json:"maxNumberOfSpacesPerMemberCluster,omitempty"`
}

// DeactivationConfig contains all configuration parameters related to deactivation
// +k8s:openapi-gen=true
type DeactivationConfig struct {

	// DeactivatingNotificationDays is the number of days after a pre-deactivating notification is sent that actual
	// deactivation occurs.  If this parameter is set to zero, then there will be no delay
	// +optional
	DeactivatingNotificationDays *int `json:"deactivatingNotificationDays,omitempty"`

	// DeactivationDomainsExcluded is a string of comma-separated domains that should be excluded from automatic user deactivation
	// For example: "@redhat.com,@ibm.com"
	// +optional
	DeactivationDomainsExcluded *string `json:"deactivationDomainsExcluded,omitempty"`

	// UserSignupDeactivatedRetentionDays is used to configure how many days we should keep deactivated UserSignup
	// resources before deleting them.  This parameter value should reflect an extended period of time sufficient for
	// gathering user metrics before removing the resources from the cluster.
	// +optional
	UserSignupDeactivatedRetentionDays *int `json:"userSignupDeactivatedRetentionDays,omitempty"`

	// UserSignupUnverifiedRetentionDays is used to configure how many days we should keep unverified (i.e. the user
	// hasn't completed the user verification process via the registration service) UserSignup resources before deleting
	// them.  It is intended for this parameter to define an aggressive cleanup schedule for unverified user signups,
	// and the default configuration value for this parameter reflects this.
	// +optional
	UserSignupUnverifiedRetentionDays *int `json:"userSignupUnverifiedRetentionDays,omitempty"`
}

// ToolchainSecret defines a reference to a secret, this type should be included inline in any structs that contain secrets eg. NotificationSecret
// +k8s:openapi-gen=true
type ToolchainSecret struct {

	// Reference is the name of the secret resource to look up
	// +optional
	Ref *string `json:"ref,omitempty"`
}

// MetricsConfig contains all configuration parameters related to metrics gathering
// +k8s:openapi-gen=true
type MetricsConfig struct {

	// ForceSynchronization is a flag used to trigger synchronization of the metrics
	// based on the resources rather than on the content of `ToolchainStatus.status.metrics`
	// +optional
	ForceSynchronization *bool `json:"forceSynchronization,omitempty"`
}

// NotificationsConfig contains all configuration parameters related to notifications
// +k8s:openapi-gen=true
type NotificationsConfig struct {

	// NotificationDeliveryService is notification delivery service to use for notifications
	// +optional
	NotificationDeliveryService *string `json:"notificationDeliveryService,omitempty"`

	// DurationBeforeNotificationDeletion is notification delivery service to use for notifications
	// +optional
	DurationBeforeNotificationDeletion *string `json:"durationBeforeNotificationDeletion,omitempty"`

	// The administrator email address for system notifications
	// +optional
	AdminEmail *string `json:"adminEmail,omitempty"`

	// TemplateSetName defines the set of notification templates. Different Sandbox instances can use different notification templates. For example Dev Sandbox and AppStudio instances use different templates. By default, the "sandbox" template set name is used.
	// +optional
	TemplateSetName *string `json:"templateSetName,omitempty"`

	// Defines all secrets related to notification configuration
	// +optional
	Secret NotificationSecret `json:"secret,omitempty"`
}

// Defines all secrets related to notification configuration
// +k8s:openapi-gen=true
type NotificationSecret struct {
	// The reference to the secret that is expected to contain the keys below
	// +optional
	ToolchainSecret `json:",inline"`

	// The key for the host operator mailgun domain used for creating an instance of mailgun
	// +optional
	MailgunDomain *string `json:"mailgunDomain,omitempty"`

	// The key for the host operator mailgun api key used for creating an instance of mailgun
	// +optional
	MailgunAPIKey *string `json:"mailgunAPIKey,omitempty"`

	// The key for the host operator mailgun senders email
	// +optional
	MailgunSenderEmail *string `json:"mailgunSenderEmail,omitempty"`

	// The key for the reply-to email address that will be set in sent notifications
	// +optional
	MailgunReplyToEmail *string `json:"mailgunReplyToEmail,omitempty"`
}

// RegistrationServiceConfig contains all configuration parameters related to the registration service
// +k8s:openapi-gen=true
type RegistrationServiceConfig struct {

	// Keeps parameters necessary for the registration service analytics config
	// +optional
	Analytics RegistrationServiceAnalyticsConfig `json:"analytics,omitempty"`

	// Keeps parameters necessary for the registration service authentication config
	// +optional
	Auth RegistrationServiceAuthConfig `json:"auth,omitempty"`

	// Environment specifies the environment such as prod, stage, unit-tests, e2e-tests, dev, etc
	// +optional
	Environment *string `json:"environment,omitempty"`

	// LogLevel specifies the logging level
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`

	// Namespace specifies the namespace in which the registration service and host operator is running
	// Consumed by host operator and set as env var on registration-service deployment
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// RegistrationServiceURL is the URL used to a ccess the registration service
	// +optional
	RegistrationServiceURL *string `json:"registrationServiceURL,omitempty"`

	// Replicas specifies the number of replicas to use for the registration service deployment
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Keeps parameters necessary for the registration service verification config
	// +optional
	Verification RegistrationServiceVerificationConfig `json:"verification,omitempty"`
}

// RegistrationServiceAnalyticsConfig contains the subset of registration service configuration parameters related to analytics
// +k8s:openapi-gen=true
type RegistrationServiceAnalyticsConfig struct {
	// DevSpaces contains the analytics configuration parameters for devspaces
	// +optional
	DevSpaces DevSpaces `json:"devSpaces,omitempty"`

	// SegmentWriteKey specifies the segment write key for sandbox
	// +optional
	SegmentWriteKey *string `json:"segmentWriteKey,omitempty"`
}

type DevSpaces struct {
	// SegmentWriteKey specifies the segment write key
	// +optional
	SegmentWriteKey *string `json:"segmentWriteKey,omitempty"`
}

// RegistrationServiceAuthConfig contains the subset of registration service configuration parameters related to authentication
// +k8s:openapi-gen=true
type RegistrationServiceAuthConfig struct {

	// AuthClientLibraryURL specifies the auth library location
	// +optional
	AuthClientLibraryURL *string `json:"authClientLibraryURL,omitempty"`

	// AuthClientConfigContentType specifies the auth config config content type
	// +optional
	AuthClientConfigContentType *string `json:"authClientConfigContentType,omitempty"`

	// AuthClientConfigRaw specifies the URL used to a access the registration service
	// +optional
	AuthClientConfigRaw *string `json:"authClientConfigRaw,omitempty"`

	// AuthClientPublicKeysURL specifies the public keys URL
	// +optional
	AuthClientPublicKeysURL *string `json:"authClientPublicKeysURL,omitempty"`
}

// RegistrationServiceVerificationConfig contains the subset of registration service configuration parameters related to verification
// +k8s:openapi-gen=true
type RegistrationServiceVerificationConfig struct {

	// Defines all secrets related to the registration service verification configuration
	// +optional
	Secret RegistrationServiceVerificationSecret `json:"secret,omitempty"`

	// VerificationEnabled specifies whether verification is enabled or not
	// Verification enablement works in the following way:
	//   1. verification.enabled == false
	//      No verification during the signup process at all. (no phone, no captcha)
	//   2. verification.enabled == true && verification.captcha.enabled == true
	//      Captcha is enabled and will bypass phone verification if the score is above the threshold but if the score is
	//      below the threshold then phone verification kicks in.
	//   3. verification.enabled == true && verification.captcha.enabled == false
	//      Only phone verification is effect.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Captcha defines any configuration related to captcha verification
	// +optional
	Captcha CaptchaConfig `json:"captcha,omitempty"`

	// VerificationDailyLimit specifies the number of times a user may initiate a phone verification request within a
	// 24 hour period
	// +optional
	DailyLimit *int `json:"dailyLimit,omitempty"`

	// VerificationAttemptsAllowed specifies the number of times a user may attempt to correctly enter a verification code,
	// if they fail then they must request another code
	// +optional
	AttemptsAllowed *int `json:"attemptsAllowed,omitempty"`

	// VerificationMessageTemplate specifies the message template used to generate the content sent to users via SMS for
	// phone verification
	// +optional
	MessageTemplate *string `json:"messageTemplate,omitempty"`

	// VerificationExcludedEmailDomains specifies the list of email address domains for which phone verification
	// is not required
	// +optional
	ExcludedEmailDomains *string `json:"excludedEmailDomains,omitempty"`

	// VerificationCodeExpiresInMin specifies an int representing the number of minutes before a verification code should
	// be expired
	// +optional
	CodeExpiresInMin *int `json:"codeExpiresInMin,omitempty"`

	// NotificationSender is used to specify which service should be used to send verification notifications. Allowed
	// values are "twilio", "aws".  If not specified, the Twilio sender will be used.
	// +optional
	NotificationSender *string `json:"notificationSender,omitempty"`

	// AWSRegion to use when sending notification SMS
	// +optional
	AWSRegion *string `json:"awsRegion,omitempty"`

	// AWSSenderID the Alphanumeric Sender ID to use, e.g. "DevSandbox"
	// +optional
	AWSSenderID *string `json:"awsSenderID,omitempty"`

	// AWSSMSType is the type of SMS message to send, either `Promotional` or `Transactional`
	// See https://docs.aws.amazon.com/sns/latest/dg/sms_publish-to-phone.html for details
	// +optional
	AWSSMSType *string `json:"awsSMSType,omitempty"`

	// TwilioSenderConfigs is an array of TwilioSenderConfig objects
	// +optional
	// +listType=atomic
	TwilioSenderConfigs []TwilioSenderConfig `json:"twilioSenderConfigs,omitempty"`
}

// TwilioSenderConfig is used to associate a particular sender ID (a sender ID is a text value that appears instead of
// a phone number when receiving an SMS message), for example "RED HAT", with an array of country
// code values for which the Sender ID value will be set via the Twilio API when sending a verification code to a user in
// any of the country codes specified.
//
// Since some countries are starting to block long form phone numbers (i.e. SMS messages from international phone numbers)
// the Sender ID may be an acceptable alternative to requiring the verification message to be sent from a local phone number.
//
// +k8s:openapi-gen=true
type TwilioSenderConfig struct {
	// SenderID
	SenderID string `json:"senderID"`

	// CountryCodes
	// +optional
	// +listType=set
	CountryCodes []string `json:"countryCodes,omitempty"`
}

// Defines all secrets related to registration service verification configuration
// +k8s:openapi-gen=true
type RegistrationServiceVerificationSecret struct {
	// The reference to the secret that is expected to contain the keys below
	// +optional
	ToolchainSecret `json:",inline"`

	// TwilioAccountSID specifies the Twilio account identifier, used for sending phone verification messages
	// +optional
	TwilioAccountSID *string `json:"twilioAccountSID,omitempty"`

	// TwilioAuthToken specifies the Twilio authentication token, used for sending phone verification messages
	// +optional
	TwilioAuthToken *string `json:"twilioAuthToken,omitempty"`

	// TwilioFromNumber specifies the phone number or alphanumeric "Sender ID" for sending phone verification messages
	// +optional
	TwilioFromNumber *string `json:"twilioFromNumber,omitempty"`

	// AWSAccessKeyId is the AWS Access Key used to authenticate in order to access AWS services
	// +optional
	AWSAccessKeyID *string `json:"awsAccessKeyID,omitempty"`

	// AWSSecretAccessKey is the AWS credential used to authenticate in order to access AWS services
	// +optional
	AWSSecretAccessKey *string `json:"awsSecretAccessKey,omitempty"`

	// RecaptchaServiceAccountFile is the GCP service account file contents encoded in base64, it is
	// to be used with the recaptcha client for authentication
	// +optional
	RecaptchaServiceAccountFile *string `json:"recaptchaServiceAccountFile,omitempty"`
}

// CaptchaConfig defines any configuration related to captcha verification
// +k8s:openapi-gen=true
type CaptchaConfig struct {
	// Enabled specifies whether the captcha verification feature is enabled or not
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// ScoreThreshold defines the captcha assessment score threshold. A score equal to or above the threshold means the user is most likely human and
	// can proceed signing up but a score below the threshold means the score is suspicious and further verification may be required.
	// +optional
	ScoreThreshold *string `json:"scoreThreshold,omitempty"`

	// RequiredScore defines the lowest captcha score, below this score the user cannot proceed with the signup process at all.
	// Users with captcha score lower than the required one can still be approved manually.
	// +optional
	RequiredScore *string `json:"requiredScore,omitempty"`

	// AllowLowScoreReactivation specifies whether the reactivation for users with low captcha score (below the RequiredScore) is enabled without the need for manual approval.
	// +optional
	AllowLowScoreReactivation *bool `json:"allowLowScoreReactivation,omitempty"`

	// SiteKey defines the recaptcha site key to use when making recaptcha requests. There can be different ones for different environments. eg. dev, stage, prod
	// +optional
	SiteKey *string `json:"siteKey,omitempty"`

	// ProjectID defines the GCP project ID that has the recaptcha service enabled.
	// +optional
	ProjectID *string `json:"projectID,omitempty"`
}

// ToolchainStatusConfig contains all configuration parameters related to the toolchain status component
// +k8s:openapi-gen=true
type ToolchainStatusConfig struct {

	// ToolchainStatusRefreshTime specifies how often the ToolchainStatus should load and refresh the current hosted-toolchain status
	// +optional
	ToolchainStatusRefreshTime *string `json:"toolchainStatusRefreshTime,omitempty"`

	// Defines all secrets related to GitHub authentication/integration
	// +optional
	GitHubSecret GitHubSecret `json:"gitHubSecret,omitempty"`
}

// TiersConfig contains all configuration parameters related to tiers
// +k8s:openapi-gen=true
type TiersConfig struct {

	// DefaultUserTier specifies the default tier to assign for new users
	// +optional
	DefaultUserTier *string `json:"defaultUserTier,omitempty"`

	// DefaultSpaceTier specifies the default tier to assign for new spaces
	// +optional
	DefaultSpaceTier *string `json:"defaultSpaceTier,omitempty"`

	// DurationBeforeChangeTierRequestDeletion specifies the duration before a ChangeTierRequest resource is deleted
	// +optional
	DurationBeforeChangeTierRequestDeletion *string `json:"durationBeforeChangeTierRequestDeletion,omitempty"`

	// TemplateUpdateRequestMaxPoolSize specifies the maximum number of concurrent TemplateUpdateRequests
	// when updating MasterUserRecords
	// +optional
	TemplateUpdateRequestMaxPoolSize *int `json:"templateUpdateRequestMaxPoolSize,omitempty"`
}

// UsersConfig contains all configuration parameters related to users
// +k8s:openapi-gen=true
type UsersConfig struct {

	// MasterUserRecordUpdateFailureThreshold specifies the number of allowed failures before stopping attempts to update a MasterUserRecord
	// +optional
	MasterUserRecordUpdateFailureThreshold *int `json:"masterUserRecordUpdateFailureThreshold,omitempty"`

	// ForbiddenUsernamePrefixes is a comma-separated string that defines the prefixes that a username may not have when signing up.
	// If a username has a forbidden prefix, then the username compliance prefix is added to the username
	// +optional
	ForbiddenUsernamePrefixes *string `json:"forbiddenUsernamePrefixes,omitempty"`

	// ForbiddenUsernameSuffixes is a comma-separated string that defines the suffixes that a username may not have when signing up.  If a
	// username has a forbidden suffix, then the username compliance suffix is added to the username
	// +optional
	ForbiddenUsernameSuffixes *string `json:"forbiddenUsernameSuffixes,omitempty"`
}

// ToolchainConfigStatus defines the observed state of ToolchainConfig
// +k8s:openapi-gen=true
type ToolchainConfigStatus struct {

	// SyncErrors is a map of sync errors indexed by toolchaincluster name that indicates whether
	// an attempt to sync configuration to a member cluster failed
	// +optional
	// +mapType=atomic
	SyncErrors map[string]string `json:"syncErrors,omitempty"`

	// Conditions is an array of the current ToolchainConfig conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// SpaceConfig allows to configure Space provisioning related functionality.
// +k8s:openapi-gen=true
type SpaceConfig struct {

	// SpaceRequestEnabled specifies whether the SpaceRequest controller should start or not.
	// This is specifically useful in order to enable/disable this functionality from configuration (e.g. disabled by default in Sandbox and enabled only for AppStudio stage/prod ...).
	// +optional
	SpaceRequestEnabled *bool `json:"spaceRequestEnabled,omitempty"`

	// SpaceBindingRequestEnabled specifies whether the SpaceBindingRequest controller should start or not.
	// This is specifically useful in order to enable/disable this functionality from configuration (e.g. disabled by default in Sandbox and enabled only for AppStudio stage/prod ...).
	// +optional
	SpaceBindingRequestEnabled *bool `json:"spaceBindingRequestEnabled,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ToolchainConfig keeps all configuration parameters needed for host and member operators
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=toolchainconfigs,scope=Namespaced
// +kubebuilder:printcolumn:name="AutomaticApproval",type="boolean",JSONPath=`.spec.host.automaticApproval.enabled`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Toolchain Operator Config"
type ToolchainConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ToolchainConfigSpec   `json:"spec,omitempty"`
	Status ToolchainConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ToolchainConfigList contains a list of ToolchainConfig
type ToolchainConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ToolchainConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ToolchainConfig{}, &ToolchainConfigList{})
}
