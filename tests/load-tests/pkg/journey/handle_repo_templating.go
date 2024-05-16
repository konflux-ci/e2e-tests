package journey

import "fmt"
import "strings"
import "regexp"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"

func TemplateRepo(f *framework.Framework, repoUrl, repoRevision, username, namespace, quayRepoName string) (string, error) {
	// PaC testing, let's template repo and return branch name
	var branchName string
	var repoName string
	var err error
	var exists bool

	// Parse just repo name out of url
	regex := regexp.MustCompile(`/([^/]+)/?$`)
	match := regex.FindStringSubmatch(repoUrl)
	if match != nil {
		repoName = match[1]
	} else {
		return "", fmt.Errorf("Failed to parse repo name out of url %s", repoUrl)
	}

	// Cleanup if it already exists
	branchName = username
	exists, err = f.AsKubeAdmin.CommonController.Github.ExistsRef(repoName, branchName)
	if err != nil {
		return "", err
	}
	if exists {
		logging.Logger.Warning("Branch %s already exists, deleting it", branchName)
		err := f.AsKubeAdmin.CommonController.Github.DeleteRef(repoName, branchName)
		if err != nil {
			return "", err
		}
	}

	// Create branch
	err = f.AsKubeAdmin.CommonController.Github.CreateRef(repoName, repoRevision, "", branchName)
	if err != nil {
		return "", err
	}

	// Template files we care about
	fileList := []string{".tekton/multi-platform-test-pull-request.yaml", ".tekton/multi-platform-test-push.yaml"}
	for _, file := range fileList {
		fileResponse, err1 := f.AsKubeAdmin.CommonController.Github.GetFile(repoName, file, branchName)
		if err1 != nil {
			return "", err1
		}

		fileContent, err2 := fileResponse.GetContent()
		if err2 != nil {
			return "", err2
		}

		fileContentNew := strings.ReplaceAll(fileContent, "NAMESPACE", namespace)
		fileContentNew = strings.ReplaceAll(fileContentNew, "QUAY_REPO", quayRepoName)

		_, err3 := f.AsKubeAdmin.CommonController.Github.UpdateFile(repoName, file, fileContentNew, branchName, *fileResponse.SHA)
		if err3 != nil {
			return "", err3
		}
	}

	return branchName, nil
}

func HandleRepoTemplating(ctx *MainContext) error {
	if !ctx.Opts.ComponentRepoTemplate {
		ctx.ComponentRepoRevision = ctx.Opts.ComponentRepoRevision
		return nil
	}

	var err error
	var branchName string

	logging.Logger.Debug("Templating repository %s branch %s", ctx.Opts.ComponentRepoUrl, ctx.Opts.ComponentRepoRevision)

	branchName, err = TemplateRepo(ctx.Framework, ctx.Opts.ComponentRepoUrl, ctx.Opts.ComponentRepoRevision, ctx.Username, ctx.Namespace, ctx.Opts.QuayRepo)
	if err != nil {
		return logging.Logger.Fail(80, "Repo templating failed: %v", err)
	}

	ctx.ComponentRepoRevision = branchName

	return nil
}
