package git

import (
	"encoding/base64"

	gitlab2 "github.com/xanzy/go-gitlab"

	"github.com/konflux-ci/e2e-tests/pkg/clients/gitlab"
)

type GitLabClient struct {
	*gitlab.GitlabClient
}

func NewGitlabClient(gl *gitlab.GitlabClient) *GitLabClient {
	return &GitLabClient{gl}
}

func (g *GitLabClient) CreateBranch(repository, baseBranchName, _, branchName string) error {
	return g.GitlabClient.CreateBranch(repository, branchName, baseBranchName)
}

func (g *GitLabClient) BranchExists(repository, branchName string) (bool, error) {
	return g.ExistsBranch(repository, branchName)
}

func (g *GitLabClient) ListPullRequests(string) ([]*PullRequest, error) {
	mrs, err := g.GetMergeRequests()
	if err != nil {
		return nil, err
	}
	var pullRequests []*PullRequest
	for _, mr := range mrs {
		pullRequests = append(pullRequests, &PullRequest{
			Number:       mr.IID,
			SourceBranch: mr.SourceBranch,
			TargetBranch: mr.TargetBranch,
			HeadSHA:      mr.SHA,
		})
	}
	return pullRequests, nil
}

func (g *GitLabClient) CreateFile(repository, pathToFile, content, branchName string) (*RepositoryFile, error) {
	_, err := g.GitlabClient.CreateFile(repository, pathToFile, content, branchName)
	if err != nil {
		return nil, err
	}

	opts := gitlab2.GetFileOptions{Ref: gitlab2.Ptr(branchName)}
	file, _, err := g.GitlabClient.GetClient().RepositoryFiles.GetFile(repository, pathToFile, &opts)
	if err != nil {
		return nil, err
	}

	resultFile := &RepositoryFile{
		CommitSHA: file.CommitID,
	}
	return resultFile, nil
}

func (g *GitLabClient) GetFile(repository, pathToFile, branchName string) (*RepositoryFile, error) {
	opts := gitlab2.GetFileOptions{Ref: gitlab2.Ptr(branchName)}
	file, _, err := g.GitlabClient.GetClient().RepositoryFiles.GetFile(repository, pathToFile, &opts)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		return nil, err
	}
	resultFile := &RepositoryFile{
		CommitSHA: file.CommitID,
		Content:   string(decoded),
	}
	return resultFile, nil
}

func (g *GitLabClient) MergePullRequest(repository string, prNumber int) (*PullRequest, error) {
	mr, err := g.AcceptMergeRequest(repository, prNumber)
	if err != nil {
		return nil, err
	}
	return &PullRequest{
		Number:         mr.IID,
		SourceBranch:   mr.SourceBranch,
		TargetBranch:   mr.TargetBranch,
		HeadSHA:        mr.SHA,
		MergeCommitSHA: mr.MergeCommitSHA,
	}, nil
}

func (g *GitLabClient) CreatePullRequest(repository, title, body, head, base string) (*PullRequest, error) {
	opts := gitlab2.CreateMergeRequestOptions{
		Title:        gitlab2.Ptr(title),
		Description:  gitlab2.Ptr(body),
		SourceBranch: gitlab2.Ptr(head),
		TargetBranch: gitlab2.Ptr(base),
	}
	mr, _, err := g.GitlabClient.GetClient().MergeRequests.CreateMergeRequest(repository, &opts)
	if err != nil {
		return nil, err
	}
	return &PullRequest{
		Number:       mr.IID,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		HeadSHA:      mr.SHA,
	}, nil
}

func (g *GitLabClient) CleanupWebhooks(repository, clusterAppDomain string) error {
	return g.DeleteWebhooks(repository, clusterAppDomain)
}

func (g *GitLabClient) DeleteBranchAndClosePullRequest(repository string, prNumber int) error {
	mr, _, err := g.GitlabClient.GetClient().MergeRequests.GetMergeRequest(repository, prNumber, nil)
	if err != nil {
		return err
	}
	err = g.DeleteBranch(repository, mr.SourceBranch)
	if err != nil {
		return err
	}
	return g.CloseMergeRequest(repository, prNumber)
}
