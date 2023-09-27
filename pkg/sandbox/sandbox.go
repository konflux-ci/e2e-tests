package sandbox

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	toolchainApi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-e2e/testsupport/md5"
	. "github.com/onsi/ginkgo/v2"
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

	// Returns the username namespace provisioned by toolchain
	UserNamespace string

	// Add a description about kubeconfigpath
	KubeconfigPath string

	// Url of user api to access kubernetes host
	ProxyUrl string

	// User token used as bearer to authenticate against kubernetes host
	UserToken string
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
	client := &http.Client{Transport: LoggingRoundTripper{&http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	},
	}
	return client, nil
}

// same as NewKeyCloakApiController but for stage
func NewDevSandboxStageController() (*SandboxController, error) {
	newHttpClient, err := NewHttpClient()
	if err != nil {
		return nil, err
	}

	return &SandboxController{
		HttpClient: newHttpClient,
		KubeClient: nil,
		KubeRest:   nil,
	}, nil
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

// This type implements the http.RoundTripper interface
type LoggingRoundTripper struct {
	Proxied http.RoundTripper
}

func (lrt LoggingRoundTripper) RoundTrip(req *http.Request) (res *http.Response, e error) {
	// Do "before sending requests" actions here.
	GinkgoWriter.Printf("Sandbox proxy sending request to %v:%v %v\n", req.URL, req.Header, req.Body)

	// Send the request, get the response (or the error)
	res, e = lrt.Proxied.RoundTrip(req)

	// Handle the result.
	if e != nil {
		GinkgoWriter.Printf("Sandbox proxy error: %v", e)
	} else {
		GinkgoWriter.Printf("Sandbox proxy received %v response\n", res.Status)
	}

	return res, e
}

// ReconcileUserCreation create a user in sandbox and return a valid kubeconfig for user to be used for the tests
func (s *SandboxController) ReconcileUserCreationStage(userName, toolchainApiUrl, keycloakUrl, offlineToken string) (*SandboxUserAuthInfo, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	kubeconfigPath := utils.GetEnv(constants.USER_KUBE_CONFIG_PATH_ENV, fmt.Sprintf("%s/tmp/%s.kubeconfig", wd, userName))

	userToken, err := s.GetKeycloakTokenStage(userName, keycloakUrl, offlineToken)
	if err != nil {
		return nil, err
	}

	return s.GetKubeconfigPathForSpecificUser(true, toolchainApiUrl, userName, kubeconfigPath, userToken)
}

// ReconcileUserCreation create a user in sandbox and return a valid kubeconfig for user to be used for the tests
func (s *SandboxController) ReconcileUserCreation(userName string) (*SandboxUserAuthInfo, error) {
	var compliantUsername string
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	kubeconfigPath := utils.GetEnv(constants.USER_KUBE_CONFIG_PATH_ENV, fmt.Sprintf("%s/tmp/%s.kubeconfig", wd, userName))

	toolchainApiUrl, err := s.GetOpenshiftRouteHost(DEFAULT_TOOLCHAIN_NAMESPACE, DEFAULT_TOOLCHAIN_INSTANCE_NAME)
	if err != nil {
		return nil, err
	}

	if s.KeycloakUrl, err = s.GetOpenshiftRouteHost(DEFAULT_KEYCLOAK_NAMESPACE, DEFAULT_KEYCLOAK_INSTANCE_NAME); err != nil {
		return nil, err
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

	if compliantUsername, err = s.RegisterSandboxUser(userName); err != nil {
		return nil, err
	}

	if !s.KeycloakUserExists(DEFAULT_KEYCLOAK_TESTING_REALM, adminToken.AccessToken, userName) {
		registerUser, err := s.RegisterKeycloakUser(userName, adminToken.AccessToken, DEFAULT_KEYCLOAK_TESTING_REALM)
		if err != nil && registerUser.Username == "" {
			return nil, errors.New("failed to register user in keycloak: " + err.Error())
		}
	}

	userToken, err := s.GetKeycloakToken(DEFAULT_KEYCLOAK_TEST_CLIENT_ID, userName, userName, DEFAULT_KEYCLOAK_TESTING_REALM)
	if err != nil {
		return nil, err
	}

	return s.GetKubeconfigPathForSpecificUser(false, toolchainApiUrl, compliantUsername, kubeconfigPath, userToken)
}

func (s *SandboxController) GetKubeconfigPathForSpecificUser(isStage bool, toolchainApiUrl string, userName string, kubeconfigPath string, keycloakAuth *KeycloakAuth) (*SandboxUserAuthInfo, error) {
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
	var ns string
	if isStage {
		ns = fmt.Sprintf("%s-tenant", userName)
	} else {
		ns, err = s.GetUserProvisionedNamespace(userName)
		if err != nil {
			return nil, fmt.Errorf("error getting provisioned usernamespace: %v", err)
		}
	}

	return &SandboxUserAuthInfo{
		UserName:       userName,
		UserNamespace:  ns,
		KubeconfigPath: kubeconfigPath,
		ProxyUrl:       toolchainApiUrl,
		UserToken:      keycloakAuth.AccessToken,
	}, nil
}

func (s *SandboxController) RegisterSandboxUser(userName string) (compliantUsername string, err error) {
	userSignup := getUserSignupSpecs(userName)

	if err := s.KubeRest.Create(context.TODO(), userSignup); err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			GinkgoWriter.Printf("User %s already exists\n", userName)
		} else {
			return "", err
		}
	}

	err = utils.WaitUntil(func() (done bool, err error) {
		err = s.KubeRest.Get(context.TODO(), types.NamespacedName{
			Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
			Name:      userName,
		}, userSignup)

		if err != nil {
			return false, err
		}

		for _, condition := range userSignup.Status.Conditions {
			if condition.Type == toolchainApi.UserSignupComplete && condition.Status == corev1.ConditionTrue {
				compliantUsername = userSignup.Status.CompliantUsername
				if len(compliantUsername) < 1 {
					GinkgoWriter.Printf("Status.CompliantUsername field in UserSignup CR %s in %s namespace is empty\n", userSignup.GetName(), userSignup.GetNamespace())
					return false, nil
				}
				return true, nil
			}
		}
		GinkgoWriter.Printf("Waiting for UserSignup %s to have condition Complete:True\n", userSignup.GetName())
		return false, nil
	}, 4*time.Minute)

	if err != nil {
		return "", err
	}
	return compliantUsername, nil

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

func (s *SandboxController) GetUserProvisionedNamespace(userName string) (namespace string, err error) {
	ns, err := s.waitForNamespaceToBeProvisioned(userName)
	if err != nil {
		return "", err
	}

	return ns, err
}

func (s *SandboxController) waitForNamespaceToBeProvisioned(userName string) (provisionedNamespace string, err error) {
	err = utils.WaitUntil(func() (done bool, err error) {
		var namespaceProvisioned bool
		userSpace := &toolchainApi.Space{}
		err = s.KubeRest.Get(context.TODO(), types.NamespacedName{
			Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
			Name:      userName,
		}, userSpace)

		if err != nil {
			return false, err
		}

		// check if a namespace with the username prefix was provisioned
		for _, pns := range userSpace.Status.ProvisionedNamespaces {
			if strings.Contains(pns.Name, userName) {
				namespaceProvisioned = true
				provisionedNamespace = pns.Name
			}
		}

		for _, condition := range userSpace.Status.Conditions {
			if condition.Reason == toolchainApi.SpaceProvisionedReason && condition.Status == corev1.ConditionTrue && namespaceProvisioned {
				return true, nil
			}
		}

		return false, nil
	}, 4*time.Minute)

	return provisionedNamespace, err
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
