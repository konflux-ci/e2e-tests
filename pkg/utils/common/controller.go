package common

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

// Create the struct for kubernetes and github clients
type SuiteController struct {
	*kubeCl.CustomClient
	Github *github.Github
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
	return &SuiteController{
		kubeC,
		gh,
	}, nil
}
