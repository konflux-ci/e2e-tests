package common

import (
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

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

// GetClusterTask return a clustertask object from cluster and if don't exist returns an error
func (s *SuiteController) GetClusterTask(name string, namespace string) (*v1beta1.ClusterTask, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	clusterTask := &v1beta1.ClusterTask{}
	if err := s.KubeRest().Get(context.TODO(), namespacedName, clusterTask); err != nil {
		return nil, err
	}
	return clusterTask, nil
}

// ListClusterTask return a list of ClusterTasks with a specific label selectors
func (s *SuiteController) CheckIfClusterTaskExists(name string) bool {
	clusterTasks := &v1beta1.ClusterTaskList{}
	if err := s.KubeRest().List(context.TODO(), clusterTasks, &rclient.ListOptions{}); err != nil {
		return false
	}
	for _, ctasks := range clusterTasks.Items {
		if ctasks.Name == name {
			return true
		}
	}
	return false
}

func (s *SuiteController) WaitForPodToBeReady(labelKey string, labelValue string, componentNamespace string) error {
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}
		listOptions := metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
			Limit:         100,
		}
		pods, err := s.KubeInterface().CoreV1().Pods(componentNamespace).List(context.TODO(), listOptions)
		if err != nil {
			return false, nil
		}
		if len(pods.Items) > 0 {
			return true, nil
		}
		return false, nil
	})
}
