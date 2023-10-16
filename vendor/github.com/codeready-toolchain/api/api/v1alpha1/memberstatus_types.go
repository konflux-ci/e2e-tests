package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MemberStatusSpec defines the desired state of MemberStatus
// +k8s:openapi-gen=true
type MemberStatusSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// spec is intentionally empty since only the status fields will be used for reporting status of the toolchain
}

// MemberStatusStatus defines the observed state of the toolchain member status
// +k8s:openapi-gen=true
type MemberStatusStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Che is the status of Che/CRW, such as installed and whether the member configuration is correct
	// +optional
	Che *CheStatus `json:"che,omitempty"`

	// MemberOperator is the status of a toolchain member operator
	// +optional
	MemberOperator *MemberOperatorStatus `json:"memberOperator,omitempty"`

	// HostConnection is the status of the connection with the host cluster
	// +optional
	HostConnection *ToolchainClusterStatus `json:"hostConnection,omitempty"`

	// Host is the status of the connection with the host cluster
	// +optional
	Host *HostStatus `json:"host,omitempty"`

	// Conditions is an array of current toolchain status conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Resource usage of the cluster
	// +optional
	ResourceUsage ResourceUsage `json:"resourceUsage,omitempty"`

	// Routes/URLs of the cluster, such as Console and Che Dashboard URLs
	// +optional
	Routes *Routes `json:"routes,omitempty"`
}

// CheStatus contains information about the status of Che/CRW, such as installed and whether the member configuration is correct
// +k8s:openapi-gen=true
type CheStatus struct {
	// Conditions is an array of current Che status conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// Routes contains information about the public routes available to the user in the cluster
// +k8s:openapi-gen=true
type Routes struct {
	// ConsoleURL is the web console URL of the cluster
	// +optional
	ConsoleURL string `json:"consoleURL,omitempty"`

	// CheDashboardURL is the Che Dashboard URL of the cluster if Che is installed
	// +optional
	CheDashboardURL string `json:"cheDashboardURL,omitempty"`

	// Conditions is an array of current member operator status conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// Contains information about the resource usage of the cluster
// +k8s:openapi-gen=true
type ResourceUsage struct {
	// How many percent of the available memory is used per node role (eg. worker, master)
	// +optional
	MemoryUsagePerNodeRole map[string]int `json:"memoryUsagePerNodeRole,omitempty"`
}

// HostStatus defines the status of the connection with the host cluster
type HostStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Conditions is an array of current member operator status conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// MemberOperatorStatus defines the observed state of a toolchain's member operator
type MemberOperatorStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// The version of the operator
	Version string `json:"version"`

	// The commit id from the member-operator repository used to build the operator
	Revision string `json:"revision"`

	// The timestamp of the member operator build
	BuildTimestamp string `json:"buildTimestamp"`

	// The status of the member operator's deployment
	DeploymentName string `json:"deploymentName"`

	// Conditions is an array of current member operator status conditions
	// Supported condition types: ConditionReady
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MemberStatus is used to track toolchain member status
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Last Updated",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].lastUpdatedTime`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="CodeReady Toolchain Member Status"
type MemberStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MemberStatusSpec   `json:"spec,omitempty"`
	Status MemberStatusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MemberStatusList contains a list of MemberStatus
type MemberStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MemberStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MemberStatus{}, &MemberStatusList{})
}
