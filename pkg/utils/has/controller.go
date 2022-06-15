package has

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*kubeCl.K8sClient
	Github *github.API
}

func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {
	// Check if a github organization env var is set, if not use by default the redhat-appstudio-qe org. See: https://github.com/redhat-appstudio-qe
	gh := github.NewGitubClient(utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))
	return &SuiteController{
		kube,
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
func (h *SuiteController) CreateComponent(applicationName, componentName, namespace, gitSourceURL, containerImageSource, outputContainerImage, secret string) (*appservice.Component, error) {
	var containerImage string
	if outputContainerImage != "" {
		containerImage = outputContainerImage
	} else {
		containerImage = containerImageSource
	}
	component := &appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
		Spec: appservice.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
			Source: appservice.ComponentSource{
				ComponentSourceUnion: appservice.ComponentSourceUnion{
					GitSource: &appservice.GitSource{
						URL: gitSourceURL,
					},
				},
			},
			Secret:         secret,
			ContainerImage: containerImage,
			Replicas:       1,
			TargetPort:     8081,
			Route:          "",
		},
	}
	err := h.KubeRest().Create(context.TODO(), component)
	if err != nil {
		return nil, err
	}
	return component, nil
}

// GetComponentPipeline returns the pipeline for a given component labels
func (h *SuiteController) GetComponentPipeline(componentName string, applicationName string, namespace string) (v1beta1.PipelineRun, error) {
	pipelineRunLabels := map[string]string{"build.appstudio.openshift.io/component": componentName, "build.appstudio.openshift.io/application": applicationName}
	list := &v1beta1.PipelineRunList{}
	err := h.KubeRest().List(context.TODO(), list, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(pipelineRunLabels), Namespace: namespace})

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

// CreateTestNamespace creates a namespace where Application and Component CR will be created
func (h *SuiteController) CreateTestNamespace(name string) (*corev1.Namespace, error) {

	// Check if the E2E test namespace already exists
	ns, err := h.KubeInterface().CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// Create the E2E test namespace if it doesn't exist
			nsTemplate := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{constants.ArgoCDLabelKey: constants.ArgoCDLabelValue},
				}}
			ns, err = h.KubeInterface().CoreV1().Namespaces().Create(context.TODO(), &nsTemplate, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("error when creating %s namespace: %v", name, err)
			}
		} else {
			return nil, fmt.Errorf("error when getting the '%s' namespace: %v", name, err)
		}
	} else {
		// Check whether the test namespace contains correct label
		if val, ok := ns.Labels[constants.ArgoCDLabelKey]; ok && val == constants.ArgoCDLabelValue {
			return ns, nil
		}
		// Update test namespace labels in case they are missing argoCD label
		ns.Labels[constants.ArgoCDLabelKey] = constants.ArgoCDLabelValue
		ns, err = h.KubeInterface().CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("error when updating labels in '%s' namespace: %v", name, err)
		}
	}

	return ns, nil
}

// DeleteTestNamespace deletes a namespace where Application and Component
func (h *SuiteController) DeleteTestNamespace(name string) (*corev1.Namespace, error) {

	// Check if the E2E test namespace already exists
	ns, err := h.KubeInterface().CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		if k8sErrors.IsNotFound((err)) {
			klog.Info("namespace does not exist in cluster!: ", ns.Name)
			return nil, fmt.Errorf("namespace '%s' is not on cluster ", ns.Name)
			// return h.KubeInterface().CoreV1().Namespaces().Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
		}
		// klog.Error("error when trying to get namespace '%s' namespace: %v", name, err)
	} else {
		klog.Info("namespace is deleted!: ", ns.Name)
		return nil, h.KubeInterface().CoreV1().Namespaces().Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
	}
	return nil, fmt.Errorf("error when deleting'%s' namespace: %v", name, err)
}

