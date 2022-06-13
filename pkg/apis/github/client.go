package github

import (
	"context"

	"github.com/google/go-github/v44/github"
	"golang.org/x/oauth2"
)

type Github struct {
	client       *github.Client
	organization string
}

func NewGithubClient(token, organization string) *Github {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	return &Github{
		client:       github.NewClient(tc),
		organization: organization,
	}
}
