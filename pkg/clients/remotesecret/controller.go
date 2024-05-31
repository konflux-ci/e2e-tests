package remotesecret

import (
	kubeCl "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
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
