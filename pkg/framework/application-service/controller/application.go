package controller

import (
	"context"

	applicationservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

//GetHasApplicationStatus return the status from the Application Custom Resource object
func (h *HASSuiteController) GetHasApplicationStatus(name, namespace string) (*applicationservice.ApplicationStatus, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	application := applicationservice.Application{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, &application)
	if err != nil {
		return nil, err
	}
	return &application.Status, nil
}

//CreateHasApplication create an application Custom Resource object
func (h *HASSuiteController) CreateHasApplication(name, namespace string) (*applicationservice.Application, error) {
	application := applicationservice.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: applicationservice.ApplicationSpec{},
	}
	err := h.KubeRest().Create(context.TODO(), &application)
	if err != nil {
		return nil, err
	}
	return &application, nil
}

// DeleteHasApplication delete an has application from a given name and namespace
func (h *HASSuiteController) DeleteHasApplication(name, namespace string) error {
	application := applicationservice.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), &application)
}
