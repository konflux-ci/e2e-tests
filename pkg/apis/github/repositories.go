package github

import (
	"context"
)

func (g *API) CheckIfRepositoryExist(repository string) bool {
	repoRequest, err := g.Get(context.TODO(), "application/json", nil, repository)
	if err != nil {
		return false
	}
	if repoRequest.StatusCode == 200 {
		return true
	}
	return false
}
