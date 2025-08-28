package renovate

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/konflux-ci/build-service/pkg/git"
	"github.com/konflux-ci/build-service/pkg/git/github"
	"github.com/konflux-ci/build-service/pkg/git/githubapp"
)

// GithubAppRenovaterTaskProvider is an implementation of TaskProvider that provides Renovate tasks for GitHub App installations.
type GithubAppRenovaterTaskProvider struct {
	appConfigReader githubapp.ConfigReader
}

func NewGithubAppRenovaterTaskProvider(appConfigReader githubapp.ConfigReader) GithubAppRenovaterTaskProvider {
	return GithubAppRenovaterTaskProvider{appConfigReader: appConfigReader}
}
func (g GithubAppRenovaterTaskProvider) GetNewTasks(ctx context.Context, components []*git.ScmComponent) []*Task {
	log := ctrllog.FromContext(ctx)
	githubAppId, privateKey, err := g.appConfigReader.GetConfig(ctx)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "failed to get GitHub App configuration")
		}
		return nil
	}
	githubAppInstallations, slug, err := github.GetAllAppInstallations(githubAppId, privateKey)
	if err != nil {
		log.Error(err, "failed to get GitHub App installations")
		return nil
	}
	componentUrlToBranchesMap := git.ComponentUrlToBranchesMap(components)

	// Match installed repositories with Components and get custom branch if defined
	var newTasks []*Task
	for _, githubAppInstallation := range githubAppInstallations {
		var repositories []*Repository
		for _, repository := range githubAppInstallation.Repositories {
			branches, ok := componentUrlToBranchesMap[repository.GetHTMLURL()]
			// Filter repositories with installed GH App but missing Component
			if !ok {
				continue
			}
			for i := range branches {
				if branches[i] == git.InternalDefaultBranch {
					branches[i] = repository.GetDefaultBranch()
				}
			}

			repositories = append(repositories, &Repository{
				BaseBranches: branches,
				Repository:   repository.GetFullName(),
			})
		}
		// Do not add installation which has no matching repositories
		if len(repositories) == 0 {
			continue
		}
		newTasks = append(newTasks, newGithubTask(slug, githubAppInstallation.Token, repositories))
	}
	return newTasks
}

func newGithubTask(slug string, token string, repositories []*Repository) *Task {
	return &Task{
		Platform:     "github",
		Endpoint:     git.BuildAPIEndpoint("github").APIEndpoint("github.com"),
		Username:     fmt.Sprintf("%s[bot]", slug),
		GitAuthor:    fmt.Sprintf("%s <123456+%s[bot]@users.noreply.github.com>", slug, slug),
		Token:        token,
		Repositories: repositories,
	}
}
