package credentials

import (
	"context"

	"github.com/konflux-ci/build-service/pkg/git"
)

const (
	ScmCredentialsSecretLabel     = "appstudio.redhat.com/credentials"
	ScmSecretHostnameLabel        = "appstudio.redhat.com/scm.host"
	ScmSecretRepositoryAnnotation = "appstudio.redhat.com/scm.repository"
)

type BasicAuthCredentials struct {
	Username string
	Password string
}
type SSHCredentials struct {
	PrivateKey []byte
}
type BasicAuthCredentialsProvider interface {
	GetBasicAuthCredentials(ctx context.Context, component *git.ScmComponent) (*BasicAuthCredentials, error)
}
type BasicAuthCredentialsProviderFunc func(ctx context.Context, component *git.ScmComponent) (*BasicAuthCredentials, error)

func (f BasicAuthCredentialsProviderFunc) GetBasicAuthCredentials(ctx context.Context, component *git.ScmComponent) (*BasicAuthCredentials, error) {
	return f(ctx, component)
}

type SSHCredentialsCredentialsProvider interface {
	GetSSHCredentials(ctx context.Context, component git.ScmComponent) (*SSHCredentials, error)
}
