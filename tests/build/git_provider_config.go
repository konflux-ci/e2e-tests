package build

import (
	"fmt"
	"os"
	"strings"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/clients/git"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck // ST1001 - Ginkgo DSL convention
	. "github.com/onsi/gomega"    //nolint:staticcheck // ST1001 - Gomega DSL convention
)

const (
	// E2E_GIT_PROVIDERS_ENV controls which git providers to run tests against.
	// Comma-separated list of provider prefixes: "gh,gl,gt" or "gh" or "gl,gt"
	// If not set or empty, all registered providers will be used.
	// Examples:
	//   E2E_GIT_PROVIDERS=gh           -> only GitHub
	//   E2E_GIT_PROVIDERS=gl           -> only GitLab
	//   E2E_GIT_PROVIDERS=gh,gl        -> GitHub and GitLab
	//   E2E_GIT_PROVIDERS=gh,gl,gt     -> all three (GitHub, GitLab, Gitea)
	//   E2E_GIT_PROVIDERS=""           -> all registered providers (default)
	E2E_GIT_PROVIDERS_ENV = "E2E_GIT_PROVIDERS"
)

// GitProviderConfig holds all provider-specific configuration
type GitProviderConfig struct {
	// Provider identifier
	Provider git.GitProvider

	// Prefix used for component names (e.g., "gh", "gl", "gt")
	Prefix string

	// LabelName is the Ginkgo label used for --label-filter (e.g., "github", "gitlab", "gitea")
	LabelName string

	// URLFormat is the format string for repository URLs (e.g., "https://github.com/%s/%s")
	URLFormat string

	// Organization/group name for repositories
	Org string

	// SourceRepoProjectID is used for providers that need full project path (e.g., GitLab)
	// For GitHub, this can be just the repo name
	SourceRepoProjectID string

	// TokenEnvVar is the environment variable name for the provider token
	TokenEnvVar string

	// SecretName is the name of the build secret to create for this provider
	SecretName string

	// CreateClient is a function that creates the git client for this provider
	CreateClient func(f *framework.Framework) git.Client

	// SetupBuildSecret is an optional function to setup provider-specific build secrets
	// Returns nil if no secret setup is needed
	SetupBuildSecret func(f *framework.Framework) error

	// BuildTargetRepoName builds the target repository name/path for forking
	BuildTargetRepoName func(baseRepoName string) string

	// BuildTargetRepoURL builds the full URL for the target repository
	BuildTargetRepoURL func(org, repoName string) string
}

// GitProviderRegistry holds all registered git provider configurations
var GitProviderRegistry = map[git.GitProvider]*GitProviderConfig{}

// RegisterGitProvider registers a git provider configuration
func RegisterGitProvider(config *GitProviderConfig) {
	GitProviderRegistry[config.Provider] = config
}

// GetGitProviderConfig returns the configuration for a given provider
func GetGitProviderConfig(provider git.GitProvider) (*GitProviderConfig, error) {
	config, exists := GitProviderRegistry[provider]
	if !exists {
		return nil, fmt.Errorf("git provider %v is not registered", provider)
	}
	return config, nil
}

// GetRegisteredProviderEntries returns all registered providers as Ginkgo table entries
// This can be used directly in DescribeTableSubtree.
// Each entry includes a Label matching the provider name (e.g., "github", "gitlab")
// so that --label-filter works correctly for provider-based filtering.
func GetRegisteredProviderEntries() []TableEntry {
	var entries []TableEntry
	for provider, config := range GitProviderRegistry {
		entries = append(entries, Entry(config.Prefix, Label(config.LabelName), provider, config.Prefix))
	}
	return entries
}

