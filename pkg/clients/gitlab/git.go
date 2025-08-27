package gitlab

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	"github.com/xanzy/go-gitlab"

	utils "github.com/konflux-ci/e2e-tests/pkg/utils"
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
		commit, _, err := gc.client.Commits.GetCommit(projectID, baseBranch, &gitlab.GetCommitOptions{})
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
	mergeRequests, _, err := gc.client.MergeRequests.ListMergeRequests(&gitlab.ListMergeRequestsOptions{State: gitlab.Ptr("opened")})
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

// DeleteWebhooks deletes webhooks in Gitlab repo by given project ID,
// and if the webhook URL contains the cluster's domain name.
func (gc *GitlabClient) DeleteWebhooks(projectID, clusterAppDomain string) error {

	// Check if clusterAppDomain is empty returns error, else continue
	if clusterAppDomain == "" {
		return fmt.Errorf("Framework.ClusterAppDomain is empty")
	}

	// List project hooks
	webhooks, _, err := gc.client.Projects.ListProjectHooks(projectID, nil)
	if err != nil {
		return fmt.Errorf("failed to list project hooks for project id: %s with error: %v", projectID, err)
	}

	// Delete matching webhooks
	for _, webhook := range webhooks {
		if strings.Contains(webhook.URL, clusterAppDomain) {
			if _, err := gc.client.Projects.DeleteProjectHook(projectID, webhook.ID); err != nil {
				return fmt.Errorf("failed to delete webhook (ID: %d): %v", webhook.ID, err)
			}
			break
		}
	}

	return nil
}

func (gc *GitlabClient) CreateFile(projectId, pathToFile, fileContent, branchName string) (*gitlab.FileInfo, error) {
	opts := &gitlab.CreateFileOptions{
		Branch:        gitlab.Ptr(branchName),
		Content:       &fileContent,
		CommitMessage: gitlab.Ptr("e2e test commit message"),
	}

	file, resp, err := gc.client.RepositoryFiles.CreateFile(projectId, pathToFile, opts)
	if resp.StatusCode != 201 || err != nil {
		return nil, fmt.Errorf("error when creating file contents: response (%v) and error: %v", resp, err)
	}

	return file, nil
}

func (gc *GitlabClient) GetFile(projectId, pathToFile, branchName string) (string, error) {
	file, _, err := gc.client.RepositoryFiles.GetFile(projectId, pathToFile, gitlab.Ptr(gitlab.GetFileOptions{Ref: gitlab.Ptr(branchName)}))
	if err != nil {
		return "", fmt.Errorf("Failed to get file: %v", err)
	}

	decodedContent, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		return "", fmt.Errorf("Failed to decode file content: %v", err)
	}
	fileContentString := string(decodedContent)

	return fileContentString, nil
}

func (gc *GitlabClient) GetFileMetaData(projectID, pathToFile, branchName string) (*gitlab.File, error) {
	metadata, _, err := gc.client.RepositoryFiles.GetFileMetaData(projectID, pathToFile, gitlab.Ptr(gitlab.GetFileMetaDataOptions{Ref: gitlab.Ptr(branchName)}))
	return metadata, err
}

func (gc *GitlabClient) UpdateFile(projectId, pathToFile, fileContent, branchName string) (string, error) {
	updateOptions := &gitlab.UpdateFileOptions{
		Branch:        gitlab.Ptr(branchName),
		Content:       gitlab.Ptr(fileContent),
		CommitMessage: gitlab.Ptr("e2e test commit message"),
	}

	_, _, err := gc.client.RepositoryFiles.UpdateFile(projectId, pathToFile, updateOptions)
	if err != nil {
		return "", fmt.Errorf("Failed to update/create file: %v", err)
	}

	// Well, this is not atomic, but best I figured.
	file, _, err := gc.client.RepositoryFiles.GetFile(projectId, pathToFile, gitlab.Ptr(gitlab.GetFileOptions{Ref: gitlab.Ptr(branchName)}))
	if err != nil {
		return "", fmt.Errorf("Failed to get file: %v", err)
	}

	return file.CommitID, nil
}

func (gc *GitlabClient) AcceptMergeRequest(projectID string, mrID int) (*gitlab.MergeRequest, error) {
	mr, _, err := gc.client.MergeRequests.AcceptMergeRequest(projectID, mrID, nil)
	return mr, err
}

func (gc *GitlabClient) GetCommitStatusConclusion(statusName, projectID, commitSHA string, mergeRequestID int) string {
	var matchingStatus *gitlab.CommitStatus
	timeout := time.Minute * 10

	Eventually(func() bool {
		statuses, _, err := gc.client.Commits.GetCommitStatuses(projectID, commitSHA, &gitlab.GetCommitStatusesOptions{})
		if err != nil {
			fmt.Printf("got error when listing commit statuses: %+v\n", err)
			return false
		}
		for _, status := range statuses {
			if strings.Contains(status.Name, statusName) {
				matchingStatus = status
				return true
			}
		}
		return false
	}, timeout, time.Second*2).Should(BeTrue(), fmt.Sprintf("timed out waiting for the PaC commit status to appear for %s", commitSHA))

	Eventually(func() bool {
		statuses, _, err := gc.client.Commits.GetCommitStatuses(projectID, commitSHA, &gitlab.GetCommitStatusesOptions{})
		if err != nil {
			fmt.Printf("got error when checking commit status: %+v\n", err)
			return false
		}
		for _, status := range statuses {
			if strings.Contains(status.Name, statusName) {
				currentState := status.Status
				if currentState != "pending" && currentState != "running" {
					matchingStatus = status
					return true
				}
				fmt.Printf("expecting commit status to be completed, got: %s\n", currentState)
				return false
			}
		}
		return false
	}, timeout, time.Second*2).Should(BeTrue(), fmt.Sprintf("timed out waiting for the PaC commit status to be completed for %s", commitSHA))

	return matchingStatus.Status
}

