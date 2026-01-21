package gitlab

import (
	"net/http"
	"time"

	gitlabClient "github.com/xanzy/go-gitlab"
)

const (
	HEADS = "refs/heads/%s"
)

type GitlabClient struct {
	client *gitlabClient.Client
}

func NewGitlabClient(accessToken, baseUrl string) (*GitlabClient, error) {
	var err error
	var glc = &GitlabClient{}

	// Create a custom http.Client with a 1 minute timeout
	var customHttpClient = &http.Client{Timeout: 1 * time.Minute}
	customHttpClient.Transport = &http.Transport{
		MaxIdleConns:    100,
		IdleConnTimeout: 20 * time.Minute,
	}

	glc.client, err = gitlabClient.NewClient(accessToken, gitlabClient.WithBaseURL(baseUrl), gitlabClient.WithHTTPClient(customHttpClient))
	if err != nil {
		return nil, err
	}
	return glc, nil
}

// GetClient returns the underlying gitlab client
func (gc *GitlabClient) GetClient() *gitlabClient.Client {
	return gc.client
}
