package journey

import "fmt"
import "strings"
import "regexp"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
import types "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/types"

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
	//   repoUrl: https://gitlab.example.com/abc/nodejs-devfile-sample, match[1]: nodejs-devfile-sample
	//   repoUrl: https://gitlab.example.com/abc/def/nodejs-devfile-sample, match[1]: nodejs-devfile-sample
	var regex *regexp.Regexp
	regex = regexp.MustCompile(`/([^/]+?)(.git)?/?$`)
	match := regex.FindStringSubmatch(repoUrl)
	if match != nil {
		return match[1], nil
	} else {
		return "", fmt.Errorf("Failed to parse repo name out of url %s", repoUrl)
	}
}

// Parse repo organization out of repo url
func getRepoOrgFromRepoUrl(repoUrl string) (string, error) {
	// Answer taken from https://stackoverflow.com/questions/7124778/how-can-i-match-anything-up-until-this-sequence-of-characters-in-a-regular-exp
	// Tested with these input data:
	//   repoUrl: https://github.com/abc/nodejs-devfile-sample.git/, match[1]: abc
	//   repoUrl: https://github.com/abc/nodejs-devfile-sample.git, match[1]: abc
	//   repoUrl: https://github.com/abc/nodejs-devfile-sample/, match[1]: abc
	//   repoUrl: https://github.com/abc/nodejs-devfile-sample, match[1]: abc
	//   repoUrl: https://gitlab.example.com/abc/nodejs-devfile-sample, match[1]: abc
	//   repoUrl: https://gitlab.example.com/abc/def/nodejs-devfile-sample, match[1]: abc/def
	var regex *regexp.Regexp
	regex = regexp.MustCompile(`^[^/]+://[^/]+/(.*)/.+(.git)?/?$`)
	match := regex.FindStringSubmatch(repoUrl)
	if match != nil {
		return match[1], nil
	} else {
		return "", fmt.Errorf("Failed to parse repo org out of url %s", repoUrl)
	}
}

// Parse repo ID (<organization>/<name>) out of repo url
func getRepoIdFromRepoUrl(repoUrl string) (string, error) {
	repoOrgName, err := getRepoOrgFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}
	repoName, err := getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}
	return repoOrgName + "/" + repoName, nil
}

// Get file content from repository, no matter if on GitLab or GitHub
func getRepoFileContent(f *framework.Framework, repoUrl, repoRevision, fileName string) (string, error) {
	var fileContent string

	repoName, err := getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}
	repoOrgName, err := getRepoOrgFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}

	if strings.Contains(repoUrl, "gitlab.") {
		fileContent, err = f.AsKubeAdmin.CommonController.Gitlab.GetFile(repoOrgName + "/" + repoName, fileName, repoRevision)
		if err != nil {
			return "", fmt.Errorf("Failed to get file %s from repo %s revision %s: %v", fileName, repoOrgName + "/" + repoName, repoRevision, err)
		}
	} else {
		fileResponse, err := f.AsKubeAdmin.CommonController.Github.GetFileWithOrg(repoOrgName, repoName, fileName, repoRevision)
		if err != nil {
			return "", fmt.Errorf("Failed to get file %s from repo %s revision %s: %v", fileName, repoName, repoRevision, err)
		}

		fileContent, err = fileResponse.GetContent()
		if err != nil {
			return "", err
		}
	}

	return fileContent, nil
}

