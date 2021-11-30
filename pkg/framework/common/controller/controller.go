package controller

import (
	"fmt"

	"github.com/redhat-appstudio/e2e-tests/pkg/client"
)

type HASSuiteController struct {
	*client.K8sClient
}

func NewCommonSuiteController() (*HASSuiteController, error) {
	client, err := client.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("Error creating client-go")
	}
	return &HASSuiteController{
		client,
	}, nil
}
