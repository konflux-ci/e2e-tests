package installation

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/devfile/library/pkg/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	DEFAULT_TMP_DIR                     = "tmp"
	DEFAULT_INFRA_DEPLOYMENTS_BRANCH    = "main"
	DEFAULT_INFRA_DEPLOYMENTS_GH_ORG    = "redhat-appstudio"
	DEFAULT_LOCAL_FORK_NAME             = "qe"
	DEFAULT_LOCAL_FORK_ORGANIZATION     = "redhat-appstudio-qe"
	DEFAULT_E2E_APPLICATIONS_NAMEPSPACE = "appstudio-e2e-test"
	DEFAULT_SHARED_SECRETS_NAMESPACE    = "build-templates"
	DEFAULT_SHARED_SECRET_NAME          = "redhat-appstudio-user-workload"
	DEFAULT_E2E_QUAY_ORG                = "redhat-appstudio-qe"
)

var (
	previewInstallArgs = []string{"preview", "--keycloak", "--tolchain"}
)

type InstallAppStudio struct {
	// Kubernetes Client to interact with Openshift Cluster
	KubernetesClient *kubeCl.K8sClient

	// TmpDirectory to store temporary files like git repos or some metadata
	TmpDirectory string

	// Directory where to clone https://github.com/redhat-appstudio/infra-deployments repo
	InfraDeploymentsCloneDir string

	// Branch to clone from https://github.com/redhat-appstudio/infra-deployments. By default will be main
	InfraDeploymentsBranch string

	// Github organization from where will be cloned
	InfraDeploymentsOrganizationName string

	// Desired fork name for testing
	LocalForkName string

	// Github organization to use for testing purposes in preview mode
	LocalGithubForkOrganization string

	// Namespace where build applications will be placed
	E2EApplicationsNamespace string

	// A namespace used for storing a build secret which will be shared across all namespaces
	SharedSecretNamespace string

	// Default quay image repository
	HasDefaultImageRepository string

	// base64-encoded content of a docker/config.json file which contains a valid login credentials for quay.io
	QuayToken string
}

func NewAppStudioInstallController() (*InstallAppStudio, error) {
	cwd, _ := os.Getwd()
	k8sClient, err := kubeCl.NewK8SClient()

	if err != nil {
		return nil, err
	}

	return &InstallAppStudio{
		KubernetesClient:                 k8sClient,
		TmpDirectory:                     DEFAULT_TMP_DIR,
		InfraDeploymentsCloneDir:         fmt.Sprintf("%s/%s/infra-deployments", cwd, DEFAULT_TMP_DIR),
		InfraDeploymentsBranch:           utils.GetEnv("INFRA_DEPLOYMENTS_BRANCH", DEFAULT_INFRA_DEPLOYMENTS_BRANCH),
		InfraDeploymentsOrganizationName: utils.GetEnv("INFRA_DEPLOYMENTS_ORG", DEFAULT_INFRA_DEPLOYMENTS_GH_ORG),
		LocalForkName:                    DEFAULT_LOCAL_FORK_NAME,
		LocalGithubForkOrganization:      utils.GetEnv("MY_GITHUB_ORG", DEFAULT_LOCAL_FORK_ORGANIZATION),
		E2EApplicationsNamespace:         utils.GetEnv("E2E_APPLICATIONS_NAMESPACE", DEFAULT_E2E_APPLICATIONS_NAMEPSPACE),
		SharedSecretNamespace:            DEFAULT_SHARED_SECRETS_NAMESPACE,
		HasDefaultImageRepository:        utils.GetEnv("HAS_DEFAULT_IMAGE_REPOSITORY", fmt.Sprintf("quay.io/%s/test-images-protected", DEFAULT_E2E_QUAY_ORG)),
		QuayToken:                        utils.GetEnv("QUAY_TOKEN", ""),
	}, nil
}

// Start the appstudio installation in preview mode.
func (i *InstallAppStudio) InstallAppStudioPreviewMode() error {
	if _, err := i.cloneInfraDeployments(); err != nil {
		return err
	}
	i.setInstallationEnvironments()

	if err := utils.ExecuteCommandInASpecificDirectory("hack/bootstrap-cluster.sh", previewInstallArgs, i.InfraDeploymentsCloneDir); err != nil {
		return err
	}

	if err := i.createSharedSecret(); err != nil {
		return err
	}

	return i.CreateOauth()
}

