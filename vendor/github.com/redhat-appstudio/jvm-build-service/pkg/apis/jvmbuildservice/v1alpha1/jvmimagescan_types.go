package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JvmImageScanSpec struct {
	Image string `json:"image,omitempty"`
}

type JvmImageScanStatus struct {
	State   JvmImageDependenciesState `json:"state,omitempty"`
	Message string                    `json:"message,omitempty"`
	Results []JavaDependency          `json:"results,omitempty"`
}
type JavaDependency struct {
	GAV        string            `json:"gav,omitempty"`
	Source     string            `json:"source,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type JvmImageDependenciesState string

const (
	// JvmImageScanStateNew A new resource that has not been acted on by the operator
	JvmImageScanStateNew JvmImageDependenciesState = "JvmImageScanNew"
	// JvmImageScanStateDiscovering The discovery pipeline is running to try and figure out what is in the image
	JvmImageScanStateDiscovering JvmImageDependenciesState = "JvmImageScanDiscovering"
	// JvmImageScanStateFailed The build failed
	JvmImageScanStateFailed JvmImageDependenciesState = "JvmImageScanFailed"
	// JvmImageScanStateComplete The discovery completed successfully, the resource can be removed
	JvmImageScanStateComplete JvmImageDependenciesState = "JvmImageScanComplete"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=jvmimagescans,scope=Namespaced
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// JvmImageScan TODO provide godoc description
type JvmImageScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JvmImageScanSpec   `json:"spec"`
	Status JvmImageScanStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JvmImageScanList contains a list of JvmImageScan
type JvmImageScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JvmImageScan `json:"items"`
}
