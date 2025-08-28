package v1alpha1

const NamespaceTypeDefault = "default"

// SpaceNamespace is a common type to define the information about a namespace within a Space
// Used in NSTemplateSet, Space and Workspace status
type SpaceNamespace struct {

	// Name the name of the namespace.
	// +optional
	Name string `json:"name,omitempty"`

	// Type the type of the namespace. eg. default
	// +optional
	Type string `json:"type,omitempty"`
}
