package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These are valid status condition reasons for Toolchain status
const (
	// ToolchainStatusUnreadyNotificationCreated is used when a notification has been sent to an admin mailing list
	// after the toolchain status has been in an "unready" condition for an extended period of time
	ToolchainStatusUnreadyNotificationCreated ConditionType = "ToolchainStatusUnreadyNotificationCreated"

	// ToolchainStatusUnreadyNotificationCRCreatedReason is used to indicate the Unready Notification CR has been created
	ToolchainStatusUnreadyNotificationCRCreatedReason = "UnreadyNotificationCRCreated"

	// ToolchainStatusUnreadyNotificationCRCreationFailedReason is set when the controller fails to create an unready notification CR
	ToolchainStatusUnreadyNotificationCRCreationFailedReason = "UnreadyNotificationCRCreationFailed"

	// ToolchainStatusRestoredNotificationCRCreationFailedReason is set when the controller fails to create restored notification
	ToolchainStatusRestoredNotificationCRCreationFailedReason = "RestoredNotificationCRCreationFailed"

	// overall status condition reasons
	ToolchainStatusAllComponentsReadyReason = "AllComponentsReady"
	ToolchainStatusComponentsNotReadyReason = "ComponentsNotReady"

	// deployment reasons
	ToolchainStatusDeploymentReadyReason    = "DeploymentReady"
	ToolchainStatusDeploymentNotReadyReason = "DeploymentNotReady"
	ToolchainStatusDeploymentNotFoundReason = "DeploymentNotFound"

	// host connection reasons
	ToolchainStatusClusterConnectionReadyReason                 = "HostConnectionReady"
	ToolchainStatusClusterConnectionNotReadyReason              = "HostConnectionNotReady"
	ToolchainStatusClusterConnectionNotFoundReason              = "ToolchainClusterNotFound"
	ToolchainStatusClusterConnectionLastProbeTimeExceededReason = "ToolchainClusterLastProbeTimeExceeded"

	// registration service reasons
	ToolchainStatusRegServiceReadyReason    = "RegServiceReady"
	ToolchainStatusRegServiceNotReadyReason = "RegServiceNotReady"

	// Host routes reasons
	ToolchainStatusProxyRouteUnavailableReason = "ProxyRouteUnavailable"
	ToolchainStatusHostRoutesAvailableReason   = "HostRoutesAvailable"

	// member status reasons
	ToolchainStatusMemberStatusNotFoundReason                  = "MemberStatusNotFound"
	ToolchainStatusMemberToolchainClusterMissingReason         = "MemberToolchainClusterMissing"
	ToolchainStatusMemberStatusConsoleRouteUnavailableReason   = "ConsoleRouteUnavailable"
	ToolchainStatusMemberStatusRoutesAvailableReason           = "RoutesAvailable"
	ToolchainStatusMemberStatusCheRouteUnavailableReason       = "CheRouteUnavailable"
	ToolchainStatusMemberStatusCheUserAPICheckFailedReason     = "CheUserAPICheckFailed"
	ToolchainStatusMemberStatusCheNotRequiredReason            = "CheNotRequired"
	ToolchainStatusMemberStatusCheAdminUserNotConfiguredReason = "CheAdminUserNotConfigured"
	ToolchainStatusMemberStatusCheUserDeletionNotEnabledReason = "CheUserDeletionNotEnabled"
	ToolchainStatusMemberStatusCheReadyReason                  = "CheReady"

	// Metric Keys
	// MasterUserRecordsPerDomainMetricKey the key to store the metric for the number of MasterUserRecords per email address domain
	MasterUserRecordsPerDomainMetricKey = "masterUserRecordsPerDomain"
	// UserSignupsPerActivationAndDomainMetricKey the key to store the metric for the number of UserSignups per activations and per email address domain
	UserSignupsPerActivationAndDomainMetricKey = "userSignupsPerActivationAndDomain"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ToolchainStatusSpec defines the desired state of ToolchainStatus
// +k8s:openapi-gen=true
type ToolchainStatusSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// spec is intentionally empty since only the status fields will be used for reporting status of the toolchain
}

