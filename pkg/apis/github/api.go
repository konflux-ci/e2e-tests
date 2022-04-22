package github

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

type API struct {
	httpClient   *http.Client
	githubAPIURL string
	organization string
}

func NewGitubClient(organization string) *API {
	api := API{
		githubAPIURL: "https://api.github.com/repos/",
		organization: organization,
	}
	api.httpClient = &http.Client{
		Transport: &http.Transport{},
	}
	return &api
}

func (c *API) Do(req *http.Request) (*http.Response, error) {
	res, err := c.httpClient.Do(req)
	return res, err
}

func (c *API) Get(ctx context.Context, contentType string, body io.Reader, repository string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s%s/%s", c.githubAPIURL, c.organization, repository), nil)
	if err != nil {
		return nil, err
	}

	// We need to set the Authorization header because the application-service create private repositories. The github token is already checked if exists in before tests suites
	req.Header.Set("Authorization", fmt.Sprintf("token %s", utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")))
	req.Header.Set("Content-Type", contentType)

	return c.Do(req)
}
