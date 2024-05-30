package gitops

import (
	"context"
	"fmt"

	"github.com/konflux-ci/e2e-tests/pkg/logs"
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
		return nil, fmt.Errorf("error occurred while trying to list deploymentTargets in %s namespace: %w", namespace, err)
	}

	return deploymentTargetList, nil
}

// StoreDeploymentTarget stores a given DeploymentTarget as an artifact.
func (g *GitopsController) StoreDeploymentTarget(deploymentTarget *appservice.DeploymentTarget) error {
	return logs.StoreResourceYaml(deploymentTarget, "deploymentTarget-"+deploymentTarget.Name)
}

// StoreAllDeploymentTargets stores all DeploymentTargets in a given namespace.
func (g *GitopsController) StoreAllDeploymentTargets(namespace string) error {
	deploymentTargetList, err := g.GetDeploymentTargetsList(namespace)
	if err != nil {
		return err
	}

	for _, deploymentTarget := range deploymentTargetList.Items {
		if err := g.StoreDeploymentTarget(&deploymentTarget); err != nil {
			return err
		}
	}
	return nil
}
