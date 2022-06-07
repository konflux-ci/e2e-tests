package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v44/github"
	"k8s.io/klog/v2"
)

func (g *Github) CheckIfRepositoryExist(repository string) bool {
	_, resp, err := g.client.Repositories.Get(context.Background(), g.organization, repository)
	if err != nil {
		klog.Errorf("error when sending request to Github API: %v", err)
		return false
	}
	klog.Infof("repository %s status request to github: %d", repository, resp.StatusCode)
	return resp.StatusCode == 200
}

func (g *Github) UpdateFile(repository, pathToFile, newContent string) (*github.RepositoryContentResponse, error) {
	file, _, _, err := g.client.Repositories.GetContents(context.Background(), g.organization, repository, pathToFile, &github.RepositoryContentGetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error when listing file contents: %v", err)
	}
	fileSha := file.GetSHA()
	newFileContent := &github.RepositoryContentFileOptions{
		Message: github.String("e2e test commit message"),
		SHA:     github.String(fileSha),
		Content: []byte(newContent),
	}
	updatedFile, _, err := g.client.Repositories.UpdateFile(context.Background(), g.organization, repository, pathToFile, newFileContent)
	if err != nil {
		return nil, fmt.Errorf("error when updating a file on github: %v", err)
	}

	return updatedFile, nil
}
