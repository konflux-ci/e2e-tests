package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// OpenShiftRouteTarget captures the look up information for retrieving an OpenShift Route object in the member cluster.
type OpenShiftRouteTarget struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// ProxyPluginSpec defines the desired state of ProxyPlugin
// +k8s:openapi-gen=true
type ProxyPluginSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// OpenShiftRouteTargetEndpoint is an optional field that represents the look up information for an OpenShift Route
	// as the endpoint for the registration service to proxy requests to that have the https://<proxy-host>/plugins/<ProxyPlugin.ObjectMeta.Name>
	// in its incoming URL.  As we add more types besides OpenShift Routes, we will add more optional fields to this spec
	// object
	// +optional
	OpenShiftRouteTargetEndpoint *OpenShiftRouteTarget `json:"openShiftRouteTargetEndpoint,omitempty"`
}

// ProxyPluginStatus defines the observed state of ProxyPlugin
// +k8s:openapi-gen=true
type ProxyPluginStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Conditions is an array of current Proxy Plugin conditions
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

// ProxyPlugin represents the configuration to handle GET's to k8s services in member clusters that first route through
// the registration service running in the sandbox host cluster.  Two forms of URL are supported:
// https://<proxy-host>/plugins/<ProxyPlugin.ObjectMeta.Name>/v1alpha2/<namespace-name>/
// https://<proxy-host>/plugins/<ProxyPlugin.ObjectMeta.Name>/workspaces/<workspace-name>/v1alpha2/<namespace-name>
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Proxy Plugin"
type ProxyPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxyPluginSpec   `json:"spec,omitempty"`
	Status ProxyPluginStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProxyPluginList contains a list of ProxyPlugin
type ProxyPluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxyPlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxyPlugin{}, &ProxyPluginList{})
}
