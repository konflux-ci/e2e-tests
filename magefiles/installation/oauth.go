package installation

import (
	"context"
	"fmt"
	"os"

	retry "github.com/avast/retry-go/v4"
	"github.com/devfile/library/pkg/util"
	ocpOauth "github.com/openshift/api/config/v1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	DEFAULT_IDP_SECRET_NAME            = "htpass-secret"
	DEFAULT_IDP_NAME                   = "ci_identity"
	DEFAULT_OAUTH_NAME                 = "cluster"
	DEFAULT_OPENSHIFT_CONFIG_NAMESPACE = "openshift-config"
)

var (
	randomOCPUserName = util.GenerateRandomString(6)
	randomOCPUserPass = util.GenerateRandomString(10)
)

// CreateOauth generate a new random admin user for testing. This functionality will be executed only in Openshift CI.
func (i *InstallAppStudio) CreateOauth() error {

	if os.Getenv("CI") != "true" {
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(randomOCPUserPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error generating openshift password: %v", err)
	}

	if secret, err := i.KubernetesClient.KubeInterface().CoreV1().Secrets(DEFAULT_OPENSHIFT_CONFIG_NAMESPACE).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DEFAULT_IDP_SECRET_NAME,
			Namespace: DEFAULT_OPENSHIFT_CONFIG_NAMESPACE,
		},
		StringData: map[string]string{
			"htpasswd": fmt.Sprintf("%s:%s", randomOCPUserName, passwordHash),
		},
	}, metav1.CreateOptions{}); err != nil {

		klog.Infof("failed to create secret %s. error: %v", secret.Name, err)
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
		TypeMeta: metav1.TypeMeta{
			APIVersion: ocpOauth.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
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
			return utils.ExecuteCommandInASpecificDirectory("oc", []string{"login", fmt.Sprintf("--kubeconfig=%s", tempKubeconfigPath), fmt.Sprintf("--server=%s", cfg.Host), "--username", randomOCPUserName, "--password", randomOCPUserPass, "--insecure-skip-tls-verify=true"}, "")
		},
		retry.Attempts(30),
	)
	os.Setenv("KUBECONFIG", tempKubeconfigPath)
	return err
}
