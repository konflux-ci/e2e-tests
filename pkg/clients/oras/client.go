package oras

import (
	"context"
	"os"

	"github.com/openshift/library-go/pkg/image/reference"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// PullArtifacts pulls artifacts from the given imagePullSpec.
// Pulled artifacts will be stored in a local directory, whose path is returned.
func PullArtifacts(imagePullSpec string) (string, error) {
	storePath, err := os.MkdirTemp("", "pulled-artifacts")
	if err != nil {
		return "", err
	}
	fs, err := file.New(storePath)
	if err != nil {
		return "", err
	}
	defer fs.Close()

	imageRef, err := reference.Parse(imagePullSpec)
	if err != nil {
		return "", err
	}

	repo, err := remote.NewRepository(imagePullSpec)
	if err != nil {
		return "", err
	}
	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(imageRef.Registry, auth.Credential{
			AccessToken: os.Getenv("QUAY_TOKEN"),
		}),
	}

	ctx := context.Background()
	tag := imageRef.Tag
	if _, err := oras.Copy(ctx, repo, tag, fs, tag, oras.DefaultCopyOptions); err != nil {
		return "", err
	}

	return storePath, nil
}
