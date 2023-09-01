package gitops

import (
	"context"
	"fmt"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/logs"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Returns a list of a deploymenttargetclaims from a specific namespace in the kubernetes cluster
func (g *GitopsController) GetDeploymentTargetClaimsList(namespace string) (*appservice.DeploymentTargetClaimList, error) {
	deploymentTargetClaimList := &appservice.DeploymentTargetClaimList{}

	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := g.KubeRest().List(context.Background(), deploymentTargetClaimList, opts...)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error occurred while trying to list DeploymentTargetClaim in %s namespace: %v", namespace, err)
	}

	return deploymentTargetClaimList, nil
}

// StoreDeploymentTargetClaim stores a given DeploymentTargetClaim as an artifact.
func (g *GitopsController) StoreDeploymentTargetClaim(deploymentTargetClaim *appservice.DeploymentTargetClaim) error {
	return logs.StoreResourceYaml(deploymentTargetClaim, "deploymentTargetClaim-"+deploymentTargetClaim.Name)
}

// StoreAllDeploymentTargetClaims stores all DeploymentTargetClaims in a given namespace.
func (g *GitopsController) StoreAllDeploymentTargetClaims(namespace string) error {
	deploymentTargetClaimList, err := g.GetDeploymentTargetClaimsList(namespace)
	if err != nil {
		return err
	}

	for _, deploymentTargetClaim := range deploymentTargetClaimList.Items {
		if err := g.StoreDeploymentTargetClaim(&deploymentTargetClaim); err != nil {
			return err
		}
	}
	return nil
}
