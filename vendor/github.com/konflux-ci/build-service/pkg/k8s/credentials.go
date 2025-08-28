package k8s

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/konflux-ci/build-service/pkg/boerrors"
	. "github.com/konflux-ci/build-service/pkg/common"
	"github.com/konflux-ci/build-service/pkg/git"
	. "github.com/konflux-ci/build-service/pkg/git/credentials"
	bslices "github.com/konflux-ci/build-service/pkg/slices"
)

type ConfigReader struct {
	client        client.Client
	scheme        *runtime.Scheme
	eventRecorder record.EventRecorder
}

func NewGithubAppConfigReader(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) ConfigReader {
	return ConfigReader{
		client:        client,
		scheme:        scheme,
		eventRecorder: eventRecorder,
	}
}

func (k ConfigReader) GetConfig(ctx context.Context) (githubAppIdStr string, appPrivateKeyPem []byte, err error) {
	//Check if GitHub Application is used, if not then skip
	pacSecret := corev1.Secret{}
	globalPaCSecretKey := types.NamespacedName{Namespace: BuildServiceNamespaceName, Name: PipelinesAsCodeGitHubAppSecretName}
	if err := k.client.Get(ctx, globalPaCSecretKey, &pacSecret); err != nil {
		k.eventRecorder.Event(&pacSecret, "Warning", "ErrorReadingPaCSecret", err.Error())
		return "", nil, err
	}

	// validate content of the fields
	if _, e := strconv.ParseInt(string(pacSecret.Data[PipelinesAsCodeGithubAppIdKey]), 10, 64); e != nil {
		return "", nil, fmt.Errorf(" Pipelines as Code: failed to parse GitHub application ID. Cause: %w", e)
	}

	return string(pacSecret.Data[PipelinesAsCodeGithubAppIdKey]), pacSecret.Data[PipelinesAsCodeGithubPrivateKey], err
}

// GitCredentialProvider is an implementation of the git.CredentialsProvider that retrieves
// the git credentials from the Kubernetes secrets
type GitCredentialProvider struct {
	client client.Client
}

func NewGitCredentialProvider(client client.Client) *GitCredentialProvider {
	return &GitCredentialProvider{
		client: client,
	}
}

func (k *GitCredentialProvider) GetBasicAuthCredentials(ctx context.Context, component *git.ScmComponent) (*BasicAuthCredentials, error) {
	secretWithCredentials, err := k.LookupSecret(ctx, component, corev1.SecretTypeBasicAuth)
	if err != nil {
		return nil, err
	}
	return &BasicAuthCredentials{
		Username: string(secretWithCredentials.Data[corev1.BasicAuthUsernameKey]),
		Password: string(secretWithCredentials.Data[corev1.BasicAuthPasswordKey]),
	}, nil
}

func (k *GitCredentialProvider) GetSSHCredentials(ctx context.Context, component *git.ScmComponent) (*SSHCredentials, error) {
	secretWithCredentials, err := k.LookupSecret(ctx, component, corev1.SecretTypeSSHAuth)
	if err == nil {
		return nil, err
	}
	return &SSHCredentials{
		PrivateKey: secretWithCredentials.Data[corev1.SSHAuthPrivateKey],
	}, nil
}

func (k *GitCredentialProvider) LookupSecret(ctx context.Context, component *git.ScmComponent, secretType corev1.SecretType) (*corev1.Secret, error) {
	log := ctrllog.FromContext(ctx)

	log.Info("looking for scm secret", "component", component)

	secretList := &corev1.SecretList{}
	opts := client.ListOption(&client.MatchingLabels{
		ScmCredentialsSecretLabel: "scm",
		ScmSecretHostnameLabel:    component.RepositoryHost(),
	})

	if err := k.client.List(ctx, secretList, client.InNamespace(component.NamespaceName()), opts); err != nil {
		return nil, fmt.Errorf("failed to list Pipelines as Code secrets in %s namespace: %w", component.NamespaceName(), err)
	}
	log.Info("found secrets", "count", len(secretList.Items))
	secretsWithCredentialsCandidates := bslices.Filter(secretList.Items, func(secret corev1.Secret) bool {
		return secret.Type == secretType && len(secret.Data) > 0
	})
	secretWithCredential := bestMatchingSecret(ctx, component.Repository(), secretsWithCredentialsCandidates)
	if secretWithCredential != nil {
		return secretWithCredential, nil
	}
	log.Info("no matching secret found for component", "component", component)
	return nil, boerrors.NewBuildOpError(boerrors.EComponentGitSecretMissing, nil)
}

// finds the best matching secret for the given repository, considering the repository annotation match priority:
//   - Highest priority is given to the secret with the direct repository path match to the component repository
//   - If no direct match is found, the secret with the longest component paths intersection is returned
//     i.e. for the org/proj/sub1 component URL, secret with org/proj/* will have a higher priority than secret with
//     just org/* and will be returned first
//   - If no secret with matching repository annotation is found, the one with just matching hostname label is returned
func bestMatchingSecret(ctx context.Context, componentRepository string, secrets []corev1.Secret) *corev1.Secret {
	log := ctrllog.FromContext(ctx)
	// secrets without repository annotation
	var hostOnlySecrets []corev1.Secret

	// map of secret index and its best path intersections count, i.e. the count of path parts matched,
	var potentialMatches = make(map[int]int, len(secrets))

	for index, secret := range secrets {
		repositoryAnnotation, exists := secret.Annotations[ScmSecretRepositoryAnnotation]
		log.Info("found secret", "secret", secret.Name, "repositoryAnnotation", repositoryAnnotation, "exists", exists)
		if !exists || repositoryAnnotation == "" {
			hostOnlySecrets = append(hostOnlySecrets, secret)
			continue
		}
		secretRepositories := strings.Split(repositoryAnnotation, ",")
		log.Info("found secret repositories", "repositories", secretRepositories)
		//trim possible slashes at the beginning of the repository path
		for i, repository := range secretRepositories {
			secretRepositories[i] = strings.TrimPrefix(repository, "/")
		}

		// Direct repository match, return secret
		log.Info("checking for direct match", "componentRepository", componentRepository, "secretRepositories", secretRepositories)
		if slices.Contains(secretRepositories, componentRepository) {
			return &secret
		}
		log.Info("no direct match found", "componentRepository", componentRepository, "secretRepositories", secretRepositories)
		// No direct match, check for wildcard match, i.e. org/repo/* matches org/repo/foo, org/repo/bar, etc.
		componentRepoParts := strings.Split(componentRepository, "/")

		// Find wildcard repositories
		wildcardRepos := slices.Filter(nil, secretRepositories, func(s string) bool { return strings.HasSuffix(s, "*") })

		for _, repo := range wildcardRepos {
			i := bslices.Intersection(componentRepoParts, strings.Split(strings.TrimSuffix(repo, "*"), "/"))
			if i > 0 && potentialMatches[index] < i {
				// Add whole secret index to potential matches
				potentialMatches[index] = i
			}
		}
	}
	log.Info("potential matches", "count", len(potentialMatches))
	if len(potentialMatches) == 0 {
		if len(hostOnlySecrets) == 0 {
			return nil // Nothing matched
		}
		log.Info("Using host only secret", "name", hostOnlySecrets[0].Name)
		return &hostOnlySecrets[0] // Return first host-only secret
	}
	log.Info("host only secrets", "count", len(hostOnlySecrets), "potentialMatches", potentialMatches)
	// find the best matching secret
	var bestIndex, bestCount int
	for i, count := range potentialMatches {
		if count > bestCount {
			bestCount = count
			bestIndex = i
		}
	}
	return &secrets[bestIndex]
}
