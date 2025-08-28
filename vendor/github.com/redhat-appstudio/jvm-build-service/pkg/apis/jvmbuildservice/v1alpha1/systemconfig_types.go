package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	DefaultRecipeDatabase = "https://github.com/redhat-appstudio/jvm-build-data"
)

type SystemConfigSpec struct {
	Builders            map[string]BuilderImageInfo `json:"builders,omitempty"`
	MaxAdditionalMemory int                         `json:"maxAdditionalMemory,omitempty"`
	RecipeDatabase      string                      `json:"recipeDatabase,omitempty"`
}

type BuilderImageInfo struct {
	Image    string `json:"image,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

type SystemConfigStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=systemconfigs,scope=Cluster
// SystemConfig TODO provide godoc description
type SystemConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SystemConfigSpec   `json:"spec"`
	Status SystemConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SystemConfigList contains a list of SystemConfig
type SystemConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SystemConfig `json:"items"`
}
