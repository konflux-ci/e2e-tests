package gitlab

import (
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	gitlab "github.com/xanzy/go-gitlab"
)

type Gitlab struct {
	client *gitlab.Client
}

// CreateRef creates a new ref (GitLab branch) in a specified GitLab repository,
// that will be based on the commit specified with sha. If sha is not specified
// the latest commit from base branch will be used.
// CreateBranch creates a new branch in a GitLab project with the given projectID and newBranchName
func (gc *GitlabClient) CreateBranch(projectID, newBranchName, defaultBranch string) error {
	// Prepare the branch creation request
	branchOpts := &gitlab.CreateBranchOptions{
		Branch: gitlab.String(newBranchName),
		Ref:    gitlab.String(defaultBranch),
	}

	// Perform the branch creation
	_, _, err := gc.client.Branches.CreateBranch(projectID, branchOpts)
	if err != nil {
		return fmt.Errorf("failed to create branch %s in project %s: %w", newBranchName, projectID, err)
	}

	// Wait for the branch to actually exist
	Eventually(func(gomega Gomega) {
		exist, err := gc.ExistsRef(projectID, newBranchName)
		gomega.Expect(err).NotTo(HaveOccurred())
		gomega.Expect(exist).To(BeTrue())

	}, 2*time.Minute, 2*time.Second).Should(Succeed())

	return nil
}

// ExistsRef checks if a branch exists in a specified GitLab repository.
func (gc *GitlabClient) ExistsRef(projectID, branchName string) (bool, error) {

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

// DeleteAllBranchesOfProjectID deletes all branches in a given projectID , it delets the first 20.
func (gc *GitlabClient) DeleteAllBranchesOfProjectID(projectID string) (bool, error) {

	// List branches for the project
	branches, _, err := gc.client.Branches.ListBranches(projectID, &gitlab.ListBranchesOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list branches: %v", err)
	}

	// Delete each branch
	for _, branch := range branches {
		if _, err := gc.client.Branches.DeleteBranch(projectID, branch.Name); err != nil {
			fmt.Printf("failed to delete branch %s: %v", branch.Name, err)
		} else {
			fmt.Printf("Deleted branch: %s", branch.Name)
		}
	}
	return true, nil
}

// DeleteBranch deletes a branch  by its name from a specific project ID
func (gc *GitlabClient) DeleteBranch(projectID, branchName string) (bool, error) {

	_, err := gc.client.Branches.DeleteBranch(projectID, branchName)
	if err != nil {
		return false, fmt.Errorf("failed to delete branch %s: %v", branchName, err)
	}

	fmt.Printf("Deleted branch: %s", branchName)

	return true, nil
}
