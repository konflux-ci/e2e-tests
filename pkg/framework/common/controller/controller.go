package controller

import (
	"fmt"

	"github.com/redhat-appstudio/e2e-tests/pkg/client"
)

// Create the struct for kubernetes clients
type CommonSuiteController struct {
	*client.K8sClient
}

// Create controller for Application/Component crud operations
func NewCommonSuiteController() (*CommonSuiteController, error) {
	client, err := client.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("Error creating client-go")
	}
	return &CommonSuiteController{
		client,
	}, nil
}
