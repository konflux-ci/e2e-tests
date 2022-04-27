package github

import (
	"context"

	"k8s.io/klog/v2"
)

func (g *API) CheckIfRepositoryExist(repository string) bool {
	repoRequest, err := g.Get(context.TODO(), "application/json", nil, repository)
	if err != nil {
		klog.Errorf("Error when sending request to Github API: %v", err)
		return false
	}
	klog.Infof("Repository %s status request to github: %d", repository, repoRequest.StatusCode)
	return repoRequest.StatusCode == 200
}
