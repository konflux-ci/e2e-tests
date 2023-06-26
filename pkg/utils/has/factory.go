package has

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
)

// Contains all embebed interfaces for application-services operations.
type ApplicationService interface {
	// Interface for all applications related operations in the kubernetes cluster.
	ApplicationsInterface

	// Interface for all components related operations in the kubernetes cluster.
	ComponentsInterface

	// Interface for all applications related operations in the kubernetes cluster.
	ComponentDetectionQueriesInterface

	// Interface for all snapshotenvironmentbinding related operations in the kubernetes cluster.
	SnapshotEnvironmentBindingsInterface
}

// Factory to initialize the comunication against different API like github or kubernetes.
type hasFactory struct {
	// A Client manages communication with the GitHub API in a specific Organization.
	Github *github.Github

	// Generates a kubernetes client to interact with clusters.
	*kubeCl.CustomClient
}

// Initializes all the clients and return interface to operate with application-service controller.
func NewSuiteController(kube *kubeCl.CustomClient) (ApplicationService, error) {
	gh, err := github.NewGithubClient(utils.GetEnv(constants.GITHUB_TOKEN_ENV, ""),
		utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))
	if err != nil {
		return nil, err
	}

	return &hasFactory{
		gh,
		kube,
	}, nil
}
