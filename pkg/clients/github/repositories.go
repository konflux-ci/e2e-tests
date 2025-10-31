package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/google/go-github/v44/github"
	. "github.com/onsi/ginkgo/v2"
)

func (g *Github) CheckIfReleaseExist(owner, repositoryName, releaseURL string) bool {
	urlParts := strings.Split(releaseURL, "/")
	tagName := urlParts[len(urlParts)-1]
	_, _, err := g.client.Repositories.GetReleaseByTag(context.Background(), owner, repositoryName, tagName)
	if err != nil {
		GinkgoWriter.Printf("GetReleaseByTag %s returned error in repo %s : %v\n", tagName, repositoryName, err)
		return false
	}
	GinkgoWriter.Printf("Release tag %s is found in repository %s \n", tagName, repositoryName)
	return true
}

func (g *Github) DeleteRelease(owner, repositoryName, releaseURL string) bool {
	urlParts := strings.Split(releaseURL, "/")
	tagName := urlParts[len(urlParts)-1]
	release, _, err := g.client.Repositories.GetReleaseByTag(context.Background(), owner, repositoryName, tagName)
	if err != nil {
		GinkgoWriter.Printf("GetReleaseByTag returned error in repo %s : %v\n", repositoryName, err)
		return false
	}

	_, err = g.client.Repositories.DeleteRelease(context.Background(), owner, repositoryName, *release.ID)
	if err != nil {
		GinkgoWriter.Printf("DeleteRelease returned error: %v", err)
	}
	GinkgoWriter.Printf("Release tag %s is deleted in repository %s \n", tagName, repositoryName)
	return true
}

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
		opts.Ref = fmt.Sprintf(HEADS, branchName)
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
		opts.Ref = fmt.Sprintf(HEADS, branchName)
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
		getOpts.Ref = fmt.Sprintf(HEADS, branchName)
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

func (g *Github) DeleteRepositoryIfExists(name string) error {
	ctx := context.Background()

	_, resp, err := g.client.Repositories.Get(ctx, g.organization, name)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil
		}
		return fmt.Errorf("Error checking repository %s/%s: %v\n", g.organization, name, err)
	}

	_, deleteErr := g.client.Repositories.Delete(ctx, g.organization, name)
	if deleteErr != nil {
		return fmt.Errorf("Error deleting repository %s/%s: %v\n", g.organization, name, deleteErr)
	}

	return nil
}

func (g *Github) ForkRepositoryWithOrgs(sourceOrgName, sourceName, targetOrgName, targetName string) (*github.Repository, error) {
	var fork *github.Repository
	var resp *github.Response
	var repo *github.Repository

	ctx := context.Background()

	forkOptions := &github.RepositoryCreateForkOptions{
		Organization: targetOrgName,
	}

	err1 := utils.WaitUntilWithInterval(func() (done bool, err error) {
		fork, resp, err = g.client.Repositories.CreateFork(ctx, sourceOrgName, sourceName, forkOptions)
		if err != nil {
			if _, ok := err.(*github.AcceptedError); ok && resp.StatusCode == 202 {
				// This meens forking is happening asynchronously
				return true, nil
			}
			if resp.StatusCode == 403 {
				// This catches error: "403 Repository is already being forked."
				// This happens whem more than ~3 forks of one repo is ongoing in parallel
				fmt.Printf("Warning, got 403: %s", resp.Body)
				return false, nil
			}
			if resp.StatusCode == 500 {
				// This catches error 500 seen few times
				fmt.Printf("Warning, got 500: %s", resp.Body)
				return false, nil
			}
			return false, fmt.Errorf("Error forking %s/%s: %v", sourceOrgName, sourceName, err)
		}
		return true, nil
	}, time.Second * 10, time.Minute * 5)
	if err1 != nil {
		return nil, fmt.Errorf("Failed waiting for fork %s/%s: %v", sourceOrgName, sourceName, err1)
	}

	err2 := utils.WaitUntilWithInterval(func() (done bool, err error) {
		// Using this to detect repo is created and populated with content
		// https://stackoverflow.com/questions/33666838/determine-if-a-fork-is-ready
		_, _, err = g.client.Repositories.ListCommits(ctx, targetOrgName, fork.GetName(), &github.CommitsListOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	}, time.Second * 10, time.Minute * 10)
	if err2 != nil {
		return nil, fmt.Errorf("Failed waiting for commits %s/%s: %v", targetOrgName, fork.GetName(), err2)
	}

	editedRepo := &github.Repository{
		Name: github.String(targetName),
	}

	err3 := utils.WaitUntilWithInterval(func() (done bool, err error) {
		repo, resp, err = g.client.Repositories.Edit(ctx, targetOrgName, fork.GetName(), editedRepo)
		if err != nil {
			if resp.StatusCode == 422 {
				// This started to happen recently. Docs says 422 is "Validation failed, or the endpoint has been spammed." so we need to be patient.
				// Error we are getting: "422 Validation Failed [{Resource:Repository Field:name Code:custom Message:name a repository operation is already in progress}]"
				return false, nil
			}
			return false, fmt.Errorf("Error renaming %s/%s to %s: %v\n", targetOrgName, fork.GetName(), targetName, err)
		}
		return true, nil
	}, time.Second * 10, time.Minute * 10)
	if err3 != nil {
		return nil, fmt.Errorf("Failed waiting for renaming %s/%s: %v", targetOrgName, targetName, err3)
	}

	return repo, nil
}

// Fork repository in our organization
func (g *Github) ForkRepository(sourceName, targetName string) (*github.Repository, error) {
	return g.ForkRepositoryWithOrgs(g.organization, sourceName, g.organization, targetName)
}

// For repozitory from our organization to another org
func (g *Github) ForkRepositoryToOrg(sourceName, targetName, targetOrgName string) (*github.Repository, error) {
	return g.ForkRepositoryWithOrgs(g.organization, sourceName, targetOrgName, targetName)
}

// Fork repository from another organization to our org
func (g *Github) ForkRepositoryFromOrg(sourceName, targetName, sourceOrgName string) (*github.Repository, error) {
	return g.ForkRepositoryWithOrgs(sourceOrgName, sourceName, g.organization, targetName)
}
