package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v44/github"
)

func (g *Github) ListPullRequests(repository string) ([]*github.PullRequest, error) {
	prs, _, err := g.client.PullRequests.List(context.Background(), g.organization, repository, &github.PullRequestListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error when listing pull requests for the repo %s: %v", repository, err)
	}

	return prs, nil
}

func (g *Github) ListPullRequestCommentsSince(repository string, prNumber int, since time.Time) ([]*github.IssueComment, error) {
	comments, _, err := g.client.Issues.ListComments(context.Background(), g.organization, repository, prNumber, &github.IssueListCommentsOptions{
		Since:     &since,
		Sort:      github.String("created"),
		Direction: github.String("asc"),
	})
	if err != nil {
		return nil, fmt.Errorf("error when listing pull requests comments for the repo %s: %v", repository, err)
	}

	return comments, nil
}

func (g *Github) MergePullRequest(repository string, prNumber int) (*github.PullRequestMergeResult, error) {
	mergeResult, _, err := g.client.PullRequests.Merge(context.Background(), g.organization, repository, prNumber, "", &github.PullRequestOptions{})
	if err != nil {
		return nil, fmt.Errorf("error when merging pull request number %d for the repo %s: %v", prNumber, repository, err)
	}

	return mergeResult, nil
}
