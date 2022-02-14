package github

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"os"
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
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	return &api
}

func (c *API) Do(req *http.Request) (*http.Response, error) {
	res, err := c.httpClient.Do(req)
	return res, err
}

func (c *API) Get(ctx context.Context, contentType string, body io.Reader, repository string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.githubAPIURL+c.organization+"/"+repository, nil)
	if err != nil {
		return nil, err
	}
	// We need to set the Authorization header because the application-service create private repositories.
	req.Header.Set("Authorization", "token "+os.Getenv("GITHUB_TOKEN"))
	req.Header.Set("Content-Type", contentType)

	return c.Do(req)
}
