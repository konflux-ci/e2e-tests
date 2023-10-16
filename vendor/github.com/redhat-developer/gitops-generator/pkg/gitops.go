//
// Copyright 2021-2022 Red Hat, Inc.
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
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/redhat-developer/gitops-generator/pkg/util"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/factory"

	gitopsv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	"github.com/spf13/afero"
)

const defaultRepoDescription = "Bootstrapped GitOps Repository based on Components"

type CommandType string

const (
	GitCommand        CommandType = "git"
	RmCommand         CommandType = "rm"
	unsupportedCmdMsg             = "Unsupported command \"%s\" "
)

type Generator interface {
	CloneGenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, context string, doPush bool) error
	CommitAndPush(outputPath string, repoPathOverride string, remote string, componentName string, branch string, commitMessage string) error
	GenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, doPush bool, createdBy string) error
	GenerateOverlaysAndPush(outputPath string, clone bool, remote string, options gitopsv1alpha1.GeneratorOptions, applicationName, environmentName, imageName, namespace string, appFs afero.Afero, branch string, context string, doPush bool, componentGeneratedResources map[string][]string) error
	GitRemoveComponent(outputPath string, remote string, componentName string, branch string, context string) error
	CloneRepo(outputPath string, remote string, componentName string, branch string) error
	GetCommitIDFromRepo(fs afero.Afero, repoPath string) (string, error)
}

// NewGitopsGen returns a Generator implementation
func NewGitopsGen() Gen {
	return Gen{
		Log: zap.New(zap.UseFlagOptions(&zap.Options{
			Development: true,
			TimeEncoder: zapcore.ISO8601TimeEncoder,
		})),
	}
}

func NewGitopsGenWithLogger(log logr.Logger) Gen {
	return Gen{
		Log: log,
	}
}

type Gen struct {
	Log logr.Logger
}

// expose as a global variable for the purpose of running mock tests
// only "git" and "rm" are supported
/* #nosec G204 -- used internally to execute various gitops actions and eventual cleanup of artifacts.  Calling methods validate user input to ensure commands are used appropriately */
var execute = func(baseDir string, cmd CommandType, args ...string) ([]byte, error) {
	if cmd == GitCommand || cmd == RmCommand {
		c := exec.Command(string(cmd), args...)
		c.Dir = baseDir
		output, err := c.CombinedOutput()
		return output, err
	}

	return []byte(""), fmt.Errorf(unsupportedCmdMsg, string(cmd))
}

// CloneGenerateAndPush takes in the following args and generates the gitops resources for a given component
// 1. outputPath: Where to output the gitops resources to
// 2. remote: A string of the form https://$token@<domain>/<org>/<repo>, where <domain> is either github.com or gitlab.com and $token is optional. Corresponds to the component's gitops repository
// 3. options: Options for resource generation
// 4. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 5. The branch to push to
// 6. The path within the repository to generate the resources in
// 7. The gitops config containing the build bundle;
// Adapted from https://github.com/redhat-developer/kam/blob/master/pkg/pipelines/utils.go#L79
func (s Gen) CloneGenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, context string, doPush bool) error {
	componentName := options.Name

	invalidRemoteErr := util.ValidateRemote(remote)
	if invalidRemoteErr != nil {
		return invalidRemoteErr
	}

	s.Log.V(6).Info("Cloning GitOps repository")
	if out, err := execute(outputPath, GitCommand, "clone", remote, componentName); err != nil {
		return &GitCmdError{path: outputPath, cmdResult: string(out), err: err, cmdType: cloneRepo}
	}
	s.Log.V(6).Info("GitOps repository cloned")

	repoPath := filepath.Join(outputPath, componentName)
	gitopsFolder := filepath.Join(repoPath, context)
	componentPath := filepath.Join(gitopsFolder, "components", componentName, "base")

	// Checkout the specified branch
	s.Log.V(6).Info(fmt.Sprintf("Checking out branch %s", branch))
	if _, err := execute(repoPath, GitCommand, "switch", branch); err != nil {
		if out, err := execute(repoPath, GitCommand, "checkout", "-b", branch); err != nil {
			return &GitBranchError{branch: branch, repoPath: repoPath, cmdResult: string(out), err: err, cmdType: checkoutBranch}
		}
	}
	s.Log.V(6).Info(fmt.Sprintf("Branch %s checked out", branch))

	if out, err := execute(repoPath, RmCommand, "-rf", filepath.Join("components", componentName, "base")); err != nil {
		return &DeleteFolderError{componentPath: filepath.Join("components", componentName, "base"), repoPath: repoPath, cmdResult: string(out), err: err}
	}

	// Generate the gitops resources and update the parent kustomize yaml file
	s.Log.V(6).Info(fmt.Sprintf("Generating GitOps resources under %s", componentPath))
	if err := Generate(appFs, gitopsFolder, componentPath, options); err != nil {
		return &GitGenResourcesAndOverlaysError{path: componentPath, componentName: componentName, err: err}
	}
	s.Log.V(6).Info(fmt.Sprintf("GitOps resources generated under %s", componentPath))

	if doPush {
		s.Log.V(6).Info("Pushing GitOps resources to repository")
		return s.CommitAndPush(outputPath, "", remote, componentName, branch, fmt.Sprintf("Generate GitOps base resources for component %s", componentName))
	}
	return nil
}

