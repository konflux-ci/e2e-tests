package common

import (
	"fmt"

	"github.com/konflux-ci/e2e-tests/pkg/clients/github"
	"github.com/konflux-ci/e2e-tests/pkg/clients/gitlab"
	kubeCl "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

// Create the struct for kubernetes and github clients.
type SuiteController struct {
	// Wrap K8S client go to interact with Kube cluster
	*kubeCl.CustomClient

	// Github client to interact with GH apis
	Github *github.Github
	Gitlab *gitlab.GitlabClient
}

/*
Create controller for the common kubernetes API crud operations. This controller should be used only to interact with non RHTAP/AppStudio APIS like routes, deployment, pods etc...
Check if a github organization env var is set, if not use by default the redhat-appstudio-qe org. See: https://github.com/redhat-appstudio-qe
*/
func NewSuiteController(kubeC *kubeCl.CustomClient) (*SuiteController, error) {
	gh, err := github.NewGithubClient(utils.GetEnv(constants.GITHUB_TOKEN_ENV, ""), utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))
	if err != nil {
		return nil, err
	}

	gl, err := gitlab.NewGitlabClient(utils.GetEnv(constants.GITLAB_TOKEN_ENV, ""), utils.GetEnv(constants.GITLAB_URL_ENV, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with GitLab: %w", err)
	}

	return &SuiteController{
		kubeC,
		gh,
		gl,
	}, nil
}
