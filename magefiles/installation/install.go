package installation

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/devfile/library/pkg/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"k8s.io/klog"
)

const (
	DEFAULT_INFRA_DEPLOYMENTS_BRANCH    = "main"
	DEFAULT_INFRA_DEPLOYMENTS_GH_ORG    = "redhat-appstudio"
	DEFAULT_LOCAL_FORK_NAME             = "qe"
	DEFAULT_LOCAL_FORK_ORGANIZATION     = "redhat-appstudio-qe"
	DEFAULT_E2E_APPLICATIONS_NAMEPSPACE = "appstudio-e2e-test"
	DEFAULT_SHARED_SECRETS_NAMESPACE    = "build-templates"
	DEFAULT_E2E_QUAY_ORG                = "redhat-appstudio-qe"
)

var (
	previewInstallArgs = []string{"preview"}
)

type InstallAppStudio struct {
	// Kubernetes Client to interact with Openshift Cluster
	KubernetesClient *kubeCl.K8sClient

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

	// build secrets which will be shared across all namespaces
	SharedSecretNamespace string

	// Default quay image repository
	HasDefaultImageRepository string

	// Valid quay token from quay.io
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
		InfraDeploymentsCloneDir:         fmt.Sprintf("%s/tmp/infra-deployments", cwd),
		InfraDeploymentsBranch:           utils.GetEnv(os.Getenv("INFRA_DEPLOYMENTS_BRANCH"), DEFAULT_INFRA_DEPLOYMENTS_BRANCH),
		InfraDeploymentsOrganizationName: utils.GetEnv(os.Getenv("INFRA_DEPLOYMENTS_ORG"), DEFAULT_INFRA_DEPLOYMENTS_GH_ORG),
		LocalForkName:                    DEFAULT_LOCAL_FORK_NAME,
		LocalGithubForkOrganization:      utils.GetEnv("MY_GITHUB_ORG", DEFAULT_LOCAL_FORK_ORGANIZATION),
		E2EApplicationsNamespace:         utils.GetEnv("E2E_APPLICATIONS_NAMESPACE", DEFAULT_E2E_APPLICATIONS_NAMEPSPACE),
		SharedSecretNamespace:            utils.GetEnv("SHARED_SECRET_NAMESPACE", DEFAULT_SHARED_SECRETS_NAMESPACE),
		HasDefaultImageRepository:        utils.GetEnv("HAS_DEFAULT_IMAGE_REPOSITORY", fmt.Sprintf("quay.io/%s/test-images-protected", DEFAULT_E2E_QUAY_ORG)),
		QuayToken:                        utils.GetEnv("QUAY_TOKEN", ""),
	}, nil
}

func (i *InstallAppStudio) InstallAppStudioPreviewMode() error {
	if _, err := i.cloneInfraDeployments(); err != nil {
		fmt.Println(err)
		return err
	}
	i.setInstallationEnvironments()

	if err := i.runInstallationScript(); err != nil {
		return err
	}

	return nil
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

	repo, _ := git.PlainClone(i.InfraDeploymentsCloneDir, false, &git.CloneOptions{
		URL:           fmt.Sprintf("https://github.com/%s/infra-deployments", i.InfraDeploymentsOrganizationName),
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", i.InfraDeploymentsBranch)),
		Progress:      os.Stdout,
	})

	return repo.CreateRemote(&config.RemoteConfig{Name: i.LocalForkName, URLs: []string{fmt.Sprintf("https://github.com/%s/infra-deployments.git", i.LocalGithubForkOrganization)}})
}

func (i *InstallAppStudio) runInstallationScript() error {
	cmd := exec.Command("hack/bootstrap-cluster.sh", previewInstallArgs...) // nolint:gosec
	cmd.Dir = i.InfraDeploymentsCloneDir
	stdin, err := cmd.StdinPipe()

	if err != nil {
		return err
	}
	defer stdin.Close() // the doc says subProcess.Wait will close it, but I'm not sure, so I kept this line

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Start(); err != nil {
		klog.Errorf("an error ocurred: %s", err)

		return err
	}

	io.WriteString(stdin, "4\n")
	cmd.Wait()

	return nil
}
