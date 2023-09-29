package gitops

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
)

// Factory to initialize the comunication against different APIs like kubernetes.
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
