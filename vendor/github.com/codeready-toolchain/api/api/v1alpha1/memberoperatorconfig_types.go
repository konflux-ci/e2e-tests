package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MemberOperatorConfigSpec contains all configuration parameters of the member operator
// +k8s:openapi-gen=true
type MemberOperatorConfigSpec struct {
	// Keeps parameters concerned with authentication
	// +optional
	Auth AuthConfig `json:"auth,omitempty"`

	// Keeps parameters concerned with the autoscaler
	// +optional
	Autoscaler AutoscalerConfig `json:"autoscaler,omitempty"`

	// Keeps parameters concerned with Che/CRW
	// +optional
	Che CheConfig `json:"che,omitempty"`

	// Keeps parameters concerned with the console
	// +optional
	Console ConsoleConfig `json:"console,omitempty"`

	// Keeps parameters concerned with member status
	// +optional
	MemberStatus MemberStatusConfig `json:"memberStatus,omitempty"`

	// Keeps parameters concerned with the toolchaincluster
	// +optional
	ToolchainCluster ToolchainClusterConfig `json:"toolchainCluster,omitempty"`

	// Keeps parameters concerned with the webhook
	// +optional
	Webhook WebhookConfig `json:"webhook,omitempty"`
}

// Defines all parameters concerned with the autoscaler
// +k8s:openapi-gen=true
type AuthConfig struct {
	// Represents the configured identity provider
	// +optional
	Idp *string `json:"idp,omitempty"`
}

// Defines all parameters concerned with the autoscaler
// +k8s:openapi-gen=true
type AutoscalerConfig struct {
	// Defines the flag that determines whether to deploy the autoscaler buffer
	// +optional
	Deploy *bool `json:"deploy,omitempty"`

	// Represents how much memory should be required by the autoscaler buffer
	// +optional
	BufferMemory *string `json:"bufferMemory,omitempty"`

	// Represents the number of autoscaler buffer replicas to request
	// +optional
	BufferReplicas *int `json:"bufferReplicas,omitempty"`
}

// Defines all parameters concerned with Che
// +k8s:openapi-gen=true
type CheConfig struct {
	// Defines the Che/CRW Keycloak route name
	// +optional
	KeycloakRouteName *string `json:"keycloakRouteName,omitempty"`

	// Defines the Che/CRW route name
	// +optional
	RouteName *string `json:"routeName,omitempty"`

	// Defines the Che/CRW operator namespace
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Defines a flag that indicates whether the Che/CRW operator is required to be installed on the cluster. May be used in monitoring.
	// +optional
	Required *bool `json:"required,omitempty"`

	// Defines a flag to turn the Che user deletion logic on/off
	// +optional
	UserDeletionEnabled *bool `json:"userDeletionEnabled,omitempty"`

	// Defines all secrets related to Che configuration
	// +optional
	Secret CheSecret `json:"secret,omitempty"`
}

// Defines all secrets related to Che configuration
// +k8s:openapi-gen=true
type CheSecret struct {
	// The reference to the secret that is expected to contain the keys below
	// +optional
	ToolchainSecret `json:",inline"`

	// The key for the Che admin username in the secret values map
	// +optional
	CheAdminUsernameKey *string `json:"cheAdminUsernameKey,omitempty"`

	// The key for the Che admin password in the secret values map
	// +optional
	CheAdminPasswordKey *string `json:"cheAdminPasswordKey,omitempty"`
}

// Defines all parameters concerned with the console
// +k8s:openapi-gen=true
type ConsoleConfig struct {
	// Defines the console route namespace
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Defines the console route name
	// +optional
	RouteName *string `json:"routeName,omitempty"`
}

// Defines all parameters concerned with the toolchaincluster resource
// +k8s:openapi-gen=true
type ToolchainClusterConfig struct {
	// Defines the period in between health checks
	// +optional
	HealthCheckPeriod *string `json:"healthCheckPeriod,omitempty"`

	// Defines the timeout for each health check
	// +optional
	HealthCheckTimeout *string `json:"healthCheckTimeout,omitempty"`
}

// Defines all parameters concerned with the Webhook
// +k8s:openapi-gen=true
type WebhookConfig struct {
	// Defines the flag that determines whether to deploy the Webhook
	// +optional
	Deploy *bool `json:"deploy,omitempty"`
}

// Defines all parameters concerned with member status
// +k8s:openapi-gen=true
type MemberStatusConfig struct {
	// Defines the period between refreshes of the member status
	// +optional
	RefreshPeriod *string `json:"refreshPeriod,omitempty"`
}

// MemberOperatorConfigStatus defines the observed state of MemberOperatorConfig
// +k8s:openapi-gen=true
type MemberOperatorConfigStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MemberOperatorConfig keeps all configuration parameters needed in member operator
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=memberoperatorconfigs,scope=Namespaced
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Member Operator Config"
type MemberOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MemberOperatorConfigSpec   `json:"spec,omitempty"`
	Status MemberOperatorConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MemberOperatorConfigList contains a list of MemberOperatorConfig
type MemberOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MemberOperatorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MemberOperatorConfig{}, &MemberOperatorConfigList{})
}
