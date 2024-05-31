package imagecontroller

import (
	kubeCl "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
)

type ImageController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*ImageController, error) {
	return &ImageController{
		kube,
	}, nil
}
