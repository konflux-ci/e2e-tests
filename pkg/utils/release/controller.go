package release

import kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"

// Factory to initialize the comunication against different API like github or kubernetes.
type ReleaseController struct {
	// Generates a kubernetes client to interact with clusters.
	*kubeCl.CustomClient
}

// Initializes all the clients and return interface to operate with release controller.
func NewSuiteController(kube *kubeCl.CustomClient) (*ReleaseController, error) {
	return &ReleaseController{
		kube,
	}, nil
}
