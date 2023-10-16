/*
Copyright 2021.

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

package v1beta1

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RemoteSecretSpec defines the desired state of RemoteSecret
type RemoteSecretSpec struct {
	// Secret defines the properties of the secret and the linked service accounts that should be
	// created in the target namespaces.
	Secret LinkableSecretSpec `json:"secret"`
	// Targets is the list of the target namespaces that the secret and service accounts should be deployed to.
	// +optional
	Targets []RemoteSecretTarget `json:"targets,omitempty"`
}

type RemoteSecretTarget struct {
	// Namespace is the name of the target namespace to which to deploy.
	Namespace string `json:"namespace,omitempty"`
	// ApiUrl specifies the URL of the API server of a remote Kubernetes cluster that this target points to. If left empty,
	// the local cluster is assumed.
	ApiUrl string `json:"apiUrl,omitempty"`
	// ClusterCredentialsSecret is the name of the secret in the same namespace as the RemoteSecret that contains the token
	// to use to authenticate with the remote Kubernetes cluster. This is ignored if `apiUrl` is empty.
	ClusterCredentialsSecret string `json:"clusterCredentialsSecret,omitempty"`
}

// RemoteSecretStatus defines the observed state of RemoteSecret
type RemoteSecretStatus struct {
	// Conditions is the list of conditions describing the state of the deployment
	// to the targets.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Targets is the list of the deployment statuses for individual targets in the spec.
	// +optional
	Targets []TargetStatus `json:"targets,omitempty"`
}

type TargetStatus struct {
	// Namespace is the namespace of the target where the secret and the service accounts have been deployed to.
	Namespace string `json:"namespace"`
	// ApiUrl is the URL of the remote Kubernetes cluster to which the target points to.
	ApiUrl string `json:"apiUrl,omitempty"`
	// SecretName is the name of the secret that is actually deployed to the target namespace
	SecretName string `json:"secretName"`
	// ServiceAccountNames is the names of the service accounts that have been deployed to the target namespace
	// +optional
	ServiceAccountNames []string `json:"serviceAccountNames,omitempty"`
	// Error the optional error message if the deployment of either the secret or the service accounts failed.
	// +optional
	Error string `json:"error,omitempty"`
}

// RemoteSecretReason is the reconciliation status of the RemoteSecret object
type RemoteSecretReason string

// RemoteSecretConditionType lists the types of conditions we track in the remote secret status
type RemoteSecretConditionType string

const (
	RemoteSecretConditionTypeDeployed     RemoteSecretConditionType = "Deployed"
	RemoteSecretConditionTypeDataObtained RemoteSecretConditionType = "DataObtained"

	RemoteSecretReasonAwaitingTokenData RemoteSecretReason = "AwaitingData"
	RemoteSecretReasonDataFound         RemoteSecretReason = "DataFound"
	RemoteSecretReasonInjected          RemoteSecretReason = "Injected"
	RemoteSecretReasonPartiallyInjected RemoteSecretReason = "PartiallyInjected"
	RemoteSecretReasonError             RemoteSecretReason = "Error"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RemoteSecret is the Schema for the RemoteSecret API
type RemoteSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteSecretSpec   `json:"spec,omitempty"`
	Status RemoteSecretStatus `json:"status,omitempty"`
}

var secretTypeMismatchError = errors.New("the type of upload secret and remote secret spec do not match")

// ValidateUploadSecretType checks weather the uploadSecret type matches the RemoteSecret type.
// The function is in the api package because it extends the contract of the CRD.
// In the future the function can be extended to validate other fields.
func (rs *RemoteSecret) ValidateUploadSecretType(uploadSecret *corev1.Secret) error {
	defaultize := func(secretType corev1.SecretType) corev1.SecretType {
		if secretType == "" {
			return corev1.SecretTypeOpaque
		}
		return secretType
	}

	if defaultize(uploadSecret.Type) != defaultize(rs.Spec.Secret.Type) {
		return fmt.Errorf("%w, uploadSecret: %s, remoteSecret: %s", secretTypeMismatchError, uploadSecret.Type, rs.Spec.Secret.Type)
	}
	return nil
}

//+kubebuilder:object:root=true

// RemoteSecretList contains a list of RemoteSecret
type RemoteSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteSecret{}, &RemoteSecretList{})
}

type LinkableSecretSpec struct {
	// Name is the name of the secret to be created. If it is not defined a random name based on the name of the binding
	// is used.
	// +optional
	Name         string `json:"name,omitempty"`
	GenerateName string `json:"generateName,omitempty"`
	// Labels contains the labels that the created secret should be labeled with.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is the keys and values that the create secret should be annotated with.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Type is the type of the secret to be created. If left empty, the default type used in the cluster is assumed (typically Opaque).
	// The type of the secret defines the automatic mapping of the token record fields to keys in the secret data
	// according to the documentation https://kubernetes.io/docs/concepts/configuration/secret/#secret-types.
	// Only kubernetes.io/service-account-token, kubernetes.io/dockercfg, kubernetes.io/dockerconfigjson and kubernetes.io/basic-auth
	// are supported. All other secret types need to have their mapping specified manually using the Fields.
	Type corev1.SecretType `json:"type,omitempty"`

	// LinkedTo specifies the objects that the secret is linked to. Currently, only service accounts are supported.
	LinkedTo []SecretLink `json:"linkedTo,omitempty"`
}
type SecretLink struct {
	// ServiceAccounts lists the service accounts that the secret is linked to.
	ServiceAccount ServiceAccountLink `json:"serviceAccount,omitempty"`
}

type ServiceAccountLink struct {
	// As specifies how the secret generated by the binding is linked to the service account.
	// This can be either `secret` meaning that the secret is listed as one of the mountable secrets
	// in the `secrets` of the service account, `imagePullSecret` which makes the secret listed as
	// one of the image pull secrets associated with the service account. If not specified, it defaults
	// to `secret`.
	// +optional
	// +kubebuilder:default:=secret
	As ServiceAccountLinkType `json:"as,omitempty"`
	// Reference specifies a pre-existing service account that the secret should be linked to. It is an error
	// if the service account doesn't exist when the operator tries to add a link to a secret with the injected
	// token.
	Reference corev1.LocalObjectReference `json:"reference,omitempty"`
	// Managed specifies the service account that is bound to the lifetime of the binding. This service account
	// must not exist and is created and deleted along with the injected secret.
	Managed ManagedServiceAccountSpec `json:"managed,omitempty"`
}

type ManagedServiceAccountSpec struct {
	// Name is the name of the service account to create/link. Either this or GenerateName
	// must be specified.
	// +optional
	Name string `json:"name"`
	// GenerateName is the generate name to be used when creating the service account. It only
	// really makes sense for the Managed service accounts that are cleaned up with the binding.
	// +optional
	GenerateName string `json:"generateName"`
	// Labels contains the labels that the created service account should be labeled with.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is the keys and values that the created service account should be annotated with.
	Annotations map[string]string `json:"annotations,omitempty"`
}
type ServiceAccountLinkType string

const (
	ServiceAccountLinkTypeSecret          ServiceAccountLinkType = "secret"
	ServiceAccountLinkTypeImagePullSecret ServiceAccountLinkType = "imagePullSecret"
)

// EffectiveSecretLinkType returns the secret link type applying the default value if LinkedSecretAs is unspecified by
// the user.
func (s *ServiceAccountLink) EffectiveSecretLinkType() ServiceAccountLinkType {
	if s.As == ServiceAccountLinkTypeImagePullSecret {
		return ServiceAccountLinkTypeImagePullSecret
	}
	return ServiceAccountLinkTypeSecret
}

type RemoteSecretErrorReason string

const (
	RemoteSecretErrorReasonTokenRetrieval RemoteSecretErrorReason = "TokenRetrieval"
	RemoteSecretErrorReasonNoError        RemoteSecretErrorReason = ""
)
