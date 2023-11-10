package jvmbuildservice

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/clients/kubernetes"
)

type JvmbuildserviceController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*JvmbuildserviceController, error) {
	return &JvmbuildserviceController{
		kube,
	}, nil
}
