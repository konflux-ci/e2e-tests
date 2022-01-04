package common

import (
	"context"
	"fmt"

	app "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/redhat-appstudio/e2e-tests/pkg/client"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Create the struct for kubernetes clients
type SuiteController struct {
	*client.K8sClient
}

// Create controller for Application/Component crud operations
func NewSuiteController() (*SuiteController, error) {
	client, err := client.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("Error creating client-go %v", err)
	}
	return &SuiteController{
		client,
	}, nil
}

func (h *SuiteController) GetAppStudioComponentStatus(name string, namespace string) (*app.ApplicationStatus, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	application := &app.Application{}

	if err := h.KubeRest().Get(context.TODO(), namespacedName, application); err != nil {
		return nil, err
	}
	return &application.Status, nil
}

// GetClusterTask return a clustertask object from cluster and if don't exist returns an error
func (h *SuiteController) GetClusterTask(name string, namespace string) (*v1beta1.ClusterTask, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	clusterTask := &v1beta1.ClusterTask{}

	if err := h.KubeRest().Get(context.TODO(), namespacedName, clusterTask); err != nil {
		return nil, err
	}
	return clusterTask, nil
}

// ListClusterTask return a list of ClusterTasks with a specific label selectors
func (h *SuiteController) ListClusterTask(labelSelector map[string]string) ([]v1beta1.ClusterTask, error) {
	clusterTasks := &v1beta1.ClusterTaskList{}
	if err := h.KubeRest().List(context.TODO(), clusterTasks, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(labelSelector)}); err != nil {
		return nil, err
	}
	return clusterTasks.Items, nil
}
