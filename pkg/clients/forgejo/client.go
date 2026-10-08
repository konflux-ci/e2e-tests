package forgejo

import (
	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// ForgejoClient wraps the Forgejo SDK client
type ForgejoClient struct {
	client *forgejo.Client
	org    string
	apiURL string
	token  string
}

// NewForgejoClient creates a new Forgejo client
func NewForgejoClient(accessToken, baseURL, org string) (*ForgejoClient, error) {
	client, err := forgejo.NewClient(baseURL,
		forgejo.SetToken(accessToken),
		// forgejo.NewClient otherwise probes "GET /api/v1/version" while constructing the
		// client. Codeberg answers that probe with HTTP 429 for IPs it has blocked (CI
		// egress IPs frequently are), which turns a Codeberg-only problem into a failure
		// of every single test, including the ones that never touch Codeberg.
		// Passing an empty version skips the probe and all SDK-side version gating.
		forgejo.SetForgejoVersion(""),
		// The Go default user agent is a common trigger for Codeberg's anti-abuse filter.
		forgejo.SetUserAgent("konflux-ci-e2e-tests"),
	)
	if err != nil {
		return nil, err
	}

	return &ForgejoClient{
		client: client,
		org:    org,
		apiURL: baseURL,
		token:  accessToken,
	}, nil
}

// GetClient returns the underlying Forgejo client
func (fc *ForgejoClient) GetClient() *forgejo.Client {
	return fc.client
}

// GetOrg returns the organization name
func (fc *ForgejoClient) GetOrg() string {
	return fc.org
}
