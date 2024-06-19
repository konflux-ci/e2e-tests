package journey

import "fmt"
import "strings"
import "regexp"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import github "github.com/google/go-github/v44/github"

var fileList = []string{"COMPONENT-pull-request.yaml", "COMPONENT-push.yaml"}

// Parse repo name out of repo url
func getRepoNameFromRepoUrl(repoUrl string) (string, error) {
	regex := regexp.MustCompile(`/([^/]+)/?$`)
	match := regex.FindStringSubmatch(repoUrl)
	if match != nil {
		return match[1], nil
	} else {
		return "", fmt.Errorf("Failed to parse repo name out of url %s", repoUrl)
	}
}

// Template file from '.template/...' to '.tekton/...', expanding placeholders (even in file name)
// Returns SHA of the commit
func templateRepoFile(f *framework.Framework, repoName, repoRevision, fileName string, placeholders *map[string]string) (string, error) {
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

	// Cleanup if it already exists
	err = f.AsKubeAdmin.CommonController.Github.DeleteRepositoryIfExists(targetName)
	if err != nil {
		return "", err
	}

	// Create fork and make sure it appears
	forkRepo, err = f.AsKubeAdmin.CommonController.Github.ForkRepository(sourceName, targetName)
	if err != nil {
		return "", err
	}

	return forkRepo.GetHTMLURL(), nil
}

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
		sha, err = templateRepoFile(f, repoName, repoRevision, file, placeholders)
		if err != nil {
			return nil, err
		}
		(*shaMap)[file] = sha
	}

	return shaMap, nil
}

func HandleRepoForking(ctx *MainContext) error {
	if !ctx.Opts.PipelineRequestConfigurePac {
		ctx.ComponentRepoUrl = ctx.Opts.ComponentRepoUrl
		return nil
	}

	logging.Logger.Debug("Templating repository %s for user %s", ctx.Opts.ComponentRepoUrl, ctx.Username)

	forkUrl, err := ForkRepo(
		ctx.Framework,
		ctx.Opts.ComponentRepoUrl,
		ctx.Opts.ComponentRepoRevision,
		ctx.Username,
	)
	if err != nil {
		ctx.TemplatingDoneWG.Done()
		return logging.Logger.Fail(80, "Repo templating failed: %v", err)
	}

	ctx.ComponentRepoUrl = forkUrl

	ctx.TemplatingDoneWG.Done()
	ctx.TemplatingDoneWG.Wait()

	return nil
}
