package githubapp

import (
	"context"
)

// ConfigReader is an interface for reading GitHub App configuration.
type ConfigReader interface {
	GetConfig(ctx context.Context) (githubAppIdStr string, appPrivateKeyPem []byte, err error)
}
