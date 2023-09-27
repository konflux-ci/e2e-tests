package tekton

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
)

// Create the struct for kubernetes clients
type TektonController struct {
	*kubeCl.CustomClient
}

// Create controller for Tekton Task/Pipeline CRUD operations
func NewSuiteController(kube *kubeCl.CustomClient) *TektonController {
	return &TektonController{
		kube,
	}
}
