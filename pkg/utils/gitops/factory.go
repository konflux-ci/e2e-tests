package gitops

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
)

// Contains all embebed interfaces for gitops CRUD operations.
type Gitops interface {
	// Interface for all environments related operations in the kubernetes cluster.
	EnvironmentsInterface

	// Interface for all gitopsdeployments related operations in the kubernetes cluster.
	GitopsDeploymentsInterface

	// Interface for all deploymenttargetclaims related operations in the kubernetes cluster.
	DeploymentTargetClaimsInterface

	// Interface for all deploymenttargets related operations in the kubernetes cluster.
	DeploymentTargetsInterface

	// Interface for all deploymenttargetclasses related operations in the kubernetes cluster.
	DeploymentTargetClassesInterface
}

// Factory to initialize the comunication against different APIs like kubernetes.
type gitopsFactory struct {
	// Generates a client to interact with kubernetes clusters.
	*kubeCl.CustomClient
}

// Initializes all the clients and return interface to operate with application-service controller.
func NewSuiteController(kube *kubeCl.CustomClient) (Gitops, error) {
	return &gitopsFactory{
		kube,
	}, nil
}
