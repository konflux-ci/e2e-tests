package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v44/github"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func (g *Github) GetPullRequest(repository string, id int) (*github.PullRequest, error) {
	pr, _, err := g.client.PullRequests.Get(context.Background(), g.organization, repository, id)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

func (g *Github) CreatePullRequest(repository, title, body, head, base string) (*github.PullRequest, error) {
	newPR := &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	}
	pr, _, err := g.client.PullRequests.Create(context.Background(), g.organization, repository, newPR)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

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

func (g *Github) ListCheckRuns(repository string, ref string) ([]*github.CheckRun, error) {
	checkRunResults, _, err := g.client.Checks.ListCheckRunsForRef(context.Background(), g.organization, repository, ref, &github.ListCheckRunsOptions{})
	if err != nil {
		return nil, fmt.Errorf("error when listing check runs for the repo %s and ref %s: %v", repository, ref, err)
	}
	return checkRunResults.CheckRuns, nil
}

func (g *Github) GetCheckRun(repository string, id int64) (*github.CheckRun, error) {
	checkRun, _, err := g.client.Checks.GetCheckRun(context.Background(), g.organization, repository, id)
	if err != nil {
		return nil, fmt.Errorf("error when getting check run with id %d for the repo %s: %v", id, repository, err)
	}
	return checkRun, nil
}

func (g *Github) GetPRDetails(ghRepo string, prID int) (string, string, error) {
	pullRequest, err := g.GetPullRequest(ghRepo, prID)
	if err != nil {
		return "", "", err
	}
	return *pullRequest.Head.Repo.CloneURL, *pullRequest.Head.Ref, nil
}

// GetCheckRunConclusion fetches a specific CheckRun within a given repo
// by matching the CheckRun's name with the given checkRunName, and
// then returns the CheckRun conclusion
func (g *Github) GetCheckRunConclusion(checkRunName, repoName, prHeadSha string, prNumber int) (string, error) {
	const checkrunStatusCompleted = "completed"
	var errMsgSuffix = fmt.Sprintf("repository: %s, PR number: %d, PR head SHA: %s, checkRun name: %s\n", repoName, prNumber, prHeadSha, checkRunName)

	var checkRun *github.CheckRun
	var timeout time.Duration
	var err error

	timeout = time.Minute * 5

	err = utils.WaitUntil(func() (done bool, err error) {
		checkRuns, err := g.ListCheckRuns(repoName, prHeadSha)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("got error when listing CheckRuns: %+v\n", err)
			return false, nil
		}
		for _, cr := range checkRuns {
			if strings.Contains(cr.GetName(), checkRunName) {
				checkRun = cr
				return true, nil
			}
		}
		return false, nil
	}, timeout)
	if err != nil {
		return "", fmt.Errorf("timed out when waiting for the PaC CheckRun to appear for %s", errMsgSuffix)
	}
	err = utils.WaitUntil(func() (done bool, err error) {
		checkRun, err = g.GetCheckRun(repoName, checkRun.GetID())
		if err != nil {
			ginkgo.GinkgoWriter.Printf("got error when listing CheckRuns: %+v\n", errMsgSuffix, err)
			return false, nil
		}
		currentCheckRunStatus := checkRun.GetStatus()
		if currentCheckRunStatus != checkrunStatusCompleted {
			ginkgo.GinkgoWriter.Printf("expecting CheckRun status %s, got: %s", checkrunStatusCompleted, currentCheckRunStatus)
			return false, nil
		}
		return true, nil
	}, timeout)
	if err != nil {
		return "", fmt.Errorf("timed out when waiting for the PaC CheckRun status to be '%s' for %s", checkrunStatusCompleted, errMsgSuffix)
	}
	return checkRun.GetConclusion(), nil
}
