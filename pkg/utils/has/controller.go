package has

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	v1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/client"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*client.K8sClient
}

func NewSuiteController() (*SuiteController, error) {
	client, err := client.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client-go %v", err)
	}
	return &SuiteController{
		client,
	}, nil
}

// GetHasApplicationStatus return the status from the Application Custom Resource object
func (h *SuiteController) GetHasApplication(name, namespace string) (*v1alpha1.Application, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	application := v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{},
	}
	err := h.KubeRest().Get(context.TODO(), namespacedName, &application)
	if err != nil {
		return nil, err
	}
	return &application, nil
}

// CreateHasApplication create an application Custom Resource object
func (h *SuiteController) CreateHasApplication(name, namespace string) (*v1alpha1.Application, error) {
	application := v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSpec{
			DisplayName: name,
		},
	}
	err := h.KubeRest().Create(context.TODO(), &application)
	if err != nil {
		return nil, err
	}
	return &application, nil
}

// DeleteHasApplication delete an has application from a given name and namespace
func (h *SuiteController) DeleteHasApplication(name, namespace string) error {
	application := v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), &application)
}

// DeleteHasComponent delete an has component from a given name and namespace
func (h *SuiteController) DeleteHasComponent(name string, namespace string) error {
	component := v1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), &component)
}

// CreateComponent create an has component from a given name, namespace, application, devfile and a container image
func (h *SuiteController) CreateComponent(applicationName string, componentName string, namespace string, sourceDevfile string, containerImage string) (*v1alpha1.Component, error) {
	component := v1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
		Spec: v1alpha1.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
			Source: v1alpha1.ComponentSource{
				v1alpha1.ComponentSourceUnion{
					GitSource:   &v1alpha1.GitSource{URL: sourceDevfile},
					ImageSource: &v1alpha1.ImageSource{},
				}},
			Replicas:   1,
			TargetPort: 8081,
			Route:      "",
			Build: v1alpha1.Build{
				ContainerImage: containerImage,
			},
		},
	}
	err := h.KubeRest().Create(context.TODO(), &component)
	if err != nil {
		return nil, err
	}
	return &component, nil
}

// GetComponentPipeline returns the pipeline for a given component labels
func (h *SuiteController) GetComponentPipeline(componentName string, applicationName string) (v1beta1.PipelineRun, error) {
	pipelineRunLabels := map[string]string{"build.appstudio.openshift.io/component": componentName, "build.appstudio.openshift.io/application": applicationName}
	list := &v1beta1.PipelineRunList{}
	err := h.KubeRest().List(context.TODO(), list, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(pipelineRunLabels)})

	if len(list.Items) > 0 {
		return list.Items[0], nil
	} else if len(list.Items) == 0 {
		return v1beta1.PipelineRun{}, fmt.Errorf("no pipelinerun found for component %s", componentName)
	}
	return v1beta1.PipelineRun{}, err
}

// GetComponentRoute returns the route for a given component name
func (h *SuiteController) GetComponentRoute(componentName string, componentNamespace string) (*routev1.Route, error) {
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("el%s", componentName),
		Namespace: componentNamespace,
	}

	route := &routev1.Route{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, route)
	if err != nil {
		return &routev1.Route{}, err
	}
	return route, nil
}

// GetComponentDeployment returns the deployment for a given component name
func (h *SuiteController) GetComponentDeployment(componentName string, componentNamespace string) (*appsv1.Deployment, error) {
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("el-%s", componentName),
		Namespace: componentNamespace,
	}

	deployment := &appsv1.Deployment{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		return &appsv1.Deployment{}, err
	}
	return deployment, nil
}

// GetComponentService returns the service for a given component name
func (h *SuiteController) GetComponentService(componentName string, componentNamespace string) (*corev1.Service, error) {
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("el-%s", componentName),
		Namespace: componentNamespace,
	}

	service := &corev1.Service{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, service)
	if err != nil {
		return &corev1.Service{}, err
	}
	return service, nil
}