// GetEnabledProviderEntries returns only the enabled providers based on E2E_GIT_PROVIDERS env var.
// This allows controlling test volume through environment configuration.
//
// Usage in tests:
//
//	DescribeTableSubtree("test PaC component build", func(gitProvider git.GitProvider, gitPrefix string) {
//	    // test code
//	}, GetEnabledProviderEntries()...)
//
// Environment variable examples:
//   - E2E_GIT_PROVIDERS=gh          -> only GitHub tests run
//   - E2E_GIT_PROVIDERS=gl          -> only GitLab tests run
//   - E2E_GIT_PROVIDERS=gh,gl       -> both GitHub and GitLab
//   - E2E_GIT_PROVIDERS=gh,gl,gt    -> all three providers
//   - E2E_GIT_PROVIDERS="" or unset -> all registered providers (default)
func GetEnabledProviderEntries() []TableEntry {
	enabledProviders := getEnabledProviderPrefixes()

	var entries []TableEntry
	for provider, config := range GitProviderRegistry {
		if isProviderEnabled(config.Prefix, enabledProviders) {
			entries = append(entries, Entry(config.Prefix, Label(config.LabelName), provider, config.Prefix))
		}
	}

	// If no entries matched (e.g., typo in env var), return all as fallback
	if len(entries) == 0 {
		return GetRegisteredProviderEntries()
	}

	return entries
}

// getEnabledProviderPrefixes parses the E2E_GIT_PROVIDERS environment variable
// Returns nil if not set (meaning all providers are enabled)
func getEnabledProviderPrefixes() []string {
	envValue := os.Getenv(E2E_GIT_PROVIDERS_ENV)
	if envValue == "" {
		return nil // all providers enabled
	}

	var prefixes []string
	for _, p := range strings.Split(envValue, ",") {
		trimmed := strings.TrimSpace(strings.ToLower(p))
		if trimmed != "" {
			prefixes = append(prefixes, trimmed)
		}
	}
	return prefixes
}

// isProviderEnabled checks if a provider prefix is in the enabled list
// If enabledList is nil, all providers are considered enabled
func isProviderEnabled(prefix string, enabledList []string) bool {
	if enabledList == nil {
		return true // all enabled
	}
	prefix = strings.ToLower(prefix)
	for _, enabled := range enabledList {
		if enabled == prefix {
			return true
		}
	}
	return false
}

// IsProviderEnabled is a public helper to check if a specific provider is enabled
// Useful for conditional test setup or skip logic
func IsProviderEnabled(provider git.GitProvider) bool {
	config, exists := GitProviderRegistry[provider]
	if !exists {
		return false
	}
	return isProviderEnabled(config.Prefix, getEnabledProviderPrefixes())
}

