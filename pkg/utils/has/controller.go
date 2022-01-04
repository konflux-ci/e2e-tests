package has

import (
	"context"
	"fmt"

	applicationservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/redhat-appstudio/e2e-tests/pkg/client"
)

type SuiteController struct {
	*client.K8sClient
}

func NewSuiteController() (*SuiteController, error) {
	client, err := client.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client-go %v", err)
	}
	return &SuiteController{
		client,
	}, nil
}

//GetHasApplicationStatus return the status from the Application Custom Resource object
func (h *SuiteController) GetHasApplicationStatus(name, namespace string) (*applicationservice.ApplicationStatus, error) {
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
func (h *SuiteController) CreateHasApplication(name, namespace string) (*applicationservice.Application, error) {
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
func (h *SuiteController) DeleteHasApplication(name, namespace string) error {
	application := applicationservice.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), &application)
}
