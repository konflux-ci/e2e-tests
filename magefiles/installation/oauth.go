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
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	HTPASSWD_FILE_NAME      = "ci.htpaswd"
	DEFAULT_IDP_SECRET_NAME = "htpass-secret"
	DEFAULT_IDP_NAME        = "ci_identity"
	DEFAULT_OAUTH_NAME      = "cluster"
	DEFAULT_HTPASSWD_BIN    = "htpasswd"
)

var (
	randomOCPUserPass = util.GenerateRandomString(10)
	randomOCPUserName = util.GenerateRandomString(6)
	httpaswdArgs      = []string{"-c", "-B", "-b", HTPASSWD_FILE_NAME, randomOCPUserName, randomOCPUserPass}
)

// CreateOauth generate a new random admin user for testing. This functionality will be executed only in Openshift CI.
func (i *InstallAppStudio) CreateOauth() error {
	if os.Getenv("CI") != "true" {
		return nil
	}

	if err := i.generateHttpaswd(); err != nil {
		return err
	}

	if err := utils.ExecuteCommandInASpecificDirectory("oc", []string{"create", "secret", "generic", DEFAULT_IDP_SECRET_NAME, fmt.Sprintf("--from-file=htpasswd=%s/%s", i.TmpDirectory, HTPASSWD_FILE_NAME), "-n", "openshift-config"}, ""); err != nil {
		return err
	}

	if err := i.updateOauthCluster(); err != nil {
		return err
	}
	return i.loginAsNewUser()
}

// updateOauthCluster update the existing oauth object in the openshift cluster
func (i *InstallAppStudio) updateOauthCluster() error {
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

// generateHttpaswd create a new htpasswd file with random user and password
func (i *InstallAppStudio) generateHttpaswd() error {
	return utils.ExecuteCommandInASpecificDirectory(DEFAULT_HTPASSWD_BIN, httpaswdArgs, i.TmpDirectory)
}

// loginAsNewUser add cluster-admin role to a random user and then generate a new kubeconfig and login to the cluster. This func will be executed with an openshift admin user. In openshift CI
// by default the admin user is system:admin
func (i *InstallAppStudio) loginAsNewUser() error {
	if err := utils.ExecuteCommandInASpecificDirectory("oc", []string{"adm", "policy", "add-cluster-role-to-user", "cluster-admin", randomOCPUserName}, ""); err != nil {
		return err
	}
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	klog.Infof("server api: %s", cfg.Host)

	tempKubeconfigPath := "/tmp/kubeconfig"
	err = retry.Do(
		func() error {
			return utils.ExecuteCommandInASpecificDirectory("oc", []string{"login", "--kubeconfig=/tmp/kubeconfig", fmt.Sprintf("--server=%s", cfg.Host), "--username", randomOCPUserName, "--password", randomOCPUserPass, "--insecure-skip-tls-verify=true"}, "")
		},
		retry.Attempts(30),
	)
	os.Setenv("KUBECONFIG", tempKubeconfigPath)
	return err
}
