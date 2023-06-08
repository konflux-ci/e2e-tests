package common

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// GetAppDeploymentByName returns the deployment for a given component name
func (h *SuiteController) GetDeployment(deploymentName string, namespace string) (*appsv1.Deployment, error) {
	namespacedName := types.NamespacedName{
		Name:      deploymentName,
		Namespace: namespace,
	}

	deployment := &appsv1.Deployment{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		return &appsv1.Deployment{}, err
	}
	return deployment, nil
}

// Checks and waits for a kubernetes deployment object to be completed or not
func (h *SuiteController) DeploymentIsCompleted(deploymentName, namespace string, readyReplicas int32) wait.ConditionFunc {
	return func() (bool, error) {
		namespacedName := types.NamespacedName{
			Name:      deploymentName,
			Namespace: namespace,
		}

		deployment := &appsv1.Deployment{}
		err := h.KubeRest().Get(context.TODO(), namespacedName, deployment)
		if err != nil && !k8sErrors.IsNotFound(err) {
			return false, err
		}
		if deployment.Status.AvailableReplicas == readyReplicas && deployment.Status.UnavailableReplicas == 0 {
			return true, nil
		}
		return false, nil
	}
}

// RolloutRestartDeployment restart a deployment by replicating 'restart rollout' bahviour
func (h *SuiteController) RolloutRestartDeployment(deploymentName string, namespace string) (*appsv1.Deployment, error) {
	namespacedName := types.NamespacedName{
		Name:      deploymentName,
		Namespace: namespace,
	}

	deployment := &appsv1.Deployment{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		return &appsv1.Deployment{}, err
	}

	newDeployment := deployment.DeepCopy()
	ann := newDeployment.ObjectMeta.Annotations
	if ann == nil {
		ann = make(map[string]string)
	}
	ann["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
	var replicas int32 = 0
	newDeployment.Spec.Replicas = &replicas
	newDeployment.SetAnnotations(ann)

	newDep, err := h.KubeInterface().AppsV1().Deployments(namespace).Update(context.TODO(), newDeployment, metav1.UpdateOptions{})
	if err != nil {
		return &appsv1.Deployment{}, err
	}

	return newDep, nil
}
