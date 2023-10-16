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

package gitlab

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/xanzy/go-gitlab"

	"github.com/redhat-appstudio/build-service/pkg/boerrors"
	gp "github.com/redhat-appstudio/build-service/pkg/git/gitprovider"
)

func getProjectPathFromRepoUrl(repoUrl string) string {
	// https://gitlab.com/namespace/projectname
	gitSourceUrlParts := strings.Split(strings.TrimSuffix(repoUrl, ".git"), "/")
	gitlabNamespace := gitSourceUrlParts[3]
	gitlabProjectName := gitSourceUrlParts[4]
	projectPath := gitlabNamespace + "/" + gitlabProjectName
	return projectPath
}

// refineGitHostingServiceError generates expected permanent error from GitHub response.
// If no one is detected, the original error will be returned.
// refineGitHostingServiceError should be called just after every GitHub API call.
func refineGitHostingServiceError(response *http.Response, originErr error) error {
	// go-gitlab APIs do not return a http.Response object if the error is not related to an HTTP request.
	if response == nil {
		return originErr
	}
	switch response.StatusCode {
	case 401:
		return boerrors.NewBuildOpError(boerrors.EGitLabTokenUnauthorized, originErr)
	case 403:
		return boerrors.NewBuildOpError(boerrors.EGitLabTokenInsufficientScope, originErr)
	default:
		return originErr
	}
}

func (g *GitlabClient) getBranch(projectPath, branchName string) (*gitlab.Branch, error) {
	branch, resp, err := g.client.Branches.GetBranch(projectPath, branchName)
	if err != nil {
		if resp.StatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}
	return branch, nil
}

