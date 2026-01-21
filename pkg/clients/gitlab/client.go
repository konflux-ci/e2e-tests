package gitlab

import (
	gitlabClient "github.com/xanzy/go-gitlab"
)

const (
	HEADS = "refs/heads/%s"
)

type GitlabClient struct {
	client  *gitlabClient.Client
	groupID string
}

func NewGitlabClient(accessToken, baseUrl, groupID string) (*GitlabClient, error) {
	var err error
	var glc = &GitlabClient{groupID: groupID}
	glc.client, err = gitlabClient.NewClient(accessToken, gitlabClient.WithBaseURL(baseUrl))
	if err != nil {
		return nil, err
	}
	return glc, nil
}

// GetClient returns the underlying gitlab client
func (gc *GitlabClient) GetClient() *gitlabClient.Client {
	return gc.client
}