// Update file content in repository, no matter if on GitLab or GitHub
func updateRepoFileContent(f *framework.Framework, repoUrl, repoRevision, fileName, fileContent string) (string, error) {
	var commitSha string

	repoName, err := getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}
	repoOrgName, err := getRepoOrgFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}

	if strings.Contains(repoUrl, "gitlab.") {
		commitSha, err = f.AsKubeAdmin.CommonController.Gitlab.UpdateFile(repoOrgName + "/" + repoName, fileName, fileContent, repoRevision)
		if err != nil {
			return "", fmt.Errorf("Failed to update file %s in repo %s revision %s: %v", fileName, repoOrgName + "/" + repoName, repoRevision, err)
		}
	} else {
		fileResponse, err := f.AsKubeAdmin.CommonController.Github.GetFile(repoName, fileName, repoRevision)
		if err != nil {
			return "", fmt.Errorf("Failed to get file %s from repo %s revision %s: %v", fileName, repoName, repoRevision, err)
		}

		repoContentResponse, err := f.AsKubeAdmin.CommonController.Github.UpdateFile(repoName, fileName, fileContent, repoRevision, *fileResponse.SHA)
		if err != nil {
			return "", fmt.Errorf("Failed to update file %s in repo %s revision %s: %v", fileName, repoName, repoRevision, err)
		}

		commitSha = *repoContentResponse.Commit.SHA
	}

	return commitSha, nil
}

// Template file from source repo and dir to '.tekton/...' in component repo, expanding placeholders (even in file name), no matter if on GitLab or GitHub
// Returns SHA of the commit
func templateRepoFile(f *framework.Framework, repoUrl, repoRevision, sourceRepo, sourceRepoDir, fileName string, placeholders *map[string]string) (string, error) {
	fileContent, err := getRepoFileContent(f, sourceRepo, "main", sourceRepoDir + fileName)
	if err != nil {
		return "", err
	}

	for key, value := range *placeholders {
		fileContent = strings.ReplaceAll(fileContent, key, value)
		fileName = strings.ReplaceAll(fileName, key, value)
	}

	commitSha, err := updateRepoFileContent(f, repoUrl, repoRevision, ".tekton/" + fileName, fileContent)
	if err != nil {
		return "", err
	}

	return commitSha, nil
}