// CommitAndPush pushes any new changes to the GitOps repo.  The folder should already be cloned in the target output folder.
// 1. outputPath: Where the gitops resources are
// 2. repoPathOverride: The default path is the componentName. Use this to override the default folder.
// 3. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 4. componentName: The component name corresponding to a single Component in an Application in AS. eg. component.Name
// 5. The branch to push to
// 6. The path within the repository to generate the resources in
func (s Gen) CommitAndPush(outputPath string, repoPathOverride string, remote string, componentName string, branch string, commitMessage string) error {

	invalidRemoteErr := util.ValidateRemote(remote)
	if invalidRemoteErr != nil {
		return invalidRemoteErr
	}

	repoPath := filepath.Join(outputPath, componentName)
	if repoPathOverride != "" {
		repoPath = filepath.Join(outputPath, repoPathOverride)
	}

	if out, err := execute(repoPath, GitCommand, "add", "."); err != nil {
		return &GitAddFilesError{componentName: componentName, repoPath: repoPath, cmdResult: string(out), err: err}
	}

	if out, err := execute(repoPath, GitCommand, "--no-pager", "diff", "--cached"); err != nil {
		return &GitCmdError{path: repoPath, cmdResult: string(out), err: err, cmdType: checkGitDiff}

	} else if string(out) != "" {
		// Pull from remote if branch is present
		if out, err := execute(repoPath, GitCommand, "ls-remote", "--heads", remote, branch); err != nil {
			return &GitLsRemoteError{err: err, cmdResult: string(out), remote: remote}
		} else if strings.Contains(string(out), "refs/heads/"+branch) {
			// only if the git repository contains the branch, pull
			if out, err := execute(repoPath, GitCommand, "pull"); err != nil {
				return &GitPullError{err: err, cmdResult: string(out), remote: remote}
			}
		}

		// Commit the changes and push
		if out, err := execute(repoPath, GitCommand, "commit", "-m", commitMessage); err != nil {
			return &GitCmdError{path: repoPath, cmdResult: string(out), err: err, cmdType: commitFiles}
		}
		if out, err := execute(repoPath, GitCommand, "push", "origin", branch); err != nil {
			return &GitCmdError{path: remote, cmdResult: string(out), err: err, cmdType: pushRemote}
		}
	}

	return nil
}

