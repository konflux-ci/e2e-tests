package gitops

import (
	"context"
	"fmt"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// CreateDeploymentTargetClass creates a DeploymentTargetClass with a "appstudio.redhat.com/devsandbox" provisioner
func (g *GitopsController) CreateDeploymentTargetClass() (*appservice.DeploymentTargetClass, error) {
	dtcls := &appservice.DeploymentTargetClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-sandbox-class",
			Annotations: map[string]string{},
		},
		Spec: appservice.DeploymentTargetClassSpec{
			Provisioner:   appservice.Provisioner_Devsandbox,
			ReclaimPolicy: "Retain",
		},
	}

	err := g.KubeRest().Create(context.TODO(), dtcls)
	if err != nil {
		return nil, fmt.Errorf("error occurred when creating the DeploymentTargetClass: %+v", err)
	}
	return dtcls, nil
}

func (g *GitopsController) DeleteDeploymentTargetClass() (error) {
	dtcls := appservice.DeploymentTargetClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sandbox-class",
		},
	}
	if err := g.KubeRest().Delete(context.TODO(), &dtcls); err != nil {
		return fmt.Errorf("error occurred when deleting the DeploymentTargetClass: %+v", err)
	}

	return nil
}
