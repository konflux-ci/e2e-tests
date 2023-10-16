//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type SPIFileContentRequestSpec struct {
	// FilePath defines target file path inside repository
	FilePath string `json:"filePath"`
	// RepoUrl defines target file repository
	RepoUrl string `json:"repoUrl"`
	// Ref defines target git reference (tag/branch/commit)
	// +optional
	Ref string `json:"ref,omitempty"`
}

type SPIFileContentRequestStatus struct {
	// Phase of the current file request
	Phase SPIFileContentRequestPhase `json:"phase"`
	// LinkedBindingName name of the binding used for repository authentication
	// +optional
	LinkedBindingName string `json:"linkedBindingName"`
	// ErrorMessage defines error message if file request failed
	// + optional
	ErrorMessage string `json:"errorMessage,omitempty"`
	// OAuthUrl URL to authenticate into target repository using OAuth
	// +optional
	OAuthUrl string `json:"oAuthUrl,omitempty"`
	// TokenUploadUrl URL to perform manual upload of the token to access target repository
	// +optional
	TokenUploadUrl string `json:"tokenUploadUrl,omitempty"`
	// Content encoded target file content
	// +optional
	Content string `json:"content,omitempty"`
	// ContentEncoding encoding used for file content
	// +optional
	ContentEncoding string `json:"contentEncoding,omitempty"`
}

type SPIFileContentRequestPhase string

const (
	SPIFileContentRequestPhaseAwaitingBinding   SPIFileContentRequestPhase = "AwaitingBinding"
	SPIFileContentRequestPhaseAwaitingTokenData SPIFileContentRequestPhase = "AwaitingTokenData"
	SPIFileContentRequestPhaseDelivered         SPIFileContentRequestPhase = "Delivered"
	SPIFileContentRequestPhaseError             SPIFileContentRequestPhase = "Error"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type SPIFileContentRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SPIFileContentRequestSpec   `json:"spec,omitempty"`
	Status SPIFileContentRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SPIFileContentRequestList contains a list of SPIAccessTokenBinding
type SPIFileContentRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SPIFileContentRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SPIFileContentRequest{}, &SPIFileContentRequestList{})
}

func (in *SPIFileContentRequest) RepoUrl() string {
	return in.Spec.RepoUrl
}
