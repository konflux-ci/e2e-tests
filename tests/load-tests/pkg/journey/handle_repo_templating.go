package journey

import "fmt"
import "strings"
import "regexp"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import github "github.com/google/go-github/v44/github"

var fileList = []string{".tekton/multi-platform-test-pull-request.yaml", ".tekton/multi-platform-test-push.yaml"}

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

func templateRepoFile(f *framework.Framework, repoName, repoRevision, fileName string, placeholders *map[string]string) error {
	fileResponse, err1 := f.AsKubeAdmin.CommonController.Github.GetFile(repoName, fileName, repoRevision)
	if err1 != nil {
		return err1
	}

	fileContent, err2 := fileResponse.GetContent()
	if err2 != nil {
		return err2
	}

	for key, value := range *placeholders {
		fileContent = strings.ReplaceAll(fileContent, key, value)
	}

	_, err3 := f.AsKubeAdmin.CommonController.Github.UpdateFile(repoName, fileName, fileContent, repoRevision, *fileResponse.SHA)
	if err3 != nil {
		return err3
	}

	return nil
}

func TemplateRepo(f *framework.Framework, repoUrl, repoRevision, username, namespace, quayRepoName string) (string, error) {
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

	// Template files we care about
	placeholders := &map[string]string{
		"NAMESPACE": namespace,
		"QUAY_REPO": quayRepoName,
	}
	for _, file := range fileList {
		err = templateRepoFile(f, targetName, repoRevision, file, placeholders)
		if err != nil {
			return "", err
		}
	}

	return forkRepo.GetHTMLURL(), nil
}

func TemplateRepoMore(f *framework.Framework, repoUrl, repoRevision, appName, compName string) error {
	// Get repo name from repo url
	repoName, err := getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return err
	}

	// Template files we care about
	placeholders := &map[string]string{
		"APPLICATION": appName,
		"COMPONENT": compName,
	}
	for _, file := range fileList {
		err = templateRepoFile(f, repoName, repoRevision, file, placeholders)
		if err != nil {
			return err
		}
	}

	return nil
}

func HandleRepoTemplating(ctx *MainContext) error {
	if !ctx.Opts.PipelineRequestConfigurePac {
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

func HandleAdditionalTemplating(ctx *PerComponentContext) error {
	if !ctx.ParentContext.ParentContext.Opts.PipelineRequestConfigurePac {
		return nil
	}

	err := TemplateRepoMore(ctx.Framework, ctx.ParentContext.ParentContext.ComponentRepoUrl, ctx.ParentContext.ParentContext.Opts.ComponentRepoRevision, ctx.ParentContext.ApplicationName, ctx.ComponentName)
	if err != nil {
		return logging.Logger.Fail(81, "Additional repo templating failed: %v", err)
	}

	return nil
}