func (i *InstallAppStudio) setInstallationEnvironments() {
	os.Setenv("MY_GITHUB_ORG", i.LocalGithubForkOrganization)
	os.Setenv("MY_GITHUB_TOKEN", utils.GetEnv("GITHUB_TOKEN", ""))
	os.Setenv("MY_GIT_FORK_REMOTE", i.LocalForkName)
	os.Setenv("E2E_APPLICATIONS_NAMESPACE", i.E2EApplicationsNamespace)
	os.Setenv("SHARED_SECRET_NAMESPACE", i.SharedSecretNamespace)
	os.Setenv("TEST_BRANCH_ID", util.GenerateRandomString(4))
	os.Setenv("HAS_DEFAULT_IMAGE_REPOSITORY", i.HasDefaultImageRepository)
	os.Setenv("QUAY_TOKEN", i.QuayToken)
}

func (i *InstallAppStudio) cloneInfraDeployments() (*git.Remote, error) {
	dirInfo, err := os.Stat(i.InfraDeploymentsCloneDir)

	if !os.IsNotExist(err) && dirInfo.IsDir() {
		klog.Warningf("folder %s already exists... removing", i.InfraDeploymentsCloneDir)

		err := os.RemoveAll(i.InfraDeploymentsCloneDir)
		if err != nil {
			return nil, fmt.Errorf("error removing %s folder", i.InfraDeploymentsCloneDir)
		}
	}

	url := fmt.Sprintf("https://github.com/%s/infra-deployments", i.InfraDeploymentsOrganizationName)
	refName := fmt.Sprintf("refs/heads/%s", i.InfraDeploymentsBranch)
	klog.Infof("cloning '%s' with git ref '%s'", url, refName)
	repo, _ := git.PlainClone(i.InfraDeploymentsCloneDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.ReferenceName(refName),
		Progress:      os.Stdout,
	})

	return repo.CreateRemote(&config.RemoteConfig{Name: i.LocalForkName, URLs: []string{fmt.Sprintf("https://github.com/%s/infra-deployments.git", i.LocalGithubForkOrganization)}})
}

// createSharedSecret make sure that redhat-appstudio-user-workload secret is created in the build-templates namespace for build purposes
func (i *InstallAppStudio) createSharedSecret() error {
	quayToken := os.Getenv("QUAY_TOKEN")
	if quayToken == "" {
		return fmt.Errorf("failed to obtain quay token from 'QUAY_TOKEN' env; make sure the env exists")
	}

	decodedToken, err := base64.StdEncoding.DecodeString(quayToken)
	if err != nil {
		return fmt.Errorf("failed to decode quay token. Make sure that QUAY_TOKEN env contain a base64 token")
	}

	sharedSecret, err := i.KubernetesClient.KubeInterface().CoreV1().Secrets(i.SharedSecretNamespace).Get(context.Background(), DEFAULT_SHARED_SECRET_NAME, metav1.GetOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := i.KubernetesClient.KubeInterface().CoreV1().Secrets(i.SharedSecretNamespace).Create(context.Background(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DEFAULT_SHARED_SECRET_NAME,
					Namespace: DEFAULT_SHARED_SECRETS_NAMESPACE,
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: decodedToken,
				},
			}, metav1.CreateOptions{})

			if err != nil {
				return fmt.Errorf("error when creating secret %s : %v", DEFAULT_SHARED_SECRET_NAME, err)
			}
		} else {
			sharedSecret.Data = map[string][]byte{
				corev1.DockerConfigJsonKey: decodedToken,
			}
			_, err = i.KubernetesClient.KubeInterface().CoreV1().Secrets(i.SharedSecretNamespace).Update(context.TODO(), sharedSecret, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("error when updating secret '%s' namespace: %v", DEFAULT_SHARED_SECRET_NAME, err)
			}
		}
	}

	return nil
}
