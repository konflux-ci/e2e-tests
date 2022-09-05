package github

import (
	"context"
	"fmt"
)

func (g *Github) DeleteRef(repository, branchName string) error {
	_, err := g.client.Git.DeleteRef(context.Background(), g.organization, repository, fmt.Sprintf("heads/%s", branchName))
	if err != nil {
		return err
	}
	return nil
}
