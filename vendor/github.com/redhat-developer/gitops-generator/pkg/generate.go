//
// Copyright 2021-2023 Red Hat, Inc.
//
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

package gitops

import (
	"fmt"
	"path/filepath"

	gitopsv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/redhat-developer/gitops-generator/pkg/resources"
	"github.com/redhat-developer/gitops-generator/pkg/util"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	yaml "github.com/redhat-developer/gitops-generator/pkg/yaml"
)

const (
	kustomizeFileName       = "kustomization.yaml"
	deploymentFileName      = "deployment.yaml"
	deploymentPatchFileName = "deployment-patch.yaml"
	ingressFileName         = "ingress.yaml"
	routeFileName           = "route.yaml"
	serviceFileName         = "service.yaml"
	otherFileName           = "other_resources.yaml"
)

var CreatedBy = "application-service"

// Generate takes in a given Component CR and
// spits out a deployment, service, and route file to disk
func Generate(fs afero.Afero, gitOpsFolder string, outputFolder string, options gitopsv1alpha1.GeneratorOptions) error {

	var deployment *appsv1.Deployment
	if len(options.KubernetesResources.Deployments) == 0 {
		deployment = generateDeployment(options)
	} else if len(options.KubernetesResources.Deployments) > 0 {
		deployment, options.KubernetesResources.Deployments = &options.KubernetesResources.Deployments[0], options.KubernetesResources.Deployments[1:]
		var otherDeployments []interface{}
		for _, deployment := range options.KubernetesResources.Deployments {
			otherDeployments = append(otherDeployments, deployment)
		}

		options.KubernetesResources.Others = append(options.KubernetesResources.Others, otherDeployments...)
	}

	k := resources.Kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
	}
	k.AddResources(deploymentFileName)
	resources := map[string]interface{}{
		deploymentFileName: deployment,
	}

	var service *corev1.Service

	if len(options.KubernetesResources.Services) == 0 && options.TargetPort != 0 {
		// If service was not provided, generate a service only if target port was provided
		// If service was not provided and target port is 0, skip generation
		service = generateService(options)
	} else if len(options.KubernetesResources.Services) > 0 {
		// If a service was provided, get the first and append the rest to others
		service, options.KubernetesResources.Services = &options.KubernetesResources.Services[0], options.KubernetesResources.Services[1:]

		var otherServices []interface{}
		for _, service := range options.KubernetesResources.Services {
			otherServices = append(otherServices, service)
		}

		options.KubernetesResources.Others = append(options.KubernetesResources.Others, otherServices...)
	}

	if len(options.KubernetesResources.Routes) > 0 {
		// If Routes were provided, append Routes from second onwards to others
		// The first Route is created in the overlays resource

		var otherRoutes []interface{}
		for _, route := range options.KubernetesResources.Routes[1:] {
			otherRoutes = append(otherRoutes, route)
		}

		options.KubernetesResources.Others = append(options.KubernetesResources.Others, otherRoutes...)
	}

	if len(options.KubernetesResources.Ingresses) > 0 {
		// If Ingresses were provided, append Ingresses from second onwards to others
		// The first Ingress is created in the overlays resource

		var otherIngresses []interface{}
		for _, ingress := range options.KubernetesResources.Ingresses[1:] {
			otherIngresses = append(otherIngresses, ingress)
		}

		options.KubernetesResources.Others = append(options.KubernetesResources.Others, otherIngresses...)
	}

	if service != nil {
		k.AddResources(serviceFileName)
		resources[serviceFileName] = service
	}

	if len(options.KubernetesResources.Others) > 0 {
		k.AddResources(otherFileName)
		resources[otherFileName] = options.KubernetesResources.Others
	}

	resources[kustomizeFileName] = k

	_, err := yaml.WriteResources(fs, outputFolder, resources)
	if err != nil {
		return err
	}

	// Re-generate the parent kustomize file and return
	return nil
}

