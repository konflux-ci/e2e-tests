package has

import (
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"

	"github.com/konflux-ci/e2e-tests/pkg/clients/github"
	"github.com/konflux-ci/e2e-tests/pkg/clients/gitlab"
	kubeCl "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
)

// Factory to initialize the comunication against different API like github, gitlab or kubernetes.
type HasController struct {
	// A Client manages communication with the GitHub API in a specific Organization.
	Github *github.Github

	// A Client manages communication with the GitLab API in a specific Organization.
	GitLab *gitlab.GitlabClient

	// Generates a kubernetes client to interact with clusters.
	*kubeCl.CustomClient
}

// Initializes all the clients and return interface to operate with application-service controller.
func NewSuiteController(kube *kubeCl.CustomClient) (*HasController, error) {
	gh, err := github.NewGithubClient(utils.GetEnv(constants.GITHUB_TOKEN_ENV, ""),
		utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))
	if err != nil {
		return nil, err
	}

	gl, err := gitlab.NewGitlabClient(utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, ""),
		utils.GetEnv(constants.GITLAB_API_URL_ENV, constants.DefaultGitLabAPIURL))
	if err != nil {
		return nil, err
	}

	return &HasController{
		gh,
		gl,
		kube,
	}, nil
}
