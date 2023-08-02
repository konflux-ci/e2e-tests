package integration

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
)

type IntegrationController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*IntegrationController, error) {
	return &IntegrationController{
		kube,
	}, nil
}
