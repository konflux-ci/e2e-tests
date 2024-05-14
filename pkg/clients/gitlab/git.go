package gitlab

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	gitlab "github.com/xanzy/go-gitlab"
)

// CreateBranch creates a new branch in a GitLab project with the given projectID and newBranchName
func (gc *GitlabClient) CreateBranch(projectID, newBranchName, defaultBranch string) error {
	// Prepare the branch creation request
	branchOpts := &gitlab.CreateBranchOptions{
		Branch: gitlab.Ptr(newBranchName),
		Ref:    gitlab.Ptr(defaultBranch),
	}

	// Perform the branch creation
	_, _, err := gc.client.Branches.CreateBranch(projectID, branchOpts)
	if err != nil {
		return fmt.Errorf("failed to create branch %s in project %s: %w", newBranchName, projectID, err)
	}

	// Wait for the branch to actually exist
	Eventually(func(gomega Gomega) {
		exist, err := gc.ExistsBranch(projectID, newBranchName)
		gomega.Expect(err).NotTo(HaveOccurred())
		gomega.Expect(exist).To(BeTrue())

	}, 2*time.Minute, 2*time.Second).Should(Succeed())

	return nil
}

// ExistsBranch checks if a branch exists in a specified GitLab repository.
func (gc *GitlabClient) ExistsBranch(projectID, branchName string) (bool, error) {

	fmt.Println("ExistRdf dddd")
	_, _, err := gc.client.Branches.GetBranch(projectID, branchName)
	if err == nil {
		return true, nil
	}
	if err, ok := err.(*gitlab.ErrorResponse); ok && err.Response.StatusCode == 404 {
		return false, nil
	}
	return false, err
}

// DeleteBranch deletes a branch by its name and project ID
func (gc *GitlabClient) DeleteBranch(projectID, branchName string) error {

	_, err := gc.client.Branches.DeleteBranch(projectID, branchName)
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %v", branchName, err)
	}

	fmt.Printf("Deleted branch: %s", branchName)

	return nil
}

// CreateGitlabNewBranch creates a new branch
func (gc *GitlabClient) CreateGitlabNewBranch(projectID, branchName, sha, baseBranch string) error {

	// If sha is not provided, get the latest commit from the base branch
	if sha == "" {
		commit, _, err := gc.client.Commits.GetCommit(projectID, baseBranch)
		if err != nil {
			return fmt.Errorf("failed to get latest commit from base branch: %v", err)
		}
		sha = commit.ID
	}

	opt := &gitlab.CreateBranchOptions{
		Branch: &branchName,
		Ref:    &sha,
	}
	_, resp, err := gc.client.Branches.CreateBranch(projectID, opt)
	if err != nil {
		// Check if the error is due to the branch already existing
		if resp != nil && resp.StatusCode == http.StatusConflict {
			return fmt.Errorf("branch '%s' already exists", branchName)
		}
		return fmt.Errorf("failed to create branch '%s': %v", branchName, err)
	}

	return nil
}

// GetMergeRequests returns a list of all MergeRequests in a given project ID and repository name
func (gc *GitlabClient) GetMergeRequests() ([]*gitlab.MergeRequest, error) {

	// Get merge requests using Gitlab client
	mergeRequests, _, err := gc.client.MergeRequests.ListMergeRequests(&gitlab.ListMergeRequestsOptions{})
	if err != nil {
		// Handle error
		return nil, err
	}

	return mergeRequests, nil
}

// CloseMergeRequest closes merge request in Gitlab repo by given MR IID
func (gc *GitlabClient) CloseMergeRequest(projectID string, mergeRequestIID int) error {

	// Get merge requests using Gitlab client
	_, _, err := gc.client.MergeRequests.GetMergeRequest(projectID, mergeRequestIID, nil)
	if err != nil {
		return fmt.Errorf("failed to get MR of IID %d in projectID %s, %v", mergeRequestIID, projectID, err)
	}

	_, _, err = gc.client.MergeRequests.UpdateMergeRequest(projectID, mergeRequestIID, &gitlab.UpdateMergeRequestOptions{
		StateEvent: gitlab.Ptr("close"),
	})
	if err != nil {
		return fmt.Errorf("failed to close MR of IID %d in projectID %s, %v", mergeRequestIID, projectID, err)
	}

	return nil
}