func (g *GitlabClient) branchExist(projectPath, branchName string) (bool, error) {
	_, resp, err := g.client.Branches.GetBranch(projectPath, branchName)
	if err != nil {
		if resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (g *GitlabClient) createBranch(projectPath, branchName, baseBranchName string) error {
	opts := &gitlab.CreateBranchOptions{
		Branch: &branchName,
		Ref:    &baseBranchName,
	}
	_, _, err := g.client.Branches.CreateBranch(projectPath, opts)
	return err
}

func (g *GitlabClient) deleteBranch(projectPath, branch string) (bool, error) {
	if resp, err := g.client.Branches.DeleteBranch(projectPath, branch); err != nil {
		if resp.Response.StatusCode == 404 {
			// The given branch doesn't exist
			return false, nil
		}
		return false, refineGitHostingServiceError(resp.Response, err)
	}
	return true, nil
}

func (g *GitlabClient) getDefaultBranch(projectPath string) (string, error) {
	projectInfo, _, err := g.client.Projects.GetProject(projectPath, nil)
	if err != nil {
		return "", err
	}
	if projectInfo == nil {
		return "", fmt.Errorf("project info is empty in GitLab API response")
	}
	return projectInfo.DefaultBranch, nil
}

func (g *GitlabClient) filesUpToDate(projectPath, branchName string, files []gp.RepositoryFile) (bool, error) {
	for _, file := range files {
		opts := &gitlab.GetRawFileOptions{
			Ref: &branchName,
		}
		fileContent, resp, err := g.client.RepositoryFiles.GetRawFile(projectPath, file.FullPath, opts)
		if err != nil {
			if resp.StatusCode != 404 {
				return false, err
			}
			return false, nil
		}
		if !bytes.Equal(fileContent, file.Content) {
			return false, nil
		}
	}
	return true, nil
}

// filesExistInDirectory checks if given files exist under specified directory.
// Returns subset of given files which exist.
func (g *GitlabClient) filesExistInDirectory(projectPath, branchName, directoryPath string, files []gp.RepositoryFile) ([]gp.RepositoryFile, error) {
	existingFiles := make([]gp.RepositoryFile, 0, len(files))

	opts := &gitlab.ListTreeOptions{
		Ref:         &branchName,
		Path:        &directoryPath,
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	dirContent, resp, err := g.client.Repositories.ListTree(projectPath, opts)
	if err != nil {
		if resp.StatusCode == 404 {
			return existingFiles, nil
		}
		return existingFiles, err
	}

	for _, file := range dirContent {
		for _, f := range files {
			if file.Path == f.FullPath {
				existingFiles = append(existingFiles, gp.RepositoryFile{FullPath: file.Path})
				break
			}
		}
	}

	return existingFiles, nil
}

func (g *GitlabClient) commitFilesIntoBranch(projectPath, branchName, commitMessage, authorName, authorEmail string, files []gp.RepositoryFile) error {
	actions := []*gitlab.CommitActionOptions{}
	for _, file := range files {
		filePath := file.FullPath
		content := string(file.Content)
		var fileAction gitlab.FileActionValue

		// Detect file action: update or create
		opts := &gitlab.GetRawFileOptions{Ref: &branchName}
		_, resp, err := g.client.RepositoryFiles.GetRawFile(projectPath, file.FullPath, opts)
		if err != nil {
			if resp.StatusCode != 404 {
				return err
			}
			fileAction = gitlab.FileCreate
		} else {
			fileAction = gitlab.FileUpdate
		}

		action := &gitlab.CommitActionOptions{
			Action:   &fileAction,
			FilePath: &filePath,
			Content:  &content,
		}

		actions = append(actions, action)
	}

	opts := &gitlab.CreateCommitOptions{
		Branch:        &branchName,
		CommitMessage: &commitMessage,
		AuthorName:    &authorName,
		AuthorEmail:   &authorEmail,
		Actions:       actions,
	}
	_, _, err := g.client.Commits.CreateCommit(projectPath, opts)
	return err
}

// Creates commit into specified branch that deletes given files.
func (g *GitlabClient) addDeleteCommitToBranch(projectPath, branchName, authorName, authorEmail, commitMessage string, files []gp.RepositoryFile) error {
	actions := []*gitlab.CommitActionOptions{}
	fileActionType := gitlab.FileDelete
	for _, file := range files {
		filePath := file.FullPath
		actions = append(actions, &gitlab.CommitActionOptions{
			Action:   &fileActionType,
			FilePath: &filePath,
		})
	}

	opts := &gitlab.CreateCommitOptions{
		Branch:        &branchName,
		CommitMessage: &commitMessage,
		AuthorName:    &authorName,
		AuthorEmail:   &authorEmail,
		Actions:       actions,
	}
	_, _, err := g.client.Commits.CreateCommit(projectPath, opts)
	return err
}

func (g *GitlabClient) diffNotEmpty(projectPath, branchName, baseBranchName string) (bool, error) {
	straight := false
	opts := &gitlab.CompareOptions{
		From:     &baseBranchName,
		To:       &branchName,
		Straight: &straight,
	}
	cmpres, _, err := g.client.Repositories.Compare(projectPath, opts)
	if err != nil {
		return false, err
	}
	return len(cmpres.Diffs) > 0, nil
}

func (g *GitlabClient) findMergeRequestByBranches(projectPath, branch, targetBranch string) (*gitlab.MergeRequest, error) {
	openedState := "opened"
	viewType := "simple"
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State:        &openedState,
		SourceBranch: &branch,
		TargetBranch: &targetBranch,
		View:         &viewType,
		ListOptions:  gitlab.ListOptions{PerPage: 100},
	}
	mrs, _, err := g.client.MergeRequests.ListProjectMergeRequests(projectPath, opts)
	if err != nil {
		return nil, err
	}
	switch len(mrs) {
	case 0:
		return nil, nil
	case 1:
		return mrs[0], nil
	default:
		return nil, fmt.Errorf("failed to find merge request by branch: %d matches found", len(mrs))
	}
}

func (g *GitlabClient) createMergeRequestWithinRepository(projectPath, branchName, baseBranchName, mrTitle, mrText string) (string, error) {
	opts := &gitlab.CreateMergeRequestOptions{
		SourceBranch: &branchName,
		TargetBranch: &baseBranchName,
		Title:        &mrTitle,
		Description:  &mrText,
	}
	mr, _, err := g.client.MergeRequests.CreateMergeRequest(projectPath, opts)
	if err != nil {
		return "", err
	}
	return mr.WebURL, nil
}

func (g *GitlabClient) getWebhookByTargetUrl(projectPath, webhookTargetUrl string) (*gitlab.ProjectHook, error) {
	opts := &gitlab.ListProjectHooksOptions{PerPage: 100}
	webhooks, resp, err := g.client.Projects.ListProjectHooks(projectPath, opts)
	if err != nil {
		return nil, refineGitHostingServiceError(resp.Response, err)
	}
	for _, webhook := range webhooks {
		if webhook.URL == webhookTargetUrl {
			return webhook, nil
		}
	}
	// Webhook with the given URL not found
	return nil, nil
}

func (g *GitlabClient) createPaCWebhook(projectPath, webhookTargetUrl, webhookSecret string) (*gitlab.ProjectHook, error) {
	opts := getPaCWebhookOpts(webhookTargetUrl, webhookSecret)
	hook, resp, err := g.client.Projects.AddProjectHook(projectPath, opts)
	return hook, refineGitHostingServiceError(resp.Response, err)
}

func (g *GitlabClient) updatePaCWebhook(projectPath string, webhookId int, webhookTargetUrl, webhookSecret string) (*gitlab.ProjectHook, error) {
	opts := gitlab.EditProjectHookOptions(*getPaCWebhookOpts(webhookTargetUrl, webhookSecret))
	hook, resp, err := g.client.Projects.EditProjectHook(projectPath, webhookId, &opts)
	return hook, refineGitHostingServiceError(resp.Response, err)
}

func (g *GitlabClient) deleteWebhook(projectPath string, webhookId int) error {
	resp, err := g.client.Projects.DeleteProjectHook(projectPath, webhookId)
	if resp.StatusCode == 404 {
		return nil
	}
	return refineGitHostingServiceError(resp.Response, err)
}

func getPaCWebhookOpts(webhookTargetUrl, webhookSecret string) *gitlab.AddProjectHookOptions {
	enableSSLVerification := !gp.IsInsecureSSL()

	mergeRequestsEvents := true
	pushEvents := true
	noteEvents := true

	return &gitlab.AddProjectHookOptions{
		URL:                   &webhookTargetUrl,
		Token:                 &webhookSecret,
		EnableSSLVerification: &enableSSLVerification,
		MergeRequestsEvents:   &mergeRequestsEvents,
		PushEvents:            &pushEvents,
		NoteEvents:            &noteEvents,
	}
}
