package gitops

import (
	"context"
	"fmt"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

// HaveAvailableDeploymentTargetClassExist attempts to find a DeploymentTargetClass with appstudioApi.Provisioner_Devsandbox as provisioner.
// reurn nil if not found
func (g *GitopsController) HaveAvailableDeploymentTargetClassExist() (*appservice.DeploymentTargetClass, error) {
	deploymentTargetClassList := &appservice.DeploymentTargetClassList{}
	err := g.KubeRest().List(context.TODO(), deploymentTargetClassList)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error occurred while trying to list all the available DeploymentTargetClass: %v", err)
	}

	for _, dtcls := range deploymentTargetClassList.Items {
		if dtcls.Spec.Provisioner == appservice.Provisioner_Devsandbox {
			return &dtcls, nil
		}
	}

	return nil, nil
}
