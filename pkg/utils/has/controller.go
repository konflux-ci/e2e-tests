package has

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	"github.com/redhat-appstudio/e2e-tests/pkg/client"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*client.K8sClient
	Github *github.API
}

func NewSuiteController() (*SuiteController, error) {
	client, err := client.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client-go %v", err)
	}

	// Check if a github organization env var is set, if not use by default the redhat-appstudio-qe org. See: https://github.com/redhat-appstudio-qe
	gh := github.NewGitubClient(utils.GetEnv(framework.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))
	return &SuiteController{
		client,
		gh,
	}, nil
}

// GetHasApplicationStatus return the status from the Application Custom Resource object
func (h *SuiteController) GetHasApplication(name, namespace string) (*appservice.Application, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	application := appservice.Application{
		Spec: appservice.ApplicationSpec{},
	}
	err := h.KubeRest().Get(context.TODO(), namespacedName, &application)
	if err != nil {
		return nil, err
	}
	return &application, nil
}

// CreateHasApplication create an application Custom Resource object
func (h *SuiteController) CreateHasApplication(name, namespace string) (*appservice.Application, error) {
	application := appservice.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appservice.ApplicationSpec{
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
	application := appservice.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), &application)
}

// DeleteHasComponent delete an has component from a given name and namespace
func (h *SuiteController) DeleteHasComponent(name string, namespace string) error {
	component := appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), &component)
}

// CreateComponent create an has component from a given name, namespace, application, devfile and a container image
func (h *SuiteController) CreateComponent(applicationName string, componentName string, namespace string, sourceDevfile string, containerImage string) (*appservice.Component, error) {
	component := &appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
		Spec: appservice.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
			Source: appservice.ComponentSource{
				appservice.ComponentSourceUnion{
					GitSource: &appservice.GitSource{
						URL: sourceDevfile,
					},
				}},
			Replicas:   1,
			TargetPort: 8081,
			Route:      "",
			Build: appservice.Build{
				ContainerImage: containerImage,
			},
		},
	}
	err := h.KubeRest().Create(context.TODO(), component)
	if err != nil {
		return nil, err
	}
	return component, nil
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
