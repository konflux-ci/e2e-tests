package git

import "fmt"

// APIEndpoint interface defines the method to get the API endpoint url for
// the source code providers.
type APIEndpoint interface {
	APIEndpoint(host string) string
}

// GithubAPIEndpoint represents an API endpoint for GitHub.
type GithubAPIEndpoint struct {
}

// APIEndpoint returns the GitHub API endpoint.
func (g *GithubAPIEndpoint) APIEndpoint(host string) string {
	return fmt.Sprintf("https://api.%s/", host)
}

// GitlabAPIEndpoint represents an API endpoint for GitLab.
type GitlabAPIEndpoint struct {
}

// APIEndpoint returns the API GitLab endpoint.
func (g *GitlabAPIEndpoint) APIEndpoint(host string) string {
	return fmt.Sprintf("https://%s/api/v4/", host)
}

// UnknownAPIEndpoint represents an endpoint for unknown or non existed provider. It returns empty string for api endpoint.
type UnknownAPIEndpoint struct {
}

// APIEndpoint returns the GitLab endpoint.
func (g *UnknownAPIEndpoint) APIEndpoint(host string) string {
	return ""
}

// BuildAPIEndpoint constructs and returns an endpoint object based on the type provided type.
func BuildAPIEndpoint(endpointType string) APIEndpoint {
	switch endpointType {
	case "github":
		return &GithubAPIEndpoint{}
	case "gitlab":
		return &GitlabAPIEndpoint{}
	default:
		return &UnknownAPIEndpoint{}
	}
}
