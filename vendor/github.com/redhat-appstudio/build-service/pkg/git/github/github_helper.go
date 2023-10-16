/*
Copyright 2022-2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package github

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/redhat-appstudio/build-service/pkg/boerrors"
	gp "github.com/redhat-appstudio/build-service/pkg/git/gitprovider"
)

func getOwnerAndRepoFromUrl(repoUrl string) (owner string, repository string) {
	// https://github.com/owner/repository
	gitSourceUrlParts := strings.Split(strings.TrimSuffix(repoUrl, ".git"), "/")
	owner = gitSourceUrlParts[3]
	repository = gitSourceUrlParts[4]
	return owner, repository
}

// refineGitHostingServiceError generates expected permanent error from GitHub response.
// If no one is detected, the original error will be returned.
// RefineGitHostingServiceError should be called just after every GitHub API call.
func refineGitHostingServiceError(response *http.Response, originErr error) error {
	// go-github APIs do not return a http.Response object if the error is not related to an HTTP request.
	if response == nil {
		return originErr
	}
	if _, ok := originErr.(*github.RateLimitError); ok {
		return boerrors.NewBuildOpError(boerrors.EGitHubReachRateLimit, originErr)
	}
	switch response.StatusCode {
	case http.StatusUnauthorized:
		// Client's access token can't be recognized by GitHub.
		return boerrors.NewBuildOpError(boerrors.EGitHubTokenUnauthorized, originErr)
	case http.StatusNotFound:
		// No expected resource is found due to insufficient scope set to the client's access token.
		scopes := response.Header["X-Oauth-Scopes"]
		err := boerrors.NewBuildOpError(boerrors.EGitHubNoResourceToOperateOn, originErr)
		if len(scopes) == 0 {
			err.ExtraInfo = "No scope is found from response header. Check it from GitHub settings."
		} else {
			err.ExtraInfo = fmt.Sprintf("Scopes set to access token: %s", strings.Join(scopes, ", "))
		}
		return err
	default:
		return originErr
	}
}

func (g *GithubClient) branchExist(owner, repository, branch string) (bool, error) {
	_, resp, err := g.client.Git.GetRef(g.ctx, owner, repository, "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	switch resp.StatusCode {
	case 401:
		return false, boerrors.NewBuildOpError(boerrors.EGitHubTokenUnauthorized, err)
	case 404:
		return false, nil
	}

	return false, err
}

func (g *GithubClient) getBranch(owner, repository, branch string) (*github.Reference, error) {
	ref, resp, err := g.client.Git.GetRef(g.ctx, owner, repository, "refs/heads/"+branch)
	return ref, refineGitHostingServiceError(resp.Response, err)
}

func (g *GithubClient) createBranch(owner, repository, branch, baseBranch string) (*github.Reference, error) {
	baseBranchRef, err := g.getBranch(owner, repository, baseBranch)
	if err != nil {
		return nil, err
	}
	newBranchRef := &github.Reference{
		Ref:    github.String("refs/heads/" + branch),
		Object: &github.GitObject{SHA: baseBranchRef.Object.SHA},
	}
	ref, resp, err := g.client.Git.CreateRef(g.ctx, owner, repository, newBranchRef)
	return ref, refineGitHostingServiceError(resp.Response, err)
}

func (g *GithubClient) deleteBranch(owner, repository, branch string) (bool, error) {
	resp, err := g.client.Git.DeleteRef(g.ctx, owner, repository, "refs/heads/"+branch)
	if err != nil {
		if resp.Response.StatusCode == 422 {
			// The given branch doesn't exist
			return false, nil
		}
		return false, refineGitHostingServiceError(resp.Response, err)
	}
	return true, nil
}

func (g *GithubClient) getDefaultBranch(owner, repository string) (string, error) {
	repositoryInfo, resp, err := g.client.Repositories.Get(g.ctx, owner, repository)
	if err != nil {
		return "", refineGitHostingServiceError(resp.Response, err)
	}
	if repositoryInfo == nil {
		return "", fmt.Errorf("repository info is empty in GitHub API response")
	}
	return *repositoryInfo.DefaultBranch, nil
}

func (g *GithubClient) filesUpToDate(owner, repository, branch string, files []gp.RepositoryFile) (bool, error) {
	for _, file := range files {
		opts := &github.RepositoryContentGetOptions{
			Ref: "refs/heads/" + branch,
		}

		fileContentReader, resp, err := g.client.Repositories.DownloadContents(g.ctx, owner, repository, file.FullPath, opts)
		if err != nil {
			// It's not clear when it returns 404 or 200 with the error message. Check both.
			if resp.StatusCode == 404 || strings.Contains(err.Error(), "no file named") {
				// Given file not found
				return false, nil
			}

			return false, refineGitHostingServiceError(resp.Response, err)
		}
		fileContent, err := io.ReadAll(fileContentReader)
		if err != nil {
			return false, err
		}

		if !bytes.Equal(fileContent, file.Content) {
			return false, nil
		}
	}
	return true, nil
}

// filesExistInDirectory checks if given files exist under specified directory.
// Returns subset of given files which exist.
func (g *GithubClient) filesExistInDirectory(owner, repository, branch, directoryPath string, files []gp.RepositoryFile) ([]gp.RepositoryFile, error) {
	existingFiles := make([]gp.RepositoryFile, 0, len(files))

	opts := &github.RepositoryContentGetOptions{
		Ref: "refs/heads/" + branch,
	}
	_, dirContent, resp, err := g.client.Repositories.GetContents(g.ctx, owner, repository, directoryPath, opts)
	if err != nil {
		switch resp.StatusCode {
		case 401:
			return existingFiles, boerrors.NewBuildOpError(boerrors.EGitHubTokenUnauthorized, err)
		case 404:
			return existingFiles, nil
		}
		return existingFiles, err
	}

	for _, file := range dirContent {
		if file.GetType() != "file" {
			continue
		}
		for _, f := range files {
			if file.GetPath() == f.FullPath {
				existingFiles = append(existingFiles, gp.RepositoryFile{FullPath: file.GetPath()})
				break
			}
		}
	}

	return existingFiles, nil
}

func (g *GithubClient) createTree(owner, repository string, baseRef *github.Reference, files []gp.RepositoryFile) (tree *github.Tree, err error) {
	// Load each file into the tree.
	entries := []*github.TreeEntry{}
	for _, file := range files {
		entries = append(entries, &github.TreeEntry{Path: github.String(file.FullPath), Type: github.String("blob"), Content: github.String(string(file.Content)), Mode: github.String("100644")})
	}

	tree, resp, err := g.client.Git.CreateTree(g.ctx, owner, repository, *baseRef.Object.SHA, entries)
	return tree, refineGitHostingServiceError(resp.Response, err)
}

func (g *GithubClient) deleteFromTree(owner, repository string, baseRef *github.Reference, files []gp.RepositoryFile) (tree *github.Tree, err error) {
	// Delete each file from the tree.
	entries := []*github.TreeEntry{}
	for _, file := range files {
		entries = append(entries, &github.TreeEntry{
			Path: github.String(file.FullPath),
			Type: github.String("blob"),
			Mode: github.String("100644"),
		})
	}

	tree, resp, err := g.client.Git.CreateTree(g.ctx, owner, repository, *baseRef.Object.SHA, entries)
	return tree, refineGitHostingServiceError(resp.Response, err)
}

func (g *GithubClient) addCommitToBranch(owner, repository, authorName, authorEmail, commitMessage string, files []gp.RepositoryFile, ref *github.Reference) error {
	// Get the parent commit to attach the commit to.
	parent, resp, err := g.client.Repositories.GetCommit(g.ctx, owner, repository, *ref.Object.SHA, nil)
	if err != nil {
		return refineGitHostingServiceError(resp.Response, err)
	}
	// This is not always populated, but is needed.
	parent.Commit.SHA = parent.SHA

	tree, err := g.createTree(owner, repository, ref, files)
	if err != nil {
		return err
	}

	// Create the commit using the tree.
	date := time.Now()
	author := &github.CommitAuthor{Date: &date, Name: &authorName, Email: &authorEmail}
	commit := &github.Commit{Author: author, Message: &commitMessage, Tree: tree, Parents: []*github.Commit{parent.Commit}}
	newCommit, resp, err := g.client.Git.CreateCommit(g.ctx, owner, repository, commit)
	if err != nil {
		return refineGitHostingServiceError(resp.Response, err)
	}

	// Attach the created commit to the given branch.
	ref.Object.SHA = newCommit.SHA
	_, resp, err = g.client.Git.UpdateRef(g.ctx, owner, repository, ref, false)
	return refineGitHostingServiceError(resp.Response, err)
}

// Creates commit into specified branch that deletes given files.
func (g *GithubClient) addDeleteCommitToBranch(owner, repository, authorName, authorEmail, commitMessage string, files []gp.RepositoryFile, ref *github.Reference) error {
	// Get the parent commit to attach the commit to.
	parent, resp, err := g.client.Repositories.GetCommit(g.ctx, owner, repository, *ref.Object.SHA, nil)
	if err != nil {
		return refineGitHostingServiceError(resp.Response, err)
	}
	// This is not always populated, but needed.
	parent.Commit.SHA = parent.SHA

	tree, err := g.deleteFromTree(owner, repository, ref, files)
	if err != nil {
		return err
	}

	// Create the commit using the tree.
	date := time.Now()
	author := &github.CommitAuthor{Date: &date, Name: &authorName, Email: &authorEmail}
	commit := &github.Commit{Author: author, Message: &commitMessage, Tree: tree, Parents: []*github.Commit{parent.Commit}}
	newCommit, resp, err := g.client.Git.CreateCommit(g.ctx, owner, repository, commit)
	if err != nil {
		return refineGitHostingServiceError(resp.Response, err)
	}

	// Attach the created commit to the given branch.
	ref.Object.SHA = newCommit.SHA
	_, resp, err = g.client.Git.UpdateRef(g.ctx, owner, repository, ref, false)
	return refineGitHostingServiceError(resp.Response, err)
}

// findPullRequestByBranchesWithinRepository searches for a PR within repository by current and target (base) branch.
func (g *GithubClient) findPullRequestByBranchesWithinRepository(owner, repository, branchName, baseBranchName string) (*github.PullRequest, error) {
	opts := &github.PullRequestListOptions{
		State:       "open",
		Base:        baseBranchName,
		Head:        owner + ":" + branchName,
		ListOptions: github.ListOptions{PerPage: 100},
	}
	prs, resp, err := g.client.PullRequests.List(g.ctx, owner, repository, opts)
	if err != nil {
		return nil, refineGitHostingServiceError(resp.Response, err)
	}
	switch len(prs) {
	case 0:
		return nil, nil
	case 1:
		return prs[0], nil
	default:
		return nil, fmt.Errorf("failed to find pull request by branch %s: %d matches found", opts.Head, len(prs))
	}
}

// createPullRequestWithinRepository create a new pull request into the same repository.
// Returns url to the created pull request.
func (g *GithubClient) createPullRequestWithinRepository(owner, repository, branchName, baseBranchName, prTitle, prText string) (string, error) {
	branch := fmt.Sprintf("%s:%s", owner, branchName)

	newPRData := &github.NewPullRequest{
		Title:               &prTitle,
		Head:                &branch,
		Base:                &baseBranchName,
		Body:                &prText,
		MaintainerCanModify: github.Bool(true),
	}

	pr, resp, err := g.client.PullRequests.Create(g.ctx, owner, repository, newPRData)
	if err != nil {
		return "", refineGitHostingServiceError(resp.Response, err)
	}

	return pr.GetHTMLURL(), nil
}

// getWebhookByTargetUrl returns webhook by its target url or nil if such webhook doesn't exist.
func (g *GithubClient) getWebhookByTargetUrl(owner, repository, webhookTargetUrl string) (*github.Hook, error) {
	// Suppose that the repository does not have more than 100 webhooks
	listOpts := &github.ListOptions{PerPage: 100}
	webhooks, resp, err := g.client.Repositories.ListHooks(g.ctx, owner, repository, listOpts)
	if err != nil {
		return nil, refineGitHostingServiceError(resp.Response, err)
	}

	for _, webhook := range webhooks {
		if webhook.Config["url"] == webhookTargetUrl {
			return webhook, nil
		}
	}
	// Webhook with the given URL not found
	return nil, nil
}

func (g *GithubClient) createWebhook(owner, repository string, webhook *github.Hook) (*github.Hook, error) {
	webhook, resp, err := g.client.Repositories.CreateHook(g.ctx, owner, repository, webhook)
	return webhook, refineGitHostingServiceError(resp.Response, err)
}

func (g *GithubClient) updateWebhook(owner, repository string, webhook *github.Hook) (*github.Hook, error) {
	webhook, resp, err := g.client.Repositories.EditHook(g.ctx, owner, repository, *webhook.ID, webhook)
	return webhook, refineGitHostingServiceError(resp.Response, err)
}

func (g *GithubClient) deleteWebhook(owner, repository string, webhookId int64) error {
	resp, err := g.client.Repositories.DeleteHook(g.ctx, owner, repository, webhookId)
	if err != nil {
		switch resp.StatusCode {
		case 401:
			return boerrors.NewBuildOpError(boerrors.EGitHubTokenUnauthorized, err)
		case 404:
			// Note: GitHub responds 404 in the following two cases:
			// 1) delete a nonexisting hook with sufficient scope
			// 2) delete an existing hook without sufficient scope.
			return nil
		}
	}
	return nil
}
