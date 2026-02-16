package build

import (
	"fmt"

	"github.com/konflux-ci/e2e-tests/pkg/clients/git"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	v1 "k8s.io/api/core/v1"
)

// setupGitProvider is a convenience wrapper that uses the GitProviderRegistry
// to setup a git provider. This keeps existing test code unchanged.
//
// Returns: git client, target repo URL, target repo name/path
func setupGitProvider(f *framework.Framework, gitProvider git.GitProvider) (git.Client, string, string) {
	return SetupGitProviderWithConfig(f, gitProvider)
}

// createBuildSecret creates a build secret for SCM authentication
func createBuildSecret(f *framework.Framework, secretName string, annotations map[string]string, token string) error {
	return createBuildSecretForHost(f, secretName, annotations, token, "github.com")
}

// createBuildSecretForHost creates a build secret for SCM authentication with a specific host
func createBuildSecretForHost(f *framework.Framework, secretName string, annotations map[string]string, token string, host string) error {
	buildSecret := v1.Secret{}
	buildSecret.Name = secretName
	buildSecret.Labels = map[string]string{
		"appstudio.redhat.com/credentials": "scm",
		"appstudio.redhat.com/scm.host":    host,
	}
	if annotations != nil {
		buildSecret.Annotations = annotations
	}
	buildSecret.Type = "kubernetes.io/basic-auth"
	buildSecret.StringData = map[string]string{
		"password": token,
	}
	_, err := f.AsKubeAdmin.CommonController.CreateSecret(f.UserNamespace, &buildSecret)
	if err != nil {
		return fmt.Errorf("error creating build secret: %v", err)
	}
	return nil
}
