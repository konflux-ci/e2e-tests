/*
Copyright 2021-2022 Red Hat, Inc.

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
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// GitSource describes the Component source
type GitSource struct {
	// If importing from git, the repository to create the component from
	URL string `json:"url"`
}

// KubernetesResources define the list of Kubernetes resources
type KubernetesResources struct {
	Deployments []appsv1.Deployment
	Services    []corev1.Service
	Routes      []routev1.Route
	Ingresses   []networkingv1.Ingress
	Others      []interface{}
}

// GeneratorOptions - This captures the options for generating the component's GitOps resources for a component of an
// application. Currently, it's the kubernetes deployment, service and route resources. Applications are a set of
// components that run together on environments.
type GeneratorOptions struct {
	// Name is the name of the component.
	Name string `json:"name"`

	// Namespace is the namespace of the component
	Namespace string `json:"namespace,omitempty"`

	// K8sLabels is the labels to add to all the generated kubernetes resources
	K8sLabels map[string]string `json:"K8sLabels,omitempty"`

	// Application to add the component to
	Application string `json:"application"`

	// Secret describes the name of a Kubernetes secret containing either:
	// 1. A Personal Access Token to access the Component's git repository (if using a Git-source component) or
	// 2. An Image Pull Secret to access the Component's container image (if using an Image-source component).
	Secret string `json:"secret,omitempty"`

	// GitSource describes the Component's source
	GitSource *GitSource `json:"gitSource,omitempty"`

	// Compute Resources required by this component
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// The number of replicas to deploy the component with
	Replicas int `json:"replicas,omitempty"`

	// The port to expose the component over. Referenced in generated service.yaml and route.yaml
	TargetPort int `json:"targetPort,omitempty"`

	// The route host name to expose the component with. Referenced in generated route.yaml
	Route string `json:"route,omitempty"`

	// The name of the route to be generated. If empty, the Component name will be used. Referenced in route.yaml
	// Should be under 30 characters, if over, it will be trimmed to ensure compatibility with generated hostnames
	RouteName string `json:"routeName,omitempty"`

	// An array of environment variables to add to the component.  BaseEnvVar describes environment variables to use for the component
	BaseEnvVar []corev1.EnvVar `json:"env,omitempty"`

	// ExtraEnvsForOverlays is an array of standard environment variables in addition to the component base EnvVars.
	// These will ONLY be added to the deployment patches overlays deployment.yaml whereas the base env vars are added
	// to the base.
	OverlayEnvVar []corev1.EnvVar `json:"overlayEnvVar"`

	// The container image to build or create the component from
	ContainerImage string `json:"containerImage,omitempty"`

	// RevisionHistoryLimit specifies the number of allowed revisions for generated deployments
	// If unset, RevisionHistorylimit in the deployment spec(s) will not be set
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// KubernetesResources to be used instead of generating the Kubernetes resources from a component
	KubernetesResources KubernetesResources `json:"kuberntesResources,omitempty"`

	// IsKubernetesCluster tells us whether it is a Kubernetes or an OpenShift cluster
	// Default is false, hence it is an OpenShift cluster
	IsKubernetesCluster bool `json:"isKubernetesCluster,omitempty"`
}
