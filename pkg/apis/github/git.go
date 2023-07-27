package github

import (
	"context"
	"fmt"
	"strings"

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
// that will be based on the commit specified with sha. If sha is not specified
// the latest commit from base branch will be used.
func (g *Github) CreateRef(repository, baseBranchName, sha, newBranchName string) error {
	ctx := context.Background()
	ref, _, err := g.client.Git.GetRef(ctx, g.organization, repository, fmt.Sprintf("heads/%s", baseBranchName))
	if err != nil {
		return fmt.Errorf("error when getting the base branch name '%s' for the repo '%s': %+v", baseBranchName, repository, err)
	}

	ref.Ref = github.String(fmt.Sprintf("heads/%s", newBranchName))

	if sha != "" {
		ref.Object.SHA = &sha
	}

	_, _, err = g.client.Git.CreateRef(ctx, g.organization, repository, ref)
	if err != nil {
		return fmt.Errorf("error when creating a new branch '%s' for the repo '%s': %+v", newBranchName, repository, err)
	}
	return nil
}

func (g *Github) ExistsRef(repository, branchName string) (bool, error) {
	_, _, err := g.client.Git.GetRef(context.Background(), g.organization, repository, fmt.Sprintf("heads/%s", branchName))
	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			return false, nil
		} else {
			return false, fmt.Errorf("error when getting the branch '%s' for the repo '%s': %+v", branchName, repository, err)
		}
	}
	return true, nil
}
