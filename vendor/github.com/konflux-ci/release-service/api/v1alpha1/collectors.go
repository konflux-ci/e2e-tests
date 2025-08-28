package v1alpha1

// Collector represents a reference to a Collector to be executed as part of the release workflow.
// +kubebuilder:object:generate=true
type Collector struct {
	// Name of the collector
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Name string `json:"name"`

	// Timeout in seconds for the collector to execute
	// +optional
	Timeout int `json:"timeout,omitempty"`

	// Type is the type of collector to be used
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Type string `json:"type"`

	// Params is a slice of parameters for a given collector
	Params []Param `json:"params"`
}

// Param represents a parameter for a collector
type Param struct {
	// Name is the name of the parameter
	Name string `json:"name"`

	// Value is the value of the parameter
	Value string `json:"value"`
}
