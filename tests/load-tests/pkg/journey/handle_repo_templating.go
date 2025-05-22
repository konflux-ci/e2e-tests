package journey

import "fmt"
import "strings"
import "regexp"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import github "github.com/google/go-github/v44/github"
import utils "github.com/konflux-ci/e2e-tests/pkg/utils"

var fileList = []string{"COMPONENT-pull-request.yaml", "COMPONENT-push.yaml"}

// Parse repo name out of repo url
func getRepoNameFromRepoUrl(repoUrl string) (string, error) {
	// Answer taken from https://stackoverflow.com/questions/7124778/how-can-i-match-anything-up-until-this-sequence-of-characters-in-a-regular-exp
	// Tested with these input data:
	//   repoUrl: https://github.com/abc/nodejs-devfile-sample.git/, match[1]: nodejs-devfile-sample
	//   repoUrl: https://github.com/abc/nodejs-devfile-sample.git, match[1]: nodejs-devfile-sample
	//   repoUrl: https://github.com/abc/nodejs-devfile-sample/, match[1]: nodejs-devfile-sample
	//   repoUrl: https://github.com/abc/nodejs-devfile-sample, match[1]: nodejs-devfile-sample
	//   repoUrl: https://gitlab.example.com/abc/nodejs-devfile-sample, match[1]: abc/nodejs-devfile-sample
	var regex *regexp.Regexp
	if strings.Contains(repoUrl, "gitlab.") {
		regex = regexp.MustCompile(`/([^/]+/[^/]+?)(.git)?/?$`)
	} else {
		regex = regexp.MustCompile(`/([^/]+?)(.git)?/?$`)
	}
	match := regex.FindStringSubmatch(repoUrl)
	if match != nil {
		return match[1], nil
	} else {
		return "", fmt.Errorf("Failed to parse repo name out of url %s", repoUrl)
	}
}

// Template file from '.template/...' to '.tekton/...', expanding placeholders (even in file name) using Github API
// Returns SHA of the commit
func templateRepoFileGithub(f *framework.Framework, repoName, repoRevision, fileName string, placeholders *map[string]string) (string, error) {
	var fileResponse *github.RepositoryContent
	var fileContent string
	var repoContentResponse *github.RepositoryContentResponse
	var err error

	fileResponse, err = f.AsKubeAdmin.CommonController.Github.GetFile(repoName, ".template/" + fileName, repoRevision)
	if err != nil {
		return "", err
	}

	fileContent, err = fileResponse.GetContent()
	if err != nil {
		return "", err
	}

	for key, value := range *placeholders {
		fileContent = strings.ReplaceAll(fileContent, key, value)
		fileName = strings.ReplaceAll(fileName, key, value)
	}

	fileResponse, err = f.AsKubeAdmin.CommonController.Github.GetFile(repoName, ".tekton/" + fileName, repoRevision)
	if err != nil {
		return "", err
	}

	repoContentResponse, err = f.AsKubeAdmin.CommonController.Github.UpdateFile(repoName, ".tekton/" + fileName, fileContent, repoRevision, *fileResponse.SHA)
	if err != nil {
		return "", err
	}

	return *repoContentResponse.Commit.SHA, nil
}

// Template file from '.template/...' to '.tekton/...', expanding placeholders (even in file name) using Gitlab API
// Returns SHA of the commit
func templateRepoFileGitlab(f *framework.Framework, repoName, repoRevision, fileName string, placeholders *map[string]string) (string, error) {
	fileContent, err := f.AsKubeAdmin.CommonController.Gitlab.GetFile(repoName, ".template/" + fileName, repoRevision)
	if err != nil {
		return "", fmt.Errorf("Failed to get file: %v", err)
	}

	for key, value := range *placeholders {
		fileContent = strings.ReplaceAll(fileContent, key, value)
		fileName = strings.ReplaceAll(fileName, key, value)
	}

	commitID, err := f.AsKubeAdmin.CommonController.Gitlab.UpdateFile(repoName, ".tekton/" + fileName, fileContent, repoRevision)
	if err != nil {
		return "", fmt.Errorf("Failed to update file: %v", err)
	}

	logging.Logger.Info("Templated file %s with commit %s", fileName, commitID)
	return commitID, nil
}

// Fork repository and return forked repo URL
func ForkRepo(f *framework.Framework, repoUrl, repoRevision, username string) (string, error) {
	// For PaC testing, let's template repo and return forked repo name
	var forkRepo *github.Repository
	var sourceName string
	var targetName string
	var err error

	// Parse just repo name out of input repo url and construct target repo name
	sourceName, err = getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}
	targetName = fmt.Sprintf("%s-%s", sourceName, username)

	if strings.Contains(repoUrl, "gitlab.") {
		logging.Logger.Debug("Forking Gitlab repository %s", repoUrl)

		logging.Logger.Warning("Forking Gitlab repository not implemented yet, this will only work with 1 concurrent user")   // TODO

		return repoUrl, nil
	} else {
		logging.Logger.Debug("Forking Github repository %s", repoUrl)

		// Cleanup if it already exists
		err = f.AsKubeAdmin.CommonController.Github.DeleteRepositoryIfExists(targetName)
		if err != nil {
			return "", err
		}

		// Create fork and make sure it appears
		err = utils.WaitUntilWithInterval(func() (done bool, err error) {
			forkRepo, err = f.AsKubeAdmin.CommonController.Github.ForkRepository(sourceName, targetName)
			if err != nil {
				logging.Logger.Debug("Repo forking failed, trying again: %v", err)
				return false, nil
			}
			return true, nil
		}, time.Second * 20, time.Minute * 10)
		if err != nil {
			return "", err
		}

		return forkRepo.GetHTMLURL(), nil
	}
}

// Template PaC files
func templateFiles(f *framework.Framework, repoUrl, repoRevision string, placeholders *map[string]string) (*map[string]string, error) {
	var sha string

	// Get repo name from repo url
	repoName, err := getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return nil, err
	}

	// Template files we care about
	shaMap := &map[string]string{}
	for _, file := range fileList {
		if strings.Contains(repoUrl, "gitlab.") {
			sha, err = templateRepoFileGitlab(f, repoName, repoRevision, file, placeholders)
		} else {
			sha, err = templateRepoFileGithub(f, repoName, repoRevision, file, placeholders)
		}
		if err != nil {
			return nil, err
		}
		(*shaMap)[file] = sha
	}

	return shaMap, nil
}

func HandleRepoForking(ctx *MainContext) error {
	logging.Logger.Debug("Forking repository %s for user %s", ctx.Opts.ComponentRepoUrl, ctx.Username)

	forkUrl, err := ForkRepo(
		ctx.Framework,
		ctx.Opts.ComponentRepoUrl,
		ctx.Opts.ComponentRepoRevision,
		ctx.Username,
	)
	if err != nil {
		return logging.Logger.Fail(80, "Repo forking failed: %v", err)
	}

	ctx.ComponentRepoUrl = forkUrl

	return nil
}
