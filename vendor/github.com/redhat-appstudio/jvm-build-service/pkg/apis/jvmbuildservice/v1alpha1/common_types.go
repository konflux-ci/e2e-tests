package v1alpha1

type SCMInfo struct {
	SCMURL  string `json:"scmURL,omitempty"`
	SCMType string `json:"scmType,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Path    string `json:"path,omitempty"`
}