func init() {
	// Register GitHub provider
	RegisterGitProvider(&GitProviderConfig{
		Provider:            git.GitHubProvider,
		Prefix:              "gh",
		LabelName:           "github",
		URLFormat:           githubUrlFormat,
		Org:                 githubOrg,
		SourceRepoProjectID: helloWorldComponentGitSourceRepoName,
		TokenEnvVar:         "GITHUB_TOKEN",
		SecretName:          "",

		CreateClient: func(f *framework.Framework) git.Client {
			return git.NewGitHubClient(f.AsKubeAdmin.CommonController.Github)
		},

		SetupBuildSecret: nil, // GitHub doesn't need special secret setup in this context

		BuildTargetRepoName: func(baseRepoName string) string {
			return baseRepoName + "-" + util.GenerateRandomString(6)
		},

		BuildTargetRepoURL: func(org, repoName string) string {
			return fmt.Sprintf(githubUrlFormat, org, repoName)
		},
	})

	// Register GitLab provider
	RegisterGitProvider(&GitProviderConfig{
		Provider:            git.GitLabProvider,
		Prefix:              "gl",
		LabelName:           "gitlab",
		URLFormat:           gitlabUrlFormat,
		Org:                 gitlabOrg,
		SourceRepoProjectID: helloWorldComponentGitLabProjectID,
		TokenEnvVar:         constants.GITLAB_BOT_TOKEN_ENV,
		SecretName:          "pipelines-as-code-secret",

		CreateClient: func(f *framework.Framework) git.Client {
			return git.NewGitlabClient(f.AsKubeAdmin.CommonController.Gitlab)
		},

		SetupBuildSecret: func(f *framework.Framework) error {
			gitlabToken := utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, "")
			if gitlabToken == "" {
				return fmt.Errorf("GitLab token environment variable %s is not set", constants.GITLAB_BOT_TOKEN_ENV)
			}
			secretAnnotations := map[string]string{}
			return build.CreateGitlabBuildSecret(f, "pipelines-as-code-secret", secretAnnotations, gitlabToken)
		},

		BuildTargetRepoName: func(baseRepoName string) string {
			return fmt.Sprintf("%s/%s", gitlabOrg, baseRepoName+"-"+util.GenerateRandomString(6))
		},

		BuildTargetRepoURL: func(org, repoName string) string {
			return fmt.Sprintf(gitlabUrlFormat, repoName)
		},
	})

	// To add Gitea, simply add another RegisterGitProvider call here:
	//
	// RegisterGitProvider(&GitProviderConfig{
	//     Provider:            git.GiteaProvider,  // Add this constant to pkg/clients/git/git.go
	//     Prefix:              "gt",
	//     LabelName:           "gitea",
	//     URLFormat:           giteaUrlFormat,     // Add to const.go: "https://gitea.example.com/%s/%s"
	//     Org:                 giteaOrg,           // Add to const.go
	//     SourceRepoProjectID: helloWorldComponentGiteaRepoName,
	//     TokenEnvVar:         constants.GITEA_BOT_TOKEN_ENV,
	//     SecretName:          "gitea-pac-secret",
	//
	//     CreateClient: func(f *framework.Framework) git.Client {
	//         return git.NewGiteaClient(f.AsKubeAdmin.CommonController.Gitea)
	//     },
	//
	//     SetupBuildSecret: func(f *framework.Framework) error {
	//         giteaToken := utils.GetEnv(constants.GITEA_BOT_TOKEN_ENV, "")
	//         if giteaToken == "" {
	//             return fmt.Errorf("Gitea token not set")
	//         }
	//         return build.CreateGiteaBuildSecret(f, "gitea-pac-secret", nil, giteaToken)
	//     },
	//
	//     BuildTargetRepoName: func(baseRepoName string) string {
	//         return baseRepoName + "-" + util.GenerateRandomString(6)
	//     },
	//
	//     BuildTargetRepoURL: func(org, repoName string) string {
	//         return fmt.Sprintf(giteaUrlFormat, org, repoName)
	//     },
	// })
}

// SetupGitProviderWithConfig sets up a git provider using the registry configuration
// Returns: git client, target repo URL, target repo name/path
func SetupGitProviderWithConfig(f *framework.Framework, provider git.GitProvider) (git.Client, string, string) {
	config, err := GetGitProviderConfig(provider)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to get git provider config")

	// Create the git client
	gitClient := config.CreateClient(f)

	// Setup build secret if needed
	if config.SetupBuildSecret != nil {
		// First verify token is available
		token := utils.GetEnv(config.TokenEnvVar, "")
		Expect(token).ShouldNot(BeEmpty(), fmt.Sprintf("%s token environment variable %s is not set", config.Prefix, config.TokenEnvVar))

		err := config.SetupBuildSecret(f)
		Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("Failed to setup build secret for %s", config.Prefix))
	}

	// Build target repo name and URL
	targetRepoName := config.BuildTargetRepoName(helloWorldComponentGitSourceRepoName)
	targetRepoURL := config.BuildTargetRepoURL(config.Org, targetRepoName)

	// Fork the repository
	err = gitClient.ForkRepository(config.SourceRepoProjectID, targetRepoName)
	Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("Failed to fork repository for %s", config.Prefix))

	return gitClient, targetRepoURL, targetRepoName
}