// Fork repository and return forked repo URL
func ForkRepo(f *framework.Framework, repoUrl, repoRevision, suffix, targetOrgName string) (string, error) {
	// For PaC testing, let's template repo and return forked repo name
	var forkRepo *github.Repository
	var sourceName string
	var sourceOrgName string
	var targetName string
	var err error

	// Parse just repo name and org out of input repo url and construct target repo name
	sourceName, err = getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}
	sourceOrgName, err = getRepoOrgFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}

	targetName = fmt.Sprintf("%s-%s", sourceName, suffix)

	if strings.Contains(repoUrl, "gitlab.") {
		// Cleanup if it already exists
		err = f.AsKubeAdmin.CommonController.Gitlab.DeleteRepositoryIfExists(targetOrgName + "/" + targetName)
		if err != nil {
			return "", err
		}

		// Create fork and make sure it appears
		forkedRepoURL, err := f.AsKubeAdmin.CommonController.Gitlab.ForkRepository(sourceOrgName, sourceName, targetOrgName, targetName)
		if err != nil {
			return "", err
		}

		return forkedRepoURL.WebURL, nil
	} else {
		// Cleanup if it already exists
		err = f.AsKubeAdmin.CommonController.Github.DeleteRepositoryIfExists(targetName)
		if err != nil {
			return "", err
		}

		// Create fork and make sure it appears
		err = utils.WaitUntilWithInterval(func() (done bool, err error) {
			forkRepo, err = f.AsKubeAdmin.CommonController.Github.ForkRepositoryWithOrgs(sourceOrgName, sourceName, targetOrgName, targetName)
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
func templateFiles(f *framework.Framework, repoUrl, repoRevision, sourceRepo, sourceRepoDir string, placeholders *map[string]string) (*map[string]string, error) {
	// Template files we care about
	shaMap := &map[string]string{}
	for _, file := range fileList {
		sha, err := templateRepoFile(f, repoUrl, repoRevision, sourceRepo, sourceRepoDir, file, placeholders)
		if err != nil {
			return nil, err
		}
		logging.Logger.Debug("Templated file %s with commit %s", file, sha)
		(*shaMap)[file] = sha
	}

	return shaMap, nil
}

// doHarmlessCommit creates or updates file "just-trigger-build" with current timestamp and commits it
func doHarmlessCommit(f *framework.Framework, repoUrl, repoRevision string) (string, error) {
	fileName := "just-trigger-build"
	var fileContent string
	var sha *string
	var commitSha string

	repoName, err := getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}
	repoOrgName, err := getRepoOrgFromRepoUrl(repoUrl)
	if err != nil {
		return "", err
	}

	if strings.Contains(repoUrl, "gitlab.") {
		// For gitlab, we can get file content. If it fails, we assume it doesn't exist.
		// The UpdateFile API for gitlab creates the file if it doesn't exist.
		existingContent, err := f.AsKubeAdmin.CommonController.Gitlab.GetFile(repoOrgName+"/"+repoName, fileName, repoRevision)
		if err != nil {
			logging.Logger.Debug("Failed to get file %s from repo %s, assuming it does not exist: %v", fileName, repoUrl, err)
			fileContent = ""
		} else {
			fileContent = existingContent
		}
		fileContent += fmt.Sprintf("\n# %s", time.Now().String())

		commitSha, err = f.AsKubeAdmin.CommonController.Gitlab.UpdateFile(repoOrgName+"/"+repoName, fileName, fileContent, repoRevision)
		if err != nil {
			return "", fmt.Errorf("Failed to update file %s in repo %s revision %s: %v", fileName, repoOrgName+"/"+repoName, repoRevision, err)
		}
	} else {
		// For github, we need to get SHA if file exists.
		fileResponse, err := f.AsKubeAdmin.CommonController.Github.GetFile(repoName, fileName, repoRevision)
		if err != nil {
			// Assuming error means not found.
			logging.Logger.Debug("File %s not found in repo %s, will create it.", fileName, repoUrl)
			fileContent = ""
			sha = nil
		} else {
			existingContent, err := fileResponse.GetContent()
			if err != nil {
				return "", err
			}
			fileContent = existingContent
			sha = fileResponse.SHA
		}

		fileContent += fmt.Sprintf("\n# %s", time.Now().String())

		if sha == nil {
			// We have to assume a CreateFile function exists in the framework's github controller
			repoContentResponse, err := f.AsKubeAdmin.CommonController.Github.CreateFile(repoName, fileName, fileContent, repoRevision)
			if err != nil {
				return "", fmt.Errorf("Failed to create file %s in repo %s: %v", fileName, repoUrl, err)
			}
			commitSha = *repoContentResponse.Commit.SHA
		} else {
			repoContentResponse, err := f.AsKubeAdmin.CommonController.Github.UpdateFile(repoName, fileName, fileContent, repoRevision, *sha)
			if err != nil {
				return "", fmt.Errorf("Failed to update file %s in repo %s: %v", fileName, repoUrl, err)
			}
			commitSha = *repoContentResponse.Commit.SHA
		}
	}
	return commitSha, nil
}

func HandleRepoForking(ctx *types.PerUserContext) error {
	var suffix string
	if ctx.Opts.Stage {
		suffix = ctx.Opts.RunPrefix + "-" + ctx.Namespace
	} else {
		suffix = ctx.Namespace
	}
	logging.Logger.Debug("Forking repository %s with suffix %s to %s", ctx.Opts.ComponentRepoUrl, suffix, ctx.Opts.ForkTarget)

	forkUrl, err := ForkRepo(
		ctx.Framework,
		ctx.Opts.ComponentRepoUrl,
		ctx.Opts.ComponentRepoRevision,
		suffix,
		ctx.Opts.ForkTarget,
	)
	if err != nil {
		return logging.Logger.Fail(80, "Repo forking failed: %v", err)
	}

	logging.Logger.Info("Forked %s to %s", ctx.Opts.ComponentRepoUrl, forkUrl)

	ctx.ComponentRepoUrl = forkUrl

	return nil
}
