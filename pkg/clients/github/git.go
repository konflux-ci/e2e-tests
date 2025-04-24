package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v44/github"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

func (g *Github) DeleteRef(repository, branchName string) error {
	_, err := g.client.Git.DeleteRef(context.Background(), g.organization, repository, fmt.Sprintf(HEADS, branchName))
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
	ref, _, err := g.client.Git.GetRef(ctx, g.organization, repository, fmt.Sprintf(HEADS, baseBranchName))
	if err != nil {
		return fmt.Errorf("error when getting the base branch name '%s' for the repo '%s': %+v", baseBranchName, repository, err)
	}

	ref.Ref = github.String(fmt.Sprintf(HEADS, newBranchName))

	if sha != "" {
		ref.Object.SHA = &sha
	}

	_, _, err = g.client.Git.CreateRef(ctx, g.organization, repository, ref)
	if err != nil {
		return fmt.Errorf("error when creating a new branch '%s' for the repo '%s': %+v", newBranchName, repository, err)
	}
	err = utils.WaitUntilWithInterval(func() (done bool, err error) {
		exist, err := g.ExistsRef(repository, newBranchName)
		if err != nil {
			return false, err
		}
		if exist && err == nil {
			return exist, err
		}
		return false, nil
	}, 2*time.Second, 2*time.Minute) //Wait for the branch to actually exist
	if err != nil {
		return fmt.Errorf("error when waiting for ref: %+v", err)
	}
	return nil
}

func (g *Github) ExistsRef(repository, branchName string) (bool, error) {
	_, _, err := g.client.Git.GetRef(context.Background(), g.organization, repository, fmt.Sprintf(HEADS, branchName))
	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			return false, nil
		} else {
			return false, fmt.Errorf("error when getting the branch '%s' for the repo '%s': %+v", branchName, repository, err)
		}
	}
	return true, nil
}

func (g *Github) UpdateGithubOrg(githubOrg string) {
	g.organization = githubOrg
}

// CreateCommit creates a new commit directly on the specified branch
func (g *Github) CreateCommit(repository, branch, path string, content []byte, message string) (string, error) {
	ctx := context.Background()

	// Get the reference to the branch
	ref, _, err := g.client.Git.GetRef(ctx, g.organization, repository, fmt.Sprintf(HEADS, branch))
	if err != nil {
		return "", fmt.Errorf("error getting ref for branch %s: %v", branch, err)
	}

	// Get the tree for the given reference
	tree, _, err := g.client.Git.GetTree(ctx, g.organization, repository, *ref.Object.SHA, false)
	if err != nil {
		return "", fmt.Errorf("error getting tree: %v", err)
	}

	// Create a blob with the file content
	blob := &github.Blob{
		Content:  github.String(string(content)),
		Encoding: github.String("utf-8"),
	}
	blob, _, err = g.client.Git.CreateBlob(ctx, g.organization, repository, blob)
	if err != nil {
		return "", fmt.Errorf("error creating blob: %v", err)
	}

	// Create a new tree with the new file
	entries := []*github.TreeEntry{
		{
			Path: github.String(path),
			Mode: github.String("100644"),
			Type: github.String("blob"),
			SHA:  blob.SHA,
		},
	}
	newTree, _, err := g.client.Git.CreateTree(ctx, g.organization, repository, *tree.SHA, entries)
	if err != nil {
		return "", fmt.Errorf("error creating tree: %v", err)
	}

	// Create the commit
	parent, _, err := g.client.Git.GetCommit(ctx, g.organization, repository, *ref.Object.SHA)
	if err != nil {
		return "", fmt.Errorf("error getting parent commit: %v", err)
	}

	commit := &github.Commit{
		Message: github.String(message),
		Tree:    newTree,
		Parents: []*github.Commit{parent},
	}
	newCommit, _, err := g.client.Git.CreateCommit(ctx, g.organization, repository, commit)
	if err != nil {
		return "", fmt.Errorf("error creating commit: %v", err)
	}

	// Update the reference
	ref.Object.SHA = newCommit.SHA
	_, _, err = g.client.Git.UpdateRef(ctx, g.organization, repository, ref, false)
	if err != nil {
		return "", fmt.Errorf("error updating ref: %v", err)
	}

	return *newCommit.SHA, nil
}
