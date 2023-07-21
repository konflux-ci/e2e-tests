package gitops

import (
	"context"
	"fmt"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// List all deploymentTargets in a given namespace from the kubernetes cluster.
func (g *GitopsController) GetDeploymentTargetsList(namespace string) (*appservice.DeploymentTargetList, error) {
	deploymentTargetList := &appservice.DeploymentTargetList{}

	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := g.KubeRest().List(context.Background(), deploymentTargetList, opts...)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error occurred while trying to list deploymentTargets in %s namespace: %v", namespace, err)
	}

	return deploymentTargetList, nil
}
