//
// Copyright 2022 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitops

import (
	"fmt"

	"github.com/redhat-developer/gitops-generator/pkg/util"
)

type GitCmd string

const (
	cloneRepo      GitCmd = "clone git"
	checkGitDiff   GitCmd = "check git diff"
	commitFiles    GitCmd = "commit files"
	pushRemote     GitCmd = "push remote"
	initializeGit  GitCmd = "initialize git"
	addComponents  GitCmd = "add components"
	getCommitID    GitCmd = "retrieve commit id"
	switchBranch   GitCmd = "switch to"
	checkoutBranch GitCmd = "checkout"
	genOverlays    GitCmd = "overlays dir"
)

// GitCmdError is used to construct custom errors for a number of git commands that follow similar message patterns
// Used by the following command types:  cloneRepo, checkGitDiff, commitFiles, pushRemote, initializeGit, addComponents, getCommitID

type GitCmdError struct {
	path      string
	cmdResult string
	err       error
	cmdType   GitCmd
}

func (e *GitCmdError) Error() string {

	// Check to see what gitops actions have take place in order to append the correct preposition
	cmdMsg := e.cmdType
	if e.cmdType == checkGitDiff {
		cmdMsg = cmdMsg + " in"
	} else if e.cmdType == commitFiles || e.cmdType == pushRemote || e.cmdType == addComponents {
		cmdMsg = cmdMsg + " to"
	} else if e.cmdType == getCommitID {
		cmdMsg = cmdMsg + " for"
	}

	return util.SanitizeErrorMessage(fmt.Errorf("failed to %s repository %q %q: %s", cmdMsg, e.path, e.cmdResult, e.err)).Error()
}

// GitBranchError is used to construct custom errors related to git branch failures
// Used by the following command types: switchBranch, checkoutBranch
type GitBranchError struct {
	branch    string
	repoPath  string
	cmdResult string
	err       error
	cmdType   GitCmd
}

func (e *GitBranchError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("failed to %s branch %q in repository %q %q: %s", e.cmdType, e.branch, e.repoPath, string(e.cmdResult), e.err)).Error()
}

// GitPullError is used to construct custom errors related to git pull failures
type GitPullError struct {
	remote    string
	cmdResult string
	err       error
}

func (e *GitPullError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("failed to pull from remote %q %q: %s", e.remote, string(e.cmdResult), e.err)).Error()
}

// GitLsRemoteError is used to construct custom errors related to git ls-remote failures
type GitLsRemoteError struct {
	remote    string
	cmdResult string
	err       error
}

func (e *GitLsRemoteError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("failed to list git remotes for remote %q %q: %s", e.remote, string(e.cmdResult), e.err)).Error()
}

type GitGenResourcesAndOverlaysError struct {
	path          string
	componentName string
	err           error
	cmdType       GitCmd
}

func (e *GitGenResourcesAndOverlaysError) Error() string {
	failedToGenerate := "failed to generate the gitops resources in "
	if e.cmdType == genOverlays {
		failedToGenerate = failedToGenerate + string(genOverlays) + " "
	}
	return util.SanitizeErrorMessage(fmt.Errorf(failedToGenerate+"%q for component %q: %s", e.path, e.componentName, e.err)).Error()
}

// DeleteFolderError is used to construct a custom error if component removal fails
type DeleteFolderError struct {
	componentPath string
	repoPath      string
	cmdResult     string
	err           error
}

func (e *DeleteFolderError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("failed to delete %q folder in repository in %q %q: %s", e.componentPath, e.repoPath, e.cmdResult, e.err)).Error()
}

// GitCreateRepoError is used to construct a custom error if repo creation fails
type GitCreateRepoError struct {
	repoName string
	org      string
	err      error
}

func (e *GitCreateRepoError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("failed to create repository %q in namespace %q: %w", e.repoName, e.org, e.err)).Error()
}

// GitAddFilesToRemoteError is used to construct a custom error if adding files to remote repo fails
type GitAddFilesToRemoteError struct {
	componentName string
	remoteURL     string
	repoPath      string
	cmdResult     string
	err           error
}

func (e *GitAddFilesToRemoteError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("failed to add files for component %q, to remote 'origin' %q to repository in %q %q: %s", e.componentName, e.remoteURL, e.repoPath, e.cmdResult, e.err)).Error()
}

type GitAddFilesError struct {
	componentName string
	repoPath      string
	cmdResult     string
	err           error
}

func (e *GitAddFilesError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("failed to add files for component %q to repository in %q %q: %s", e.componentName, e.repoPath, e.cmdResult, e.err)).Error()
}

type GitOpsRepoGenError struct {
	gitopsURL string
	errMsg    string
	err       error
}

func (e *GitOpsRepoGenError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf(e.errMsg, e.gitopsURL, e.err)).Error()
}

type GitOpsRepoGenUserError struct {
	err error
}

func (e *GitOpsRepoGenUserError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("failed to get the user with their auth token: %w", e.err)).Error()
}
