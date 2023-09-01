package gitops

import (
	"context"
	"fmt"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	managedgitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend/apis/managed-gitops/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Remove all gitopsdeployments from a given namespace. Useful when creating a lot of resources and want to remove all of them
func (g *GitopsController) DeleteAllGitOpsDeploymentsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := g.KubeRest().DeleteAllOf(context.TODO(), &managedgitopsv1alpha1.GitOpsDeployment{}, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error when deleting gitopsdeployments in %s namespace: %+v", namespace, err)
	}

	return utils.WaitUntil(func() (done bool, err error) {
		gdList, err := g.ListAllGitOpsDeployments(namespace)
		if err != nil {
			return false, nil
		}
		return len(gdList.Items) == 0, nil
	}, timeout)
}
