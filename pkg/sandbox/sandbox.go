package sandbox

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	toolchainApi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-e2e/testsupport/md5"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DEFAULT_KEYCLOAK_MASTER_REALM = "master"

	DEFAULT_KEYCLOAK_ADMIN_CLIENT_ID = "admin-cli"

	DEFAULT_KEYCLOAK_ADMIN_USERNAME = "admin"

	DEFAULT_KEYCLOAK_ADMIN_SECRET = "credential-dev-sso"

	SECRET_KEY = "ADMIN_PASSWORD"

	DEFAULT_TOOLCHAIN_INSTANCE_NAME = "api"

	DEFAULT_TOOLCHAIN_NAMESPACE = "toolchain-host-operator"

	DEFAULT_KEYCLOAK_TESTING_REALM = "redhat-external"

	DEFAULT_KEYCLOAK_TEST_CLIENT_ID = "cloud-services"
)

type SandboxController struct {
	// A Client is an HTTP client. Its zero value (DefaultClient) is a
	// usable client that uses DefaultTransport.
	HttpClient *http.Client

	// A valid keycloak url where to point all API REST calls
	KeycloakUrl string

	// Wrapper of valid kubernetes with admin access to the cluster
	KubeClient kubernetes.Interface

	// Wrapper of valid kubernetes with admin access to the cluster
	KubeRest crclient.Client
}

// Return specs to authenticate with toolchain proxy
type SandboxUserAuthInfo struct {
	// Add a description about user
	UserName string

	// Add a description about kubeconfigpath
	KubeconfigPath string
}

// Values to create a valid user for testing purposes
type KeycloakUser struct {
	FirstName   string                    `json:"firstName"`
	LastName    string                    `json:"lastName"`
	Username    string                    `json:"username"`
	Enabled     string                    `json:"enabled"`
	Email       string                    `json:"email"`
	Credentials []KeycloakUserCredentials `json:"credentials"`
}

type KeycloakUserCredentials struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary string `json:"temporary"`
}

type HttpClient struct {
	*http.Client
}

// NewHttpClient creates http client wrapper with helper functions for rest models call
func NewHttpClient() (*http.Client, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	return client, nil
}

// NewKeyCloakApiController creates http client wrapper with helper functions for keycloak calls
func NewDevSandboxController(kube kubernetes.Interface, kubeRest crclient.Client) (*SandboxController, error) {
	newHttpClient, err := NewHttpClient()
	if err != nil {
		return nil, err
	}

	return &SandboxController{
		HttpClient: newHttpClient,
		KubeClient: kube,
		KubeRest:   kubeRest,
	}, nil
}

// ReconcileUserCreation create a user in sandbox and return a valid kubeconfig for user to be used for the tests
func (s *SandboxController) ReconcileUserCreation(userName string) (*SandboxUserAuthInfo, error) {
	userSignup := &toolchainApi.UserSignup{}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	kubeconfigPath := utils.GetEnv(constants.USER_USER_KUBE_CONFIG_PATH_ENV, fmt.Sprintf("%s/tmp/%s.kubeconfig", wd, userName))

	toolchainApiUrl, err := s.GetOpenshiftRouteHost(DEFAULT_TOOLCHAIN_NAMESPACE, DEFAULT_TOOLCHAIN_INSTANCE_NAME)
	if err != nil {
		return nil, err
	}

	if s.KeycloakUrl, err = s.GetOpenshiftRouteHost(DEFAULT_KEYCLOAK_NAMESPACE, DEFAULT_KEYCLOAK_INSTANCE_NAME); err != nil {
		return nil, err
	}

	err = s.KubeRest.Get(context.Background(), types.NamespacedName{
		Name:      userName,
		Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
	}, userSignup)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			klog.Infof("user %s don't exists... recreating", userName)
		}
	} else {
		userToken, err := s.GetKeycloakToken(DEFAULT_KEYCLOAK_TEST_CLIENT_ID, userName, userName, DEFAULT_KEYCLOAK_TESTING_REALM)
		if err != nil {
			return nil, err
		}
		if err := s.RegisterSandboxUser(userName); err != nil {
			return nil, err
		}

		return s.GetKubeconfigPathForSpecificUser(toolchainApiUrl, userName, kubeconfigPath, userToken)
	}

	if err := s.IsKeycloakRunning(); err != nil {
		return nil, err
	}

	adminSecret, err := s.GetKeycloakAdminSecret()
	if err != nil {
		return nil, err
	}

	adminToken, err := s.GetKeycloakToken(DEFAULT_KEYCLOAK_ADMIN_CLIENT_ID, DEFAULT_KEYCLOAK_ADMIN_USERNAME, adminSecret, DEFAULT_KEYCLOAK_MASTER_REALM)
	if err != nil {
		return nil, err
	}

	if s.KeycloakUserExists(DEFAULT_KEYCLOAK_TESTING_REALM, adminToken.AccessToken, userName) {
		registerUser, err := s.RegisterKeyclokUser(userName, adminToken.AccessToken, DEFAULT_KEYCLOAK_TESTING_REALM)
		if err != nil && registerUser.Username == "" {
			return nil, errors.New("failed to register user in keycloak: " + err.Error())
		}
	}

	if err := s.RegisterSandboxUser(userName); err != nil {
		return nil, err
	}

	userToken, err := s.GetKeycloakToken(DEFAULT_KEYCLOAK_TEST_CLIENT_ID, userName, userName, DEFAULT_KEYCLOAK_TESTING_REALM)
	if err != nil {
		return nil, err
	}

	return s.GetKubeconfigPathForSpecificUser(toolchainApiUrl, userName, kubeconfigPath, userToken)
}

