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

package github

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	ghinstallation "github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v45/github"
	"github.com/redhat-appstudio/build-service/pkg/boerrors"
)

// Allow mocking for tests
var NewGithubClientByApp func(appId int64, privateKeyPem []byte, repoUrl string) (*GithubClient, error) = newGithubClientByApp
var NewGithubClientForSimpleBuildByApp func(appId int64, privateKeyPem []byte) (*GithubClient, error) = newGithubClientForSimpleBuildByApp

var IsAppInstalledIntoRepository func(ghclient *GithubClient, repoUrl string) (bool, error) = isAppInstalledIntoRepository
var GetAppInstallations func(githubAppIdStr string, appPrivateKeyPem []byte) ([]ApplicationInstallation, string, error) = getAppInstallations

func newGithubClientByApp(appId int64, privateKeyPem []byte, repoUrl string) (*GithubClient, error) {
	owner, _ := getOwnerAndRepoFromUrl(repoUrl)

	itr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appId, privateKeyPem)
	if err != nil {
		// Inability to create transport based on a private key indicates that the key is bad formatted
		return nil, boerrors.NewBuildOpError(boerrors.EGitHubAppMalformedPrivateKey, err)
	}
	client := github.NewClient(&http.Client{Transport: itr})

	var installId int64
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for installId == 0 {
		installations, resp, err := client.Apps.ListInstallations(context.Background(), &opt.ListOptions)
		if err != nil {
			if resp != nil && resp.Response != nil && resp.Response.StatusCode != 0 {
				switch resp.StatusCode {
				case 401:
					return nil, boerrors.NewBuildOpError(boerrors.EGitHubAppPrivateKeyNotMatched, err)
				case 404:
					return nil, boerrors.NewBuildOpError(boerrors.EGitHubAppDoesNotExist, err)
				}
			}
			return nil, boerrors.NewBuildOpError(boerrors.ETransientError, err)
		}
		for _, val := range installations {
			if strings.EqualFold(val.GetAccount().GetLogin(), owner) {
				installId = val.GetID()
				break
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	if installId == 0 {
		err := fmt.Errorf("unable to find GitHub InstallationID for user %s", owner)
		return nil, boerrors.NewBuildOpError(boerrors.EGitHubAppNotInstalled, err)
	}
	// The user has the application installed,
	// but it doesn't guarantee that the application is installed into all user's repositories.

	token, _, err := client.Apps.CreateInstallationToken(
		context.Background(),
		installId,
		&github.InstallationTokenOptions{})
	if err != nil {
		// TODO analyze the error
		return nil, err
	}

	githubClient := NewGithubClient(token.GetToken())
	githubClient.appId = appId
	githubClient.appPrivateKeyPem = privateKeyPem
	return githubClient, nil
}

// newGithubClientForSimpleBuildByApp creates GitHub client based on an installation token.
// The installation token is generated based on a randomly picked app installation.
// This tricky approach is required for simple builds to make requests to GitHub API. Otherwise, rate limit will be hit.
func newGithubClientForSimpleBuildByApp(appId int64, privateKeyPem []byte) (*GithubClient, error) {
	itr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appId, privateKeyPem)
	if err != nil {
		// Inability to create transport based on a private key indicates that the key is bad formatted
		return nil, boerrors.NewBuildOpError(boerrors.EGitHubAppMalformedPrivateKey, err)
	}
	client := github.NewClient(&http.Client{Transport: itr})

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	installations, resp, err := client.Apps.ListInstallations(context.Background(), &opt.ListOptions)
	if err != nil {
		if resp != nil && resp.Response != nil && resp.Response.StatusCode != 0 {
			switch resp.StatusCode {
			case 401:
				return nil, boerrors.NewBuildOpError(boerrors.EGitHubAppPrivateKeyNotMatched, err)
			case 404:
				return nil, boerrors.NewBuildOpError(boerrors.EGitHubAppDoesNotExist, err)
			}
		}
		return nil, boerrors.NewBuildOpError(boerrors.ETransientError, err)
	}

	if len(installations) < 1 {
		return nil, fmt.Errorf("GitHub app is not installed in any repository")
	}
	installId := installations[rand.Intn(len(installations))].GetID()

	token, _, err := client.Apps.CreateInstallationToken(
		context.Background(),
		installId,
		&github.InstallationTokenOptions{})
	if err != nil {
		// TODO analyze the error
		return nil, err
	}

	return NewGithubClient(token.GetToken()), nil
}

// IsAppInstalledIntoRepository finds out if the application is installed into given repository.
// The application is identified by it's installation token, i.e. the client itself must be created
// from an application installation token. See newGithubClientByApp for details.
// This method should be used only with clients created by newGithubClientByApp.
func (g *GithubClient) IsAppInstalledIntoRepository(repoUrl string) (bool, error) {
	return IsAppInstalledIntoRepository(g, repoUrl)
}

func isAppInstalledIntoRepository(ghclient *GithubClient, repoUrl string) (bool, error) {
	ghclient.ensureAppConfigured()

	owner, repository := getOwnerAndRepoFromUrl(repoUrl)

	listOpts := &github.ListOptions{PerPage: 100}
	for {
		repositoriesListPage, resp, err := ghclient.client.Apps.ListRepos(ghclient.ctx, listOpts)
		if err != nil {
			return false, err
		}
		for _, repo := range repositoriesListPage.Repositories {
			if strings.EqualFold(*repo.Name, repository) && strings.EqualFold(*repo.Owner.Login, owner) {
				return true, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}
	return false, nil
}

// GetConfiguredGitAppName returns name and slug of GitHub App that created client token.
func (g *GithubClient) GetConfiguredGitAppName() (string, string, error) {
	g.ensureAppConfigured()

	itr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, g.appId, g.appPrivateKeyPem)
	if err != nil {
		// Inability to create transport based on a private key indicates that the key is bad formatted
		return "", "", boerrors.NewBuildOpError(boerrors.EGitHubAppMalformedPrivateKey, err)
	}
	client := github.NewClient(&http.Client{Transport: itr})
	githubApp, _, err := client.Apps.Get(g.ctx, "")
	if err != nil {
		return "", "", err
	}
	return githubApp.GetName(), githubApp.GetSlug(), nil
}

func (g *GithubClient) isAppConfigured() bool {
	return g.appId != 0 && len(g.appPrivateKeyPem) > 0
}

func (g *GithubClient) ensureAppConfigured() {
	if !g.isAppConfigured() {
		panic("GitHub Application is not configured for this client")
	}
}

type ApplicationInstallation struct {
	Token        string
	ID           int64
	Repositories []*github.Repository
}

func getAppInstallations(githubAppIdStr string, appPrivateKeyPem []byte) ([]ApplicationInstallation, string, error) {
	githubAppId, err := strconv.ParseInt(githubAppIdStr, 10, 64)
	if err != nil {
		return nil, "", boerrors.NewBuildOpError(boerrors.EGitHubAppMalformedId,
			fmt.Errorf("failed to convert %s to int: %w", githubAppIdStr, err))
	}

	itr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, githubAppId, appPrivateKeyPem)
	if err != nil {
		// Inability to create transport based on a private key indicates that the key is bad formatted
		return nil, "", boerrors.NewBuildOpError(boerrors.EGitHubAppMalformedPrivateKey, err)
	}
	client := github.NewClient(&http.Client{Transport: itr})
	appInstallations := []ApplicationInstallation{}
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	githubApp, _, err := client.Apps.Get(context.Background(), "")
	if err != nil {
		return nil, "", fmt.Errorf("failed to load GitHub app metadata, %w", err)
	}
	slug := (githubApp.GetSlug())
	for {
		installations, resp, err := client.Apps.ListInstallations(context.Background(), &opt.ListOptions)
		if err != nil {
			if resp != nil && resp.Response != nil && resp.Response.StatusCode != 0 {
				switch resp.StatusCode {
				case 401:
					return nil, "", boerrors.NewBuildOpError(boerrors.EGitHubAppPrivateKeyNotMatched, err)
				case 404:
					return nil, "", boerrors.NewBuildOpError(boerrors.EGitHubAppDoesNotExist, err)
				}
			}
			return nil, "", boerrors.NewBuildOpError(boerrors.ETransientError, err)
		}
		for _, val := range installations {
			token, _, err := client.Apps.CreateInstallationToken(
				context.Background(),
				*val.ID,
				&github.InstallationTokenOptions{})
			if err != nil {
				// TODO analyze the error
				continue
			}
			installationClient := NewGithubClient(token.GetToken())

			repositories, err := getRepositoriesFromClient(installationClient)
			if err != nil {
				continue
			}
			appInstallations = append(appInstallations, ApplicationInstallation{
				Token:        token.GetToken(),
				ID:           *val.ID,
				Repositories: repositories,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return appInstallations, slug, nil
}

func getRepositoriesFromClient(ghClient *GithubClient) ([]*github.Repository, error) {
	opt := &github.ListOptions{PerPage: 100}
	var repos []*github.Repository
	for {
		repoList, resp, err := ghClient.client.Apps.ListRepos(context.TODO(), opt)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repoList.Repositories...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return repos, nil
}