// GenerateAndPush generates a new gitops folder with one component, and optionally pushes to Git. Note: this does not
// clone an existing gitops repo.
// 1. outputPath: Where the gitops resources are
// 2. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 3. options: Options for resource generation
// 4. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 5. The branch to push to
// 6. Optionally push to the GitOps repository or not.  Default is not to push.
// 7. createdBy: Use a unique name to identify that clients are generating the GitOps repository. Default is "application-service" and should be overwritten.
func (s Gen) GenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, doPush bool, createdBy string) error {
	CreatedBy = createdBy
	componentName := options.Name
	repoPath := filepath.Join(outputPath, options.Application)

	// Generate the gitops resources and update the parent kustomize yaml file
	gitopsFolder := repoPath

	gitHostAccessToken := options.Secret
	componentPath := filepath.Join(gitopsFolder, "components", componentName, "base")
	if err := Generate(appFs, gitopsFolder, componentPath, options); err != nil {
		return &GitGenResourcesAndOverlaysError{path: componentPath, componentName: componentName, err: err}
	}

	// Commit the changes and push
	if doPush {

		invalidRemoteErr := util.ValidateRemote(remote)
		if invalidRemoteErr != nil {
			return invalidRemoteErr
		}

		gitOpsRepoURL := ""
		if options.GitSource != nil {
			gitOpsRepoURL = options.GitSource.URL
		}
		if gitOpsRepoURL == "" {
			return fmt.Errorf("the GitOps repo URL is not set")
		}
		u, err := url.Parse(gitOpsRepoURL)
		if err != nil {
			return &GitOpsRepoGenError{gitopsURL: gitOpsRepoURL, errMsg: "failed to parse GitOps repo URL %q: %w", err: err}
		}
		parts := strings.Split(u.Path, "/")
		var org, repoName string
		//Check length to avoid panic
		if len(parts) > 3 {
			org = parts[1]
			repoName = strings.TrimSuffix(strings.Join(parts[2:], "/"), ".git")
		}

		u.User = url.UserPassword("", gitHostAccessToken)

		client, err := factory.FromRepoURL(u.String())
		if err != nil {
			return &GitOpsRepoGenError{gitopsURL: gitOpsRepoURL, errMsg: "failed to create a client to access %q: %w", err: err}
		}
		ctx := context.Background()
		// If we're creating the repository in a personal user's account, it's a
		// different API call that's made, clearing the org triggers go-scm to use
		// the "create repo in personal account" endpoint.
		currentUser, _, err := client.Users.Find(ctx)
		if err != nil {
			return &GitOpsRepoGenUserError{err: err}
		}
		if currentUser.Login == org {
			org = ""
		}

		ri := &scm.RepositoryInput{
			Private:     true,
			Description: defaultRepoDescription,
			Namespace:   org,
			Name:        repoName,
		}
		_, _, err = client.Repositories.Create(context.Background(), ri)
		if err != nil {
			repo := fmt.Sprintf("%s/%s", org, repoName)
			if org == "" {
				repo = fmt.Sprintf("%s/%s", currentUser.Login, repoName)
			}
			if _, resp, err := client.Repositories.Find(context.Background(), repo); err == nil && resp.Status == 200 {
				return fmt.Errorf("failed to create repository, repo already exists")
			}
			return &GitCreateRepoError{repoName: repoName, org: org, err: err}
		}

		if out, err := execute(repoPath, GitCommand, "init", "."); err != nil {
			return &GitCmdError{path: repoPath, cmdResult: string(out), err: err, cmdType: initializeGit}
		}
		if out, err := execute(repoPath, GitCommand, "add", "."); err != nil {
			return &GitCmdError{path: repoPath, cmdResult: string(out), err: err, cmdType: addComponents}
		}
		if out, err := execute(repoPath, GitCommand, "commit", "-m", "Generate GitOps resources"); err != nil {
			return &GitCmdError{path: repoPath, cmdResult: string(out), err: err, cmdType: commitFiles}
		}
		if out, err := execute(repoPath, GitCommand, "branch", "-m", branch); err != nil {
			return &GitBranchError{branch: branch, repoPath: repoPath, cmdResult: string(out), err: err, cmdType: switchBranch}
		}
		if out, err := execute(repoPath, GitCommand, "remote", "add", "origin", remote); err != nil {
			return &GitAddFilesToRemoteError{componentName: componentName, remoteURL: remote, repoPath: repoPath, cmdResult: string(out), err: err}
		}
		if out, err := execute(repoPath, GitCommand, "push", "-u", "origin", branch); err != nil {
			return &GitCmdError{path: remote, cmdResult: string(out), err: err, cmdType: pushRemote}
		}
	}

	return nil
}

// GenerateOverlaysAndPush generates the overlays kustomize from App Env Snapshot Binding Spec
// 1. outputPath: Where to output the gitops resources to
// 2. clone: Optionally clone the repository first
// 3. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 4. options: Options for resource generation
// 5. applicationName: The name of the application
// 6. environmentName: The name of the environment
// 7. imageName: The image name of the source
// 8  namespace: The namespace of the component. This is used in as the namespace of the deployment yaml.
// 9. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 10. The branch to push to
// 11. The path within the repository to generate the resources in
// 12. Push the changes to the repository or not.
// 13. The gitops config containing the build bundle;
func (s Gen) GenerateOverlaysAndPush(outputPath string, clone bool, remote string, options gitopsv1alpha1.GeneratorOptions, applicationName, environmentName, imageName, namespace string, appFs afero.Afero, branch string, context string, doPush bool, componentGeneratedResources map[string][]string) error {

	if clone || doPush {
		invalidRemoteErr := util.ValidateRemote(remote)
		if invalidRemoteErr != nil {
			return invalidRemoteErr
		}
	}

	componentName := options.Name
	repoPath := filepath.Join(outputPath, applicationName)

	if clone {
		s.Log.V(6).Info("Cloning the GitOps repository")
		if out, err := execute(outputPath, GitCommand, "clone", remote, applicationName); err != nil {
			return &GitCmdError{path: outputPath, cmdResult: string(out), err: err, cmdType: cloneRepo}
		}

		// Checkout the specified branch
		if _, err := execute(repoPath, GitCommand, "switch", branch); err != nil {
			if out, err := execute(repoPath, GitCommand, "checkout", "-b", branch); err != nil {
				return &GitBranchError{branch: branch, repoPath: repoPath, cmdResult: string(out), err: err, cmdType: checkoutBranch}
			}
		}
	}

	// Generate the gitops resources and update the parent kustomize yaml file
	gitopsFolder := filepath.Join(repoPath, context)
	componentEnvOverlaysPath := filepath.Join(gitopsFolder, "components", componentName, "overlays", environmentName)

	s.Log.V(6).Info("Generating the overlays resources")
	if err := GenerateOverlays(appFs, gitopsFolder, componentEnvOverlaysPath, options, imageName, namespace, componentGeneratedResources); err != nil {
		return &GitGenResourcesAndOverlaysError{path: componentEnvOverlaysPath, componentName: componentName, err: err, cmdType: genOverlays}
	}

	if doPush {
		s.Log.V(6).Info("Committing and pushing the overlays resources")
		return s.CommitAndPush(outputPath, applicationName, remote, componentName, branch, fmt.Sprintf("Generate %s environment overlays for component %s", environmentName, componentName))
	}
	return nil
}

