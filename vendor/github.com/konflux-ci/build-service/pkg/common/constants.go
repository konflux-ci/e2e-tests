package common

const (
	BuildServiceNamespaceName = "build-service"
	// Pipelines as Code GitHub appliaction configuration secret name.
	// The secret is located in Build Service namespace.
	PipelinesAsCodeGitHubAppSecretName = "pipelines-as-code-secret"
	// Keys of the GitHub app ID and the app private key in the pipelines-as-code-secret
	PipelinesAsCodeGithubAppIdKey   = "github-application-id"
	PipelinesAsCodeGithubPrivateKey = "github-private-key"
)