func (s *SandboxController) GetKubeconfigPathForSpecificUser(toolchainApiUrl string, userName string, kubeconfigPath string, keycloakAuth *KeycloakAuth) (*SandboxUserAuthInfo, error) {
	kubeconfig := api.NewConfig()
	kubeconfig.Clusters[toolchainApiUrl] = &api.Cluster{
		Server:                toolchainApiUrl,
		InsecureSkipTLSVerify: true,
	}
	kubeconfig.Contexts[fmt.Sprintf("%s/%s/%s", userName, toolchainApiUrl, userName)] = &api.Context{
		Cluster:   toolchainApiUrl,
		Namespace: fmt.Sprintf("%s-tenant", userName),
		AuthInfo:  fmt.Sprintf("%s/%s", userName, toolchainApiUrl),
	}
	kubeconfig.AuthInfos[fmt.Sprintf("%s/%s", userName, toolchainApiUrl)] = &api.AuthInfo{
		Token: keycloakAuth.AccessToken,
	}
	kubeconfig.CurrentContext = fmt.Sprintf("%s/%s/%s", userName, toolchainApiUrl, userName)

	err := clientcmd.WriteToFile(*kubeconfig, kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("error writing sandbox user kubeconfig to %s path: %v", kubeconfigPath, err)
	}

	return &SandboxUserAuthInfo{
		UserName:       userName,
		KubeconfigPath: kubeconfigPath,
	}, nil
}

func (s *SandboxController) RegisterSandboxUser(userName string) error {
	userSignup := getUserSignupSpecs(userName)
	if err := s.KubeRest.Create(context.TODO(), userSignup); err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			klog.Infof("User %s already exists", userName)
			return nil
		}
		return err
	}

	return utils.WaitUntil(func() (done bool, err error) {
		err = s.KubeRest.Get(context.TODO(), types.NamespacedName{
			Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
			Name:      userName,
		}, userSignup)

		if err != nil {
			return false, err
		}

		klog.Info("Waiting...\n", userSignup)
		for _, condition := range userSignup.Status.Conditions {
			if condition.Type == toolchainApi.UserSignupComplete && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}, 4*time.Minute)
}

func getUserSignupSpecs(username string) *toolchainApi.UserSignup {
	return &toolchainApi.UserSignup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username,
			Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
			Annotations: map[string]string{
				"toolchain.dev.openshift.com/user-email": fmt.Sprintf("%s@user.us", username),
			},
			Labels: map[string]string{
				"toolchain.dev.openshift.com/email-hash": md5.CalcMd5(fmt.Sprintf("%s@user.us", username)),
			},
		},
		Spec: toolchainApi.UserSignupSpec{
			Userid:   username,
			Username: username,
			States: []toolchainApi.UserSignupState{
				toolchainApi.UserSignupStateApproved,
			},
		},
	}
}

func (s *SandboxController) GetOpenshiftRouteHost(namespace string, name string) (string, error) {
	route := &routev1.Route{}
	err := s.KubeRest.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, route)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s", route.Spec.Host), nil
}

func (s *SandboxController) DeleteUserSignup(userName string) (bool, error) {
	userSignup := &toolchainApi.UserSignup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
		},
	}
	if err := s.KubeRest.Delete(context.TODO(), userSignup); err != nil {
		return false, err
	}
	err := utils.WaitUntil(func() (done bool, err error) {
		err = s.KubeRest.Get(context.TODO(), types.NamespacedName{
			Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
			Name:      userName,
		}, userSignup)

		if err != nil {
			if k8sErrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}, 5*time.Minute)

	if err != nil {
		return false, err
	}

	return true, nil
}
