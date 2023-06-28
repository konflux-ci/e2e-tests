package release

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Contains all methods related with pipeline objects CRUD operations.
type PipelinesInterface interface {
	CreateReleasePipelineRoleBindingForServiceAccount(namespace string, serviceAccount *corev1.ServiceAccount) (*rbac.RoleBinding, error)

	GetPipelineRunInNamespace(namespace, releaseName, releaseNamespace string) (*v1beta1.PipelineRun, error)
}

// CreateReleasePipelineRoleBindingForServiceAccount creates a RoleBinding for the passed serviceAccount to enable
// retrieving the necessary CRs from the passed namespace.
func (r *releaseFactory) CreateReleasePipelineRoleBindingForServiceAccount(namespace string, serviceAccount *corev1.ServiceAccount) (*rbac.RoleBinding, error) {
	roleBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "release-service-pipeline-rolebinding-",
			Namespace:    namespace,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     "release-pipeline-resource-role",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	err := r.KubeRest().Create(context.TODO(), roleBinding)
	if err != nil {
		return nil, err
	}
	return roleBinding, nil
}

// GetPipelineRunInNamespace returns the Release PipelineRun referencing the given release.
func (r *releaseFactory) GetPipelineRunInNamespace(namespace, releaseName, releaseNamespace string) (*v1beta1.PipelineRun, error) {
	pipelineRuns := &v1beta1.PipelineRunList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			"release.appstudio.openshift.io/name":      releaseName,
			"release.appstudio.openshift.io/namespace": releaseNamespace,
		},
		client.InNamespace(namespace),
	}

	err := r.KubeRest().List(context.TODO(), pipelineRuns, opts...)

	if err == nil && len(pipelineRuns.Items) > 0 {
		return &pipelineRuns.Items[0], nil
	}

	return nil, fmt.Errorf("couldn't find PipelineRun in managed namespace '%s' for a release '%s' in '%s' namespace", namespace, releaseName, releaseNamespace)
}