// ToolchainStatusStatus defines the observed state of the toolchain, including host cluster and member cluster components
// +k8s:openapi-gen=true
type ToolchainStatusStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// HostOperator is the status of a toolchain host operator
	// +optional
	HostOperator *HostOperatorStatus `json:"hostOperator,omitempty"`

	// RegistrationService is the status of the registration service
	// +optional
	RegistrationService *HostRegistrationServiceStatus `json:"registrationService,omitempty"`

	// Members is an array of member status objects
	// +optional
	// +patchMergeKey=clusterName
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=clusterName
	Members []Member `json:"members,omitempty" patchStrategy:"merge" patchMergeKey:"clusterName"`

	// Metrics is a map that stores metrics to be exposed on Prometheus.
	// +optional
	// +mapType=atomic
	// +patchStrategy=merge
	Metrics map[string]Metric `json:"metrics,omitempty" patchStrategy:"merge"`

	// HostRoutes/URLs of the host cluster, such as Proxy URL
	// +optional
	HostRoutes HostRoutes `json:"hostRoutes,omitempty"`

	// Conditions is an array of the current overall toolchain status conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// HostRoutes contains information about the public routes available to the user in the host cluster
// +k8s:openapi-gen=true
type HostRoutes struct {
	// ProxyURL is the Proxy URL of the cluster
	// +optional
	ProxyURL string `json:"proxyURL,omitempty"`

	// Conditions is an array of current member operator status conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type Metric map[string]int

// HostOperatorStatus defines the observed state of a toolchain's host operator
// +k8s:openapi-gen=true
type HostOperatorStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// The version of the operator
	Version string `json:"version"`

	// The commit id from the host-operator repository used to build the operator
	Revision string `json:"revision"`

	// The timestamp of the host operator build
	BuildTimestamp string `json:"buildTimestamp"`

	// The status of the host operator's deployment
	DeploymentName string `json:"deploymentName"`

	// Conditions is an array of current host operator status conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// HostRegistrationServiceStatus defines the observed state of a toolchain's registration service
// +k8s:openapi-gen=true
type HostRegistrationServiceStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Deployment is the status of the registration service's deployment
	Deployment RegistrationServiceDeploymentStatus `json:"deployment"`

	// RegistrationServiceResources is the status for resources created for the registration service
	RegistrationServiceResources RegistrationServiceResourcesStatus `json:"registrationServiceResources"`

	// Health provides health status of the registration service
	Health RegistrationServiceHealth `json:"health"`
}

// RegistrationServiceDeploymentStatus contains status of the registration service's deployment
// +k8s:openapi-gen=true
type RegistrationServiceDeploymentStatus struct {
	// The host operator deployment name
	Name string `json:"name"`

	// Conditions is an array of current deployment status conditions for a host operator
	// Supported condition types: Available, Progressing
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// RegistrationServiceHealth contains health status of the registration service
// +k8s:openapi-gen=true
type RegistrationServiceHealth struct {
	Alive       string `json:"alive"`
	BuildTime   string `json:"buildTime"`
	Environment string `json:"environment"`
	Revision    string `json:"revision"`
	StartTime   string `json:"startTime"`

	// Conditions is an array of status conditions for the health of the registration service
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// Member contains the status of a member cluster
// +k8s:openapi-gen=true
type Member struct {
	// APIEndpoint is the server API URL of the cluster
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`

	// The cluster identifier
	ClusterName string `json:"clusterName"`

	// Number of Spaces created within the member cluster
	// +optional
	SpaceCount int `json:"spaceCount,omitempty"`

	// The array of member status objects
	MemberStatus MemberStatusStatus `json:"memberStatus"`
}

// RegistrationServiceResourcesStatus contains conditions for creation/deployment of registration service resources
// +k8s:openapi-gen=true
type RegistrationServiceResourcesStatus struct {
	// Conditions is an array of current registration service resource status conditions
	// Supported condition types: Deployed, Deploying, DeployingFailed
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ToolchainStatus is used to track overall toolchain status
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="MURs",type="integer",JSONPath=`.status.hostOperator.masterUserRecordCount`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Last Updated",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].lastUpdatedTime`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="CodeReady Toolchain Status"
type ToolchainStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ToolchainStatusSpec   `json:"spec,omitempty"`
	Status ToolchainStatusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ToolchainStatusList contains a list of ToolchainStatus
type ToolchainStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ToolchainStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ToolchainStatus{}, &ToolchainStatusList{})
}
