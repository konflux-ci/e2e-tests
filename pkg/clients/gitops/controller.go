package gitops

import (
	kubeCl "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
)

// Factory to initialize the communication against different APIs like kubernetes.
type GitopsController struct {
	// Generates a client to interact with kubernetes clusters.
	*kubeCl.CustomClient
}

// Initializes all the clients and return interface to operate with application-service controller.
func NewSuiteController(kube *kubeCl.CustomClient) (*GitopsController, error) {
	return &GitopsController{
		kube,
	}, nil
}