// DeleteRepositoryIfExists deletes a GitLab repository if it exists.
// Returns an error if the deletion fails except for project not being found (404).
func (gc *GitlabClient) DeleteRepositoryIfExists(projectID string) error {
	getProj, getResp, getErr := gc.client.Projects.GetProject(projectID, nil)
	if getErr != nil {
		if getResp != nil && getResp.StatusCode == http.StatusNotFound {
			return nil
		} else {
			return fmt.Errorf("Error getting project %s: %v", projectID, getErr)
		}
	}
	if getProj.PathWithNamespace != projectID && strings.Contains(getProj.PathWithNamespace, projectID + "-deleted-") {
		// We asked for repo like "jhutar/nodejs-devfile-sample7-ocpp01v1-konflux-perfscale"
		// and got "jhutar/nodejs-devfile-sample7-ocpp01v1-konflux-perfscale-deleted-138805"
		// and that means repo was moved by being deleted for a first
		// time, entering a grace period.

		// Now we need to delete the repository for a second time to limit
		// number of repos we keep behind as per request in INC3755661
		err := gc.DeleteRepositoryReally(getProj.PathWithNamespace)
		return err
	}

	resp, err := gc.client.Projects.DeleteProject(projectID, &gitlab.DeleteProjectOptions{})

	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("Error deleting project %s: %w", projectID, err)
	}

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Unexpected status code when deleting project %s: %d", projectID, resp.StatusCode)
	}

	err = utils.WaitUntilWithInterval(func() (done bool, err error) {
		getProj, getResp, getErr := gc.client.Projects.GetProject(projectID, nil)

		if getErr != nil {
			if getResp != nil && getResp.StatusCode == http.StatusNotFound {
				return true, nil
			} else {
				return false, getErr
			}
		}

		if getProj.PathWithNamespace != projectID && strings.Contains(getProj.PathWithNamespace, projectID + "-deleted-") {
			errDel := gc.DeleteRepositoryReally(getProj.PathWithNamespace)
			if errDel != nil {
				return false, errDel
			}
			return true, nil
		}

		fmt.Printf("Repo %s still exists: %v\n", projectID, getResp)
		return false, nil
	}, time.Second * 10, time.Minute * 5)

	return err
}

// GitLab have a concept of two deletes. First one just renames the repo,
// and only second one really deletes it. DeleteRepositoryReally is meant for
// the second deletition.
func (gc *GitlabClient) DeleteRepositoryReally(projectID string) error {
	opts := &gitlab.DeleteProjectOptions{
		FullPath: gitlab.Ptr(projectID),
		PermanentlyRemove: gitlab.Ptr(true),
	}
	_, err := gc.client.Projects.DeleteProject(projectID, opts)
	if err != nil {
		return fmt.Errorf("Error on permanently deleting project %s: %w", projectID, err)
	}
	return nil
}

// ForkRepository forks a source GitLab repository to a target repository.
// Returns the newly forked repository and an error if the operation fails.
func (gc *GitlabClient) ForkRepository(sourceOrgName, sourceName, targetOrgName, targetName string) (*gitlab.Project, error) {
	var forkedProject *gitlab.Project
	var resp *gitlab.Response
	var err error

	sourceProjectID := sourceOrgName + "/" + sourceName
	targetProjectID := targetOrgName + "/" + targetName

	opts := &gitlab.ForkProjectOptions{
		Name: gitlab.Ptr(targetName),
		NamespacePath: gitlab.Ptr(targetOrgName),
		Path: gitlab.Ptr(targetName),
	}

	err = utils.WaitUntilWithInterval(func() (done bool, err error) {
		forkedProject, resp, err = gc.client.Projects.ForkProject(sourceProjectID, opts)
		if err != nil {
			fmt.Printf("Failed to fork %s, trying again: %v\n", sourceProjectID, err)
			return false, nil
		}
		return true, nil
	}, time.Second * 10, time.Minute * 5)
	if err != nil {
		return nil, fmt.Errorf("Error forking project %s to %s: %w", sourceProjectID, targetProjectID, err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("Unexpected status code when forking project %s: %d", sourceProjectID, resp.StatusCode)
	}

	err = utils.WaitUntilWithInterval(func() (done bool, err error) {
		var getErr error

		forkedProject, _, getErr = gc.client.Projects.GetProject(forkedProject.ID, nil)
		if getErr != nil {
			return false, fmt.Errorf("Error getting forked project status for %s (ID: %d): %w", forkedProject.Name, forkedProject.ID, getErr)
		}

		if forkedProject.ImportStatus == "finished" {
			return true, nil
		} else if forkedProject.ImportStatus == "failed" || forkedProject.ImportStatus == "timeout" {
			return false, fmt.Errorf("Forking of project %s (ID: %d) failed with import status: %s", forkedProject.Name, forkedProject.ID, forkedProject.ImportStatus)
		}

		return false, nil
	}, time.Second * 10, time.Minute * 10)

	if err != nil {
		return nil, fmt.Errorf("Error waiting for project %s (ID: %d) fork to complete: %w", targetProjectID, forkedProject.ID, err)
	}

	return forkedProject, nil
}
