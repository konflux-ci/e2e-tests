package journey

import "fmt"
import "strings"
import "regexp"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import github "github.com/google/go-github/v44/github"

func TemplateRepo(f *framework.Framework, repoUrl, repoRevision, username, namespace, quayRepoName string) (string, error) {
	// For PaC testing, let's template repo and return forked repo name
	var sourceName string
	var targetName string
	var forkRepo *github.Repository
	var err error

	// Parse just repo name out of url
	regex := regexp.MustCompile(`/([^/]+)/?$`)
	match := regex.FindStringSubmatch(repoUrl)
	if match != nil {
		sourceName = match[1]
	} else {
		return "", fmt.Errorf("Failed to parse repo name out of url %s", repoUrl)
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

	// Template files we care about
	fileList := []string{".tekton/multi-platform-test-pull-request.yaml", ".tekton/multi-platform-test-push.yaml"}
	for _, file := range fileList {
		fileResponse, err1 := f.AsKubeAdmin.CommonController.Github.GetFile(targetName, file, repoRevision)
		if err1 != nil {
			return "", err1
		}

		fileContent, err2 := fileResponse.GetContent()
		if err2 != nil {
			return "", err2
		}

		fileContentNew := strings.ReplaceAll(fileContent, "NAMESPACE", namespace)
		fileContentNew = strings.ReplaceAll(fileContentNew, "QUAY_REPO", quayRepoName)

		_, err3 := f.AsKubeAdmin.CommonController.Github.UpdateFile(targetName, file, fileContentNew, repoRevision, *fileResponse.SHA)
		if err3 != nil {
			return "", err3
		}
	}

	return forkRepo.GetHTMLURL(), nil
}

func HandleRepoTemplating(ctx *MainContext) error {

	if !ctx.Opts.ComponentRepoTemplate {
		ctx.ComponentRepoUrl = ctx.Opts.ComponentRepoUrl
		return nil
	}

	logging.Logger.Debug("Templating repository %s for user %s", ctx.Opts.ComponentRepoUrl, ctx.Username)

	forkUrl, err := TemplateRepo(ctx.Framework, ctx.Opts.ComponentRepoUrl, ctx.Opts.ComponentRepoRevision, ctx.Username, ctx.Namespace, ctx.Opts.QuayRepo)
	if err != nil {
		ctx.TemplatingDoneWG.Done()
		return logging.Logger.Fail(80, "Repo templating failed: %v", err)
	}

	ctx.ComponentRepoUrl = forkUrl

	ctx.TemplatingDoneWG.Done()
	ctx.TemplatingDoneWG.Wait()

	return nil
}