// GitRemoveComponent clones the repo, removes the component, and pushes the changes back to the repository. It takes in the following args and updates the gitops resources by removing the given component
// 1. outputPath: Where to output the gitops resources to
// 2. remote: A string of the form https://$token@<domain>/<org>/<repo>, where <domain> is either github.com or gitlab.com and $token is optional. Corresponds to the component's gitops repository
// 3. componentName: The component name corresponding to a single Component in an Application. eg. component.Name
// 4. The branch to push to
// 5. The path within the repository to generate the resources in
func (s Gen) GitRemoveComponent(outputPath string, remote string, componentName string, branch string, context string) error {
	if cloneError := s.CloneRepo(outputPath, remote, componentName, branch); cloneError != nil {
		return cloneError
	}
	if removeComponentError := removeComponent(outputPath, componentName, context); removeComponentError != nil {
		return removeComponentError
	}

	return s.CommitAndPush(outputPath, "", remote, componentName, branch, fmt.Sprintf("Removed component %s", componentName))
}

// CloneRepo clones the repo, and switches to the branch
// 1. outputPath: Where to output the gitops resources to
// 2. remote: A string of the form https://$token@<domain>/<org>/<repo>, where <domain> is either github.com or gitlab.com and $token is optional. Corresponds to the component's gitops repository
// 3. componentName: The component name corresponding to a single Component in an Application. eg. component.Name
// 4. The branch to push to switch to
func (s Gen) CloneRepo(outputPath string, remote string, componentName string, branch string) error {
	invalidRemoteErr := util.ValidateRemote(remote)
	if invalidRemoteErr != nil {
		return invalidRemoteErr
	}

	repoPath := filepath.Join(outputPath, componentName)

	if out, err := execute(outputPath, GitCommand, "clone", remote, componentName); err != nil {
		return &GitCmdError{path: outputPath, cmdResult: string(out), err: err, cmdType: cloneRepo}
	}

	// Checkout the specified branch
	if _, err := execute(repoPath, GitCommand, "switch", branch); err != nil {
		if out, err := execute(repoPath, GitCommand, "checkout", "-b", branch); err != nil {
			return &GitBranchError{branch: branch, repoPath: repoPath, cmdResult: string(out), err: err, cmdType: checkoutBranch}
		}
	}
	return nil
}

// removeComponent removes the component from the local folder.  This expects the git repo to be already cloned
// 1. outputPath: Where the gitops repo contents have been cloned
// 2. componentName: The component name corresponding to a single Component in an Application. eg. component.Name
// 3. The path within the repository to generate the resources in
func removeComponent(outputPath string, componentName string, context string) error {
	repoPath := filepath.Join(outputPath, componentName)
	gitopsFolder := filepath.Join(repoPath, context)
	componentPath := filepath.Join(gitopsFolder, "components", componentName)
	if out, err := execute(repoPath, RmCommand, "-rf", componentPath); err != nil {
		return &DeleteFolderError{componentPath: componentPath, repoPath: repoPath, cmdResult: string(out), err: err}
	}
	return nil
}

// GetCommitIDFromRepo returns the commit ID for the given repository
func (s Gen) GetCommitIDFromRepo(fs afero.Afero, repoPath string) (string, error) {
	var out []byte
	var err error
	if out, err = execute(repoPath, GitCommand, "rev-parse", "HEAD"); err != nil {
		return "", &GitCmdError{path: repoPath, cmdResult: string(out), err: err, cmdType: getCommitID}
	}
	return string(out), nil
}
