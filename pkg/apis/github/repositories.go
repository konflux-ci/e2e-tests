package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v44/github"
	. "github.com/onsi/ginkgo/v2"
)

func (g *Github) CheckIfRepositoryExist(repository string) bool {
	_, resp, err := g.client.Repositories.Get(context.Background(), g.organization, repository)
	if err != nil {
		GinkgoWriter.Printf("error when sending request to Github API: %v\n", err)
		return false
	}
	GinkgoWriter.Printf("repository %s status request to github: %d\n", repository, resp.StatusCode)
	return resp.StatusCode == 200
}

func (g *Github) CreateFile(repository, pathToFile, fileContent, branchName string) (*github.RepositoryContentResponse, error) {
	opts := &github.RepositoryContentFileOptions{
		Message: github.String("e2e test commit message"),
		Content: []byte(fileContent),
		Branch:  github.String(branchName),
	}

	file, _, err := g.client.Repositories.CreateFile(context.Background(), g.organization, repository, pathToFile, opts)
	if err != nil {
		return nil, fmt.Errorf("error when creating file contents: %v", err)
	}

	return file, nil
}

func (g *Github) GetFile(repository, pathToFile, branchName string) (*github.RepositoryContent, error) {
	opts := &github.RepositoryContentGetOptions{}
	if branchName != "" {
		opts.Ref = fmt.Sprintf("heads/%s", branchName)
	}
	file, _, _, err := g.client.Repositories.GetContents(context.Background(), g.organization, repository, pathToFile, opts)
	if err != nil {
		return nil, fmt.Errorf("error when listing file contents: %v", err)
	}

	return file, nil
}

func (g *Github) UpdateFile(repository, pathToFile, newContent, branchName, fileSHA string) (*github.RepositoryContentResponse, error) {
	opts := &github.RepositoryContentGetOptions{}
	if branchName != "" {
		opts.Ref = fmt.Sprintf("heads/%s", branchName)
	}
	newFileContent := &github.RepositoryContentFileOptions{
		Message: github.String("e2e test commit message"),
		SHA:     github.String(fileSHA),
		Content: []byte(newContent),
		Branch:  github.String(branchName),
	}
	updatedFile, _, err := g.client.Repositories.UpdateFile(context.Background(), g.organization, repository, pathToFile, newFileContent)
	if err != nil {
		return nil, fmt.Errorf("error when updating a file on github: %v", err)
	}

	return updatedFile, nil
}

func (g *Github) DeleteFile(repository, pathToFile, branchName string) error {
	getOpts := &github.RepositoryContentGetOptions{}
	deleteOpts := &github.RepositoryContentFileOptions{}

	if branchName != "" {
		getOpts.Ref = fmt.Sprintf("heads/%s", branchName)
		deleteOpts.Branch = github.String(branchName)
	}
	file, _, _, err := g.client.Repositories.GetContents(context.Background(), g.organization, repository, pathToFile, getOpts)
	if err != nil {
		return fmt.Errorf("error when listing file contents on github: %v", err)
	}

	deleteOpts = &github.RepositoryContentFileOptions{
		Message: github.String("delete test files"),
		SHA:     github.String(file.GetSHA()),
	}

	_, _, err = g.client.Repositories.DeleteFile(context.Background(), g.organization, repository, pathToFile, deleteOpts)
	if err != nil {
		return fmt.Errorf("error when deleting file on github: %v", err)
	}
	return nil
}

func (g *Github) GetAllRepositories() ([]*github.Repository, error) {

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	var allRepos []*github.Repository
	for {
		repos, resp, err := g.client.Repositories.ListByOrg(context.Background(), g.organization, opt)
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allRepos, nil
}

func (g *Github) DeleteRepository(repository *github.Repository) error {
	GinkgoWriter.Printf("Deleting repository %s\n", *repository.Name)
	_, err := g.client.Repositories.Delete(context.Background(), g.organization, *repository.Name)
	if err != nil {
		return err
	}
	return nil
}
