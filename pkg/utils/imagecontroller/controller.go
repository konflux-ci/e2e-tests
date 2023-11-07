package imagecontroller

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
)

type ImageController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*ImageController, error) {
	return &ImageController{
		kube,
	}, nil
}
