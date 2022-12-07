package v1alpha1

import (
	templatev1 "github.com/openshift/api/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ClusterResourcesType contains a type name of the template containing cluster-scoped resources
	ClusterResourcesTemplateType = "clusterresources"

	// TierTemplateObjectOptionalResourceAnnotation is annotation to be used to mark a TierTemplate object as optional.
	// That means that it won't be applied if the corresponding API group is not present in the cluster.
	TierTemplateObjectOptionalResourceAnnotation = LabelKeyPrefix + "optional-resource"
)

// TierTemplateSpec defines the desired state of TierTemplate
// +k8s:openapi-gen=true
type TierTemplateSpec struct {

	// The tier of the template. For example: "basic", "advanced", or "team"
	TierName string `json:"tierName"`

	// The type of the template. For example: "code", "dev", "stage" or "cluster"
	Type string `json:"type"`

	// The revision of the corresponding template
	Revision string `json:"revision"`

	// Template contains an OpenShift Template to be used to provision either a user's namespace or cluster-wide resources
	Template templatev1.Template `json:"template"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TierTemplate is the Schema for the tiertemplates API
// +kubebuilder:resource:path=tiertemplates,scope=Namespaced
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=`.spec.revision`
// +kubebuilder:validation:XPreserveUnknownFields
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Template Tier"
type TierTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TierTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// TierTemplateList contains a list of TierTemplate
type TierTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TierTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TierTemplate{}, &TierTemplateList{})
}
