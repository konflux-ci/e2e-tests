package common

import (
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/redhat-appstudio/e2e-tests/pkg/client"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/kubernetes/pkg/client/conditions"
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
		return nil, fmt.Errorf("error creating client-go %v", err)
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

// Check if a secret exists, return secret and error
func (s *SuiteController) VerifySecretExists(ns string, name string) (*corev1.Secret, error) {
	secret, err := s.KubeInterface().CoreV1().Secrets(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (s *SuiteController) GetPod(namespace, podName string) (*corev1.Pod, error) {
	pod, err := s.KubeInterface().CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func IsPodRunning(pod *corev1.Pod, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		fmt.Printf(".")
		switch pod.Status.Phase {
		case corev1.PodRunning:
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, conditions.ErrPodCompleted
		}
		return false, nil
	}
}

func IsPodSuccessful(pod *corev1.Pod, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		fmt.Printf(".")
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			return true, nil
		case corev1.PodFailed:
			return false, conditions.ErrPodCompleted
		}
		return false, nil
	}
}

func (s *SuiteController) ListPods(namespace, labelKey, labelValue string) (*corev1.PodList, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         100,
	}
	podList, err := s.KubeInterface().CoreV1().Pods(namespace).List(context.TODO(), listOptions)

	if err != nil {
		return nil, err
	}
	return podList, nil
}

func (s *SuiteController) waitForPod(cond wait.ConditionFunc, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, cond)
}

func (s *SuiteController) WaitForPodSelector(
	fn func(pod *corev1.Pod, namespace string) wait.ConditionFunc, namespace, labelKey string, labelValue string,
	timeout int) error {
	podList, err := s.ListPods(namespace, labelKey, labelValue)
	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods in %s with label key %s and label value %s", namespace, labelKey, labelValue)
	}

	for _, pod := range podList.Items {
		if err := s.waitForPod(fn(&pod, namespace), time.Duration(timeout)*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func (s *SuiteController) GetRole(roleName, namespace string) (*rbacv1.Role, error) {
	role, err := s.KubeInterface().RbacV1().Roles(namespace).Get(context.TODO(), roleName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return role, nil
}

func (s *SuiteController) GetRoleBinding(rolebindingName, namespace string) (*rbacv1.RoleBinding, error) {
	roleBinding, err := s.KubeInterface().RbacV1().RoleBindings(namespace).Get(context.TODO(), rolebindingName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return roleBinding, nil
}
