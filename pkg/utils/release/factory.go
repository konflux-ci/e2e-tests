package release

import kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"

// Contains all embebed interfaces for release operations.
type ReleaseService interface {
	// Interface for all plan related operations.
	PlansInterface

	// Interface for all release related operations.
	ReleasesInterface

	// Interface for all sbom related operations.
	SbomInterface

	// Interface for all strategy related operations.
	StrategiesInterface
}

// Factory to initialize the comunication against different API like github or kubernetes.
type releaseFactory struct {
	// Generates a kubernetes client to interact with clusters.
	*kubeCl.CustomClient
}

// Initializes all the clients and return interface to operate with release controller.
func NewSuiteController(kube *kubeCl.CustomClient) (ReleaseService, error) {
	return &releaseFactory{
		kube,
	}, nil
}
