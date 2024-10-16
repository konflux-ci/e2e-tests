package git

// GitProvider is an enum representing possible Git providers
type GitProvider int

const (
	GitHubProvider GitProvider = iota
	GitLabProvider
)

// PullRequest represents a generic provider-agnostic pull/merge request
type PullRequest struct {
	Number int
	// SourceBranch includes the changes made in the pull request
	SourceBranch string
	// TargetBranch is the base branch on top of which will the changes be merged
	TargetBranch string
	// MergeCommitSHA is the revision of the commit which merged the PullRequest
	MergeCommitSHA string
	// HeadSHA is the revision of the commit that is on top of the SourceBranch
	HeadSHA string
}

type Client interface {
	CreateBranch(repository, baseBranchName, revision, branchName string) error
	DeleteBranch(repository, branchName string) error
	BranchExists(repository, branchName string) (bool, error)
	ListPullRequests(repository string) ([]*PullRequest, error)
	CreateFile(repository, pathToFile, content, branchName string) error
	GetFileContent(repository, pathToFile, branchName string) (string, error)
	CreatePullRequest(repository, title, body, head, base string) (*PullRequest, error)
	MergePullRequest(repository string, prNumber int) (*PullRequest, error)
	CleanupWebhooks(repository, clusterAppDomain string) error
}
