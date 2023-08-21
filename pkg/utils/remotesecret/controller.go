package remotesecret

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
)

type RemoteSecretController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*RemoteSecretController, error) {
	// Initialize a new SPI controller with just the kube client
	return &RemoteSecretController{
		kube,
	}, nil
}
