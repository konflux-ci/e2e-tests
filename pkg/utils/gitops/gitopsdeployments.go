package gitops

import (
	"context"
	"fmt"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	managedgitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend/apis/managed-gitops/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Contains all methods related with Environments CRUD operations.
type GitopsDeploymentsInterface interface {
	// Removes all gitopsdeployments from a given namespace. Useful when creating a lot of resources and want to remove all of them
	DeleteAllGitOpsDeploymentsInASpecificNamespace(namespace string, timeout time.Duration) error
}

// Remove all gitopsdeployments from a given namespace. Useful when creating a lot of resources and want to remove all of them
func (h *gitopsFactory) DeleteAllGitOpsDeploymentsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := h.KubeRest().DeleteAllOf(context.TODO(), &managedgitopsv1alpha1.GitOpsDeployment{}, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error when deleting gitopsdeployments in %s namespace: %+v", namespace, err)
	}

	gdList := &managedgitopsv1alpha1.GitOpsDeploymentList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err = h.KubeRest().List(context.Background(), gdList, client.InNamespace(namespace)); err != nil {
			return false, nil
		}
		return len(gdList.Items) == 0, nil
	}, timeout)
}