// GenerateOverlays generates the overlays director in an existing GitOps structure
func GenerateOverlays(fs afero.Afero, gitOpsFolder string, outputFolder string, options gitopsv1alpha1.GeneratorOptions, imageName, namespace string, componentGeneratedResources map[string][]string) error {
	kustomizeFileExist, err := fs.Exists(filepath.Join(outputFolder, kustomizeFileName))
	if err != nil {
		return err
	}
	// if kustomizeFile already exist, read in the content
	var originalKustomizeFileContent resources.Kustomization
	if kustomizeFileExist {
		err = yaml.UnMarshalItemFromFile(fs, filepath.Join(outputFolder, kustomizeFileName), &originalKustomizeFileContent)
		if err != nil {
			return fmt.Errorf("failed to unmarshal items from %q: %v", filepath.Join(outputFolder, kustomizeFileName), err)
		}
		err = fs.Remove(filepath.Join(outputFolder, kustomizeFileName))
		if err != nil {
			return fmt.Errorf("failed to delete %s file in folder %q: %s", kustomizeFileName, outputFolder, err)
		}
	}

	k := resources.Kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
	}

	if componentGeneratedResources == nil {
		componentGeneratedResources = make(map[string][]string)
	}

	var originalDeploymentContent appsv1.Deployment
	baseDeploymentFilePath := filepath.Join(outputFolder, "../../base/", deploymentFileName)
	DeploymentFileExist, err := fs.Exists(baseDeploymentFilePath)
	containerName := "container-image"
	if err != nil {
		return err
	}
	if DeploymentFileExist {
		err = yaml.UnMarshalItemFromFile(fs, baseDeploymentFilePath, &originalDeploymentContent)
		if err != nil {
			return fmt.Errorf("failed to unmarshal items from %q: %v", baseDeploymentFilePath, err)
		}

		if len(originalDeploymentContent.Spec.Template.Spec.Containers) > 0 {
			containerName = originalDeploymentContent.Spec.Template.Spec.Containers[0].Name
		}
	}

	deploymentPatch := generateDeploymentPatch(options, imageName, containerName, namespace)

	resources := map[string]interface{}{
		deploymentPatchFileName: deploymentPatch,
	}

	k.AddResources("../../base")
	k.AddPatches(deploymentPatchFileName)
	componentGeneratedResources[options.Name] = append(componentGeneratedResources[options.Name], deploymentPatchFileName)

	var route *routev1.Route
	var ingress *networkingv1.Ingress

	// Create an ingress if its a Kubernetes cluster, route if its an OpenShift cluster
	if options.IsKubernetesCluster {
		if len(options.KubernetesResources.Ingresses) == 0 && options.TargetPort != 0 {
			// If no Ingresses were provided and TargetPort is not 0, generate the Ingress
			ingress = generateIngress(options)
		} else if len(options.KubernetesResources.Ingresses) > 0 {
			// If Ingresses were provided, get the first Ingress
			ingress = &options.KubernetesResources.Ingresses[0]
		}
	} else {
		if len(options.KubernetesResources.Routes) == 0 && options.TargetPort != 0 {
			// If no Routes were provided and TargetPort is not 0, generate the Route
			route = generateRoute(options)
		} else if len(options.KubernetesResources.Routes) > 0 {
			// If Routes were provided, get the first Route
			route = &options.KubernetesResources.Routes[0]
		}
	}

	if ingress != nil {
		k.AddResources(ingressFileName)
		resources[ingressFileName] = ingress
	}

	if route != nil {
		k.AddResources(routeFileName)
		resources[routeFileName] = route
	}

	// add back custom kustomization patches
	k.CompareDifferenceAndAddCustomPatches(originalKustomizeFileContent.Patches, componentGeneratedResources[options.Name])

	resources[kustomizeFileName] = k

	_, err = yaml.WriteResources(fs, outputFolder, resources)
	return err
}

func UpdateExistingKustomize(fs afero.Afero, outputFolder string) error {
	k := resources.Kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
	}

	resources := map[string]interface{}{}
	fInfo, err := fs.ReadDir(outputFolder)
	if err != nil {
		return err
	}
	for _, file := range fInfo {
		if file.Name() != kustomizeFileName && !file.IsDir() {
			k.AddResources(file.Name())
		}
		if file.IsDir() {
			k.AddResources(file.Name() + "/")
		}
	}

	resources[kustomizeFileName] = k

	_, err = yaml.WriteResources(fs, outputFolder, resources)
	return err
}

func generateDeployment(component gitopsv1alpha1.GeneratorOptions) *appsv1.Deployment {
	var revHistoryLimit *int32
	if component.RevisionHistoryLimit != nil {
		revHistoryLimit = component.RevisionHistoryLimit
	}

	var containerImage string
	if component.ContainerImage != "" {
		containerImage = component.ContainerImage
	}
	replicas := getReplicas(component)
	k8sLabels := generateK8sLabels(component)
	matchLabels := getMatchLabel(component)
	deployment := appsv1.Deployment{
		TypeMeta: v1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
			Labels:    k8sLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &v1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: matchLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "container-image",
							Image:           containerImage,
							ImagePullPolicy: corev1.PullAlways,
							Env:             component.BaseEnvVar,
							Resources:       component.Resources,
						},
					},
				},
			},
		},
	}

	// If a container image source was set in the component *and* a given secret was set for it,
	// Set the secret as an image pull secret, in case the component references a private image component
	if component.ContainerImage != "" && component.Secret != "" {
		deployment.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: component.Secret,
			},
		}
	}

	// Set fields that may have been optionally configured by the component CR
	if component.TargetPort != 0 {
		deployment.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{
				ContainerPort: int32(component.TargetPort),
			},
		}
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(component.TargetPort),
				},
			},
		}
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Port: intstr.FromInt(component.TargetPort),
					Path: "/",
				},
			},
		}
	}

	if revHistoryLimit != nil {
		deployment.Spec.RevisionHistoryLimit = revHistoryLimit
	}

	return &deployment
}

