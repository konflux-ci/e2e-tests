package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=tektonwrappers,scope=Namespaced
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// TektonWrapper allows for jvm build service to potentially throttle its creation of PipelineRuns based on current
// PipelineRuns in progress and cluster quotas and limits
type TektonWrapper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonWrapperSpec   `json:"spec"`
	Status TektonWrapperStatus `json:"status,omitempty"`
}

// TektonWrapperSpec is simply the specification of this API
type TektonWrapperSpec struct {
	// PipelineRun the wrappered Tekton Object, in this case a PipelineRun, that we want the Reconciler to create if
	// cluster resource utilization allows
	// NOTE: Tekton's current lack of compatability with controller-gen for the PipelineRun type prevents us from making
	// type type *v1beta1.PipelineRun; then, attempts to use k8s RawExtension with controller-gen and deepcopy were also
	// unsuccessful, so we are just using a vanilla byte array; as this is a transient object, versioning is note considered
	// to be an issue, so not using RawExtension seems OK.
	PipelineRun []byte `json:"pipelineRun,omitempty"`
	// RequeueAfter if greater than 0, tells the Reconciler the amount of time to delay creation of the embedded
	// PipelineRun if cluster resource utilization tells the Reconciler to not create at this time.  A value of 0 will
	// mean the Reconciler uses its default value
	RequeueAfter time.Duration `json:"requeueAfter,omitempty"`
	// AbandonAfter if greater than 0, tells the Reconciler the amount of time after the creation of this object to
	// no longer attempt to create the embedded PipelineRun if cluster resource utilizaiton tell the Reconciler to not
	// create at this time.  A value of 0 will mean the Reconciler uses its default value
	AbandonAfter time.Duration `json:"abandonAfter,omitempty"`
}

const (
	// TektonWrapperStateUnprocessed The creation of the embedded PipelineRun has not been attempted, previously failed
	TektonWrapperStateUnprocessed = "TektonWrapperStateUnprocessed"
	// TektonWrapperStateThrottled
	TektonWrapperStateThrottled = "TektonWrapperStateThrottled"
	// TektonWrapperStateInProgress The creation of the embedded PipelineRun has occurred but the PipelineRun has not reached a terminal state
	TektonWrapperStateInProgress = "TektonWrapperStateInProgress"
	// TektonWrapperStateAbandoned The creation of the embedded PipelineRun was abandoned because of cluster resource constraints
	TektonWrapperStateAbandoned = "TektonWrapperStateAbandoned"
	// TektonWrapperStateComplete The embedded PipelineRun has reached a terminal statue
	TektonWrapperStateComplete = "TektonWrapperStateComplete"
)

// TektonWrapperStatus is where the Reconciler maintains the finite state machine for this API
type TektonWrapperStatus struct {
	// State reveals whether the creation of the embedded PipelineRn has been attempted or not, or whether it was successful or not
	State string `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TektonWrapperLlist contains a list of TektonWrapper
type TektonWrapperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonWrapper `json:"items"`
}
