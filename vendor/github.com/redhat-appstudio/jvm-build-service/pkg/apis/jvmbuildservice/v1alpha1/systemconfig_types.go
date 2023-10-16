package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	JDK7Builder           = "jdk7"
	JDK8Builder           = "jdk8"
	JDK11Builder          = "jdk11"
	JDK17Builder          = "jdk17"
	DefaultRecipeDatabase = "https://github.com/redhat-appstudio/jvm-build-data"
)

type QuotaImpl string

type SystemConfigSpec struct {
	Builders            map[string]JavaVersionInfo `json:"builders,omitempty"`
	MaxAdditionalMemory int                        `json:"maxAdditionalMemory,omitempty"`
	//DEPRECATED
	Quota          QuotaImpl `json:"quota,omitempty"`
	RecipeDatabase string    `json:"recipeDatabase,omitempty"`
}

type JavaVersionInfo struct {
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
