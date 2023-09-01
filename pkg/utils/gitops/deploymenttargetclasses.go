package gitops

import (
	"context"
	"fmt"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/logs"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// ListAllDeploymentTargetClasses returns a list of all DeploymentTargetClassList in a given namespace.
func (g *GitopsController) ListAllDeploymentTargetClasses(namespace string) (*appservice.DeploymentTargetClassList, error) {
	deploymentTargetClassList := &appservice.DeploymentTargetClassList{}
	err := g.KubeRest().List(context.Background(), deploymentTargetClassList, &client.ListOptions{Namespace: namespace})

	return deploymentTargetClassList, err
}

// StoreDeploymentTargetClass a stores given DeploymentTargetClass as an artifact.
func (g *GitopsController) StoreDeploymentTargetClass(deploymentTargetClass *appservice.DeploymentTargetClass) error {
	return logs.StoreResourceYaml(deploymentTargetClass, "deploymentTargetClass-"+deploymentTargetClass.Name)
}

// StoreAllDeploymentTargetClasses stores all DeploymentTargetClasses in a given namespace.
func (g *GitopsController) StoreAllDeploymentTargetClasses(namespace string) error {
	deploymentTargetClassList, err := g.ListAllDeploymentTargetClasses(namespace)
	if err != nil {
		return err
	}

	for _, deploymentTargetClass := range deploymentTargetClassList.Items {
		if err := g.StoreDeploymentTargetClass(&deploymentTargetClass); err != nil {
			return err
		}
	}
	return nil
}
