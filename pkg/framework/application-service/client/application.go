package client

import (
	"context"

	app "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applicationservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

//GetHasApplicationStatus return the status from the Application Custom Resource object
func (k *K8sClient) GetHasApplicationStatus(name, namespace string) (*applicationservice.ApplicationStatus, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	application := applicationservice.Application{}
	err := k.crClient.Get(context.TODO(), namespacedName, &application)
	if err != nil {
		return nil, err
	}
	return &application.Status, nil
}

//CreateHasApplication create an application Custom Resource object
func (k *K8sClient) CreateHasApplication(name, namespace string) (*applicationservice.Application, error) {
	application := applicationservice.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: applicationservice.ApplicationSpec{},
	}
	err := k.crClient.Create(context.TODO(), &application)
	if err != nil {
		return nil, err
	}
	return &application, nil
}

func (k *K8sClient) GetArgoApplicationStatus(name string, namespace string) (*app.ApplicationStatus, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	application := &app.Application{}

	if err := k.crClient.Get(context.TODO(), namespacedName, application); err != nil {
		return nil, err
	}
	return &application.Status, nil
}
