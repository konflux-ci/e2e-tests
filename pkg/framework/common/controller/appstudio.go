package controller

import (
	"context"

	app "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func (h *CommonSuiteController) GetAppStudioComponentStatus(name string, namespace string) (*app.ApplicationStatus, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	application := &app.Application{}

	if err := h.KubeRest().Get(context.TODO(), namespacedName, application); err != nil {
		return nil, err
	}
	return &application.Status, nil
}
