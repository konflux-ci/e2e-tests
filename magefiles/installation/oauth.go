package installation

import (
	"context"
	"fmt"
	"os"

	retry "github.com/avast/retry-go/v4"
	"github.com/devfile/library/pkg/util"
	ocpOauth "github.com/openshift/api/config/v1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	HTPASSWD_FILE_NAME      = "ci.htpaswd"
	DEFAULT_IDP_SECRET_NAME = "htpass-secretss"
	DEFAULT_IDP_NAME        = "ci_identity"
	DEFAULT_OAUTH_NAME      = "cluster"
	DEFAULT_HTPASSWD_BIN    = "htpasswd"
)

var (
	randomOCPUserPass = util.GenerateRandomString(10)
	randomOCPUserName = util.GenerateRandomString(6)
	httpaswdArgs      = []string{"-c", "-B", "-b", HTPASSWD_FILE_NAME, randomOCPUserName, randomOCPUserPass}
)

func (i *InstallAppStudio) CreateOauth() error {
	if err := i.GenerateHttpaswd(); err != nil {
		return err
	}

	if err := utils.ExecuteCommandInASpecificDirectory("oc", []string{"create", "secret", "generic", DEFAULT_IDP_SECRET_NAME, fmt.Sprintf("--from-file=htpasswd=%s/%s", i.TmpDirectory, HTPASSWD_FILE_NAME), "-n", "openshift-config"}, ""); err != nil {
		return err
	}

	if err := i.createOauthObject(); err != nil {
		return err
	}
	return i.LoginAsNewUser()
}

func (i *InstallAppStudio) createOauthObject() error {
	namespacedName := types.NamespacedName{
		Name: DEFAULT_OAUTH_NAME,
	}

	oauth := &ocpOauth.OAuth{}
	if err := i.KubernetesClient.KubeRest().Get(context.TODO(), namespacedName, oauth); err != nil {
		return err
	}

	updateObj := &ocpOauth.OAuth{
		TypeMeta: v1.TypeMeta{
			APIVersion: ocpOauth.GroupVersion.Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: DEFAULT_OAUTH_NAME,
		},
		Spec: ocpOauth.OAuthSpec{
			IdentityProviders: []ocpOauth.IdentityProvider{
				{
					MappingMethod: ocpOauth.MappingMethodClaim,
					Name:          DEFAULT_IDP_NAME,
					IdentityProviderConfig: ocpOauth.IdentityProviderConfig{

						Type: ocpOauth.IdentityProviderTypeHTPasswd,
						HTPasswd: &ocpOauth.HTPasswdIdentityProvider{
							FileData: ocpOauth.SecretNameReference{
								Name: DEFAULT_IDP_SECRET_NAME,
							},
						},
					},
				},
			},
		},
	}
	updateObj.SetResourceVersion(oauth.GetResourceVersion())

	return i.KubernetesClient.KubeRest().Update(context.Background(), updateObj)
}

func (i *InstallAppStudio) GenerateHttpaswd() error {
	return utils.ExecuteCommandInASpecificDirectory(DEFAULT_HTPASSWD_BIN, httpaswdArgs, i.TmpDirectory)
}

func (i *InstallAppStudio) LoginAsNewUser() error {
	// At this stage in CI we will use the admin user to
	utils.ExecuteCommandInASpecificDirectory("oc", []string{"whoami", "--show-token"}, "")

	if err := utils.ExecuteCommandInASpecificDirectory("oc", []string{"adm", "policy", "add-cluster-role-to-user", "cluster-admin", randomOCPUserName}, ""); err != nil {
		return err
	}
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	fmt.Println(cfg.Host)
	tempKubeconfigPath := "/tmp/kubeconfig"

	err = retry.Do(
		func() error {
			tempKubeconfigPath := "/tmp/kubeconfig"
			os.Setenv("KUBECONFIG_TEST", tempKubeconfigPath)
			return utils.ExecuteCommandInASpecificDirectory("oc", []string{"login", "--kubeconfig=/tmp/kubeconfig", "--server", cfg.Host, "--username", randomOCPUserName, "--password", randomOCPUserPass, "--insecure-skip-tls-verify=true", "--loglevel=9"}, "")
		},
		retry.Attempts(30),
	)
	os.Setenv("KUBECONFIG", tempKubeconfigPath)
	return err
}
