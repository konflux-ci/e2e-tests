/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EnterpriseContractPolicySpec is used to configure the Enterprise Contract Policy
type EnterpriseContractPolicySpec struct {
	// Description of the policy or its intended use
	// +optional
	Description string `json:"description,omitempty"`
	// One or more groups of policy rules
	// +kubebuilder:validation:MinItems:=1
	Sources []Source `json:"sources,omitempty"`
	// Authorization for per component release approvals
	// +optional
	Authorization *Authorization `json:"authorization,omitempty"`
	// Configuration handles policy modification configuration (collections, exclusions, inclusions)
	// +optional
	Configuration *EnterpriseContractPolicyConfiguration `json:"configuration,omitempty"`
	// URL of the Rekor instance. Empty string disables Rekor integration
	// +optional
	RekorUrl string `json:"rekorUrl,omitempty"`
	// Public key used to validate the signature of images and attestations
	// +optional
	PublicKey string `json:"publicKey,omitempty"`
}

// Source defines policies and data that are evaluated together
type Source struct {
	// Optional name for the source
	// +optional
	Name string `json:"name,omitempty"`
	// List of go-getter style policy source urls
	// +kubebuilder:validation:MinItems:=1
	Policy []string `json:"policy,omitempty"`
	// List of go-getter style policy data source urls
	// +optional
	Data []string `json:"data,omitempty"`
}

// Authorization defines a release approval
type Authorization struct {
	// Components based authorization
	// +optional
	Components []AuthorizedComponent `json:"components,omitempty"`
}

// Authorization defines a release approval on a component basis
type AuthorizedComponent struct {
	// ChangeID is the identifier of the change, e.g. git commit id
	// +optional
	ChangeID string `json:"changeId,omitempty"`
	// Repository of the component sources
	// +optional
	Repository string `json:"repository,omitempty"`
	// Authorizer is the email address of the person authorizing the release
	// +optional
	Authorizer string `json:"authorizer,omitempty"`
}

// EnterpriseContractPolicyConfiguration configuration of modifications to policy evaluation
type EnterpriseContractPolicyConfiguration struct {
	// Exclude set of policy exclusions that, in case of failure, do not block
	// the success of the outcome.
	// +optional
	// +listType:=set
	Exclude []string `json:"exclude,omitempty"`
	// Include set of policy inclusions that are added to the policy evaluation.
	// These override excluded rules.
	// +optional
	// +listType:=set
	Include []string `json:"include,omitempty"`
	// Collections set of predefined rules.
	// +optional
	// +listType:=set
	Collections []string `json:"collections,omitempty"`
}

// EnterpriseContractPolicyStatus defines the observed state of EnterpriseContractPolicy
type EnterpriseContractPolicyStatus struct {
	// TODO what to add here?
	// ideas;
	// - on what the policy was applied
	// - history of changes
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all
// +kubebuilder:resource:shortName=ecp
// +kubebuilder:subresource:status
// EnterpriseContractPolicy is the Schema for the enterprisecontractpolicies API
type EnterpriseContractPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnterpriseContractPolicySpec   `json:"spec,omitempty"`
	Status EnterpriseContractPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnterpriseContractPolicyList contains a list of EnterpriseContractPolicy
type EnterpriseContractPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnterpriseContractPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EnterpriseContractPolicy{}, &EnterpriseContractPolicyList{})
}
