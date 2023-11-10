package integration

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/clients/kubernetes"
)

type IntegrationController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*IntegrationController, error) {
	return &IntegrationController{
		kube,
	}, nil
}
