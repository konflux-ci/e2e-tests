package github

import (
	"context"
	"fmt"
	"github.com/google/go-github/v44/github"
)

func (g *Github) DeleteRef(repository, branchName string) error {
	_, err := g.client.Git.DeleteRef(context.Background(), g.organization, repository, fmt.Sprintf("heads/%s", branchName))
	if err != nil {
		return err
	}
	return nil
}

// CreateRef creates a new ref (GitHub branch) in a specified GitHub repository,
// that will be based on the latest commit from a specified branch name
func (g *Github) CreateRef(repository, baseBranchName, newBranchName string) error {
	ctx := context.Background()
	ref, _, err := g.client.Git.GetRef(ctx, g.organization, repository, fmt.Sprintf("heads/%s", baseBranchName))
	if err != nil {
		return fmt.Errorf("error when getting the base branch name '%s' for the repo '%s': %+v", baseBranchName, repository, err)
	}
	ref.Ref = github.String(fmt.Sprintf("heads/%s", newBranchName))
	_, _, err = g.client.Git.CreateRef(ctx, g.organization, repository, ref)
	if err != nil {
		return fmt.Errorf("error when creating a new branch '%s' for the repo '%s': %+v", newBranchName, repository, err)
	}
	return nil
}
