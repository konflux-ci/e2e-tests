package has

import (
	"context"
	"fmt"

	v1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redhat-appstudio/e2e-tests/pkg/client"
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

func (h *SuiteController) CreateComponent(applicationName string, componentName string, namespace string, sourceDevfile string) (*v1alpha1.Component, error) {
	component := v1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
		Spec: v1alpha1.ComponentSpec{
			Application:   applicationName,
			ComponentName: componentName,
			Build: v1alpha1.Build{
				ContainerImage: "quay.io/flacatus/quarkus:1.0.0",
			},
			Source: v1alpha1.ComponentSource{
				v1alpha1.ComponentSourceUnion{
					GitSource: &v1alpha1.GitSource{
						URL: sourceDevfile,
					},
				},
			},
		},
	}
	err := h.KubeRest().Create(context.TODO(), &component)
	if err != nil {
		return nil, err
	}
	return &component, nil
}

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
