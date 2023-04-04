package github

import (
	"context"
	"time"

	"github.com/gofri/go-github-ratelimit/github_ratelimit"
	"github.com/google/go-github/v44/github"
	"golang.org/x/oauth2"
)

type Github struct {
	client       *github.Client
	organization string
}

func NewGithubClient(token, organization string) (*Github, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	// https://docs.github.com/en/rest/guides/best-practices-for-integrators?apiVersion=2022-11-28#dealing-with-secondary-rate-limits
	rateLimiter, err := github_ratelimit.NewRateLimitWaiterClient(tc.Transport, github_ratelimit.WithSingleSleepLimit(time.Minute, nil))
	if err != nil {
		return &Github{}, err
	}
	client := github.NewClient(rateLimiter)
	githubClient := &Github{
		client:       client,
		organization: organization,
	}

	return githubClient, nil
}