func generateDeploymentPatch(options gitopsv1alpha1.GeneratorOptions, imageName, containerName, namespace string) *appsv1.Deployment {

	deployment := appsv1.Deployment{
		TypeMeta: v1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      options.Name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &v1.LabelSelector{},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: imageName,
						},
					},
				},
			},
		},
	}

	for _, env := range options.BaseEnvVar {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}

	// only add the environment env configurations, if a deployment/binding env is not present with the same env name
	for _, env := range options.OverlayEnvVar {
		isPresent := false
		for _, deploymentEnv := range deployment.Spec.Template.Spec.Containers[0].Env {
			if deploymentEnv.Name == env.Name {
				isPresent = true
				break
			}
		}

		if !isPresent {
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}
	}

	if options.Replicas > 0 {
		replica := int32(options.Replicas)
		deployment.Spec.Replicas = &replica
	}

	deployment.Spec.Template.Spec.Containers[0].Resources = options.Resources

	return &deployment
}

func generateService(options gitopsv1alpha1.GeneratorOptions) *corev1.Service {
	k8sLabels := generateK8sLabels(options)
	matchLabels := getMatchLabel(options)
	service := corev1.Service{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      options.Name,
			Namespace: options.Namespace,
			Labels:    k8sLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: matchLabels,
			Ports: []corev1.ServicePort{
				{
					Port:       int32(options.TargetPort),
					TargetPort: intstr.FromInt(options.TargetPort),
				},
			},
		},
	}

	return &service
}

func generateIngress(options gitopsv1alpha1.GeneratorOptions) *networkingv1.Ingress {

	ingressName := options.Name
	k8sLabels := generateK8sLabels(options)

	implementationSpecific := networkingv1.PathTypeImplementationSpecific

	ingress := networkingv1.Ingress{
		TypeMeta: v1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ingressName,
			Namespace: options.Namespace,
			Labels:    k8sLabels,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &implementationSpecific,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: options.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(options.TargetPort),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if options.Route != "" && len(ingress.Spec.Rules) > 0 {
		ingress.Spec.Rules[0].Host = options.Route
	}

	return &ingress
}

func generateRoute(options gitopsv1alpha1.GeneratorOptions) *routev1.Route {

	// If a specific Route name was passed in, use it, otherwise use the Component's name
	var routeName string
	if options.RouteName != "" {
		routeName = options.RouteName
	} else {
		routeName = options.Name

	}
	// Trim the generated route name to under 30 characters (plus a few random characters for uniqueness)
	// To ensure issues where the generated hostname (componentName-namespace) is too long
	if len(routeName) >= 30 {
		routeName = routeName[0:25] + util.GetRandomString(4, true)
	}

	k8sLabels := generateK8sLabels(options)
	weight := int32(100)
	route := routev1.Route{
		TypeMeta: v1.TypeMeta{
			Kind:       "Route",
			APIVersion: "route.openshift.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      routeName,
			Namespace: options.Namespace,
			Labels:    k8sLabels,
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(options.TargetPort),
			},
			TLS: &routev1.TLSConfig{
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   routev1.TLSTerminationEdge,
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   options.Name,
				Weight: &weight,
			},
		},
	}

	// If the route field is set in the spec, set it to be the host for the route
	if options.Route != "" {
		route.Spec.Host = options.Route
	}

	return &route
}

// getReplicas returns the number of replicas to be created for the component
// If the field is not set, it returns a default value of 1
// ToDo: Handle as part of a defaulting webhook
func getReplicas(options gitopsv1alpha1.GeneratorOptions) int32 {
	if options.Replicas > 0 {
		return int32(options.Replicas)
	}
	return 1
}

// generateLabels returns a map containing the following common Kubernetes labels:
// app.kubernetes.io/name: "<component-name>"
// app.kubernetes.io/instance: "<component-cr-name>"
// app.kubernetes.io/part-of: "<application-name>"
// app.kubernetes.io/managed-by: "kustomize"
// app.kubernetes.io/created-by: "application-service"
func generateK8sLabels(options gitopsv1alpha1.GeneratorOptions) map[string]string {
	if options.K8sLabels != nil {
		return options.K8sLabels
	}
	return map[string]string{
		"app.kubernetes.io/name":       options.Name,
		"app.kubernetes.io/instance":   options.Name,
		"app.kubernetes.io/part-of":    options.Application,
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/created-by": CreatedBy,
	}
}

// GetMatchLabel returns the label selector that will be used to tie deployments, services, and pods together
// For cleanliness, using just one unique label from the generateK8sLabels function
func getMatchLabel(options gitopsv1alpha1.GeneratorOptions) map[string]string {
	return map[string]string{
		"app.kubernetes.io/instance": options.Name,
	}
}
