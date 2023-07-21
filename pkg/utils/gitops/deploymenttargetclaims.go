package gitops

import (
	"context"
	"fmt"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
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
