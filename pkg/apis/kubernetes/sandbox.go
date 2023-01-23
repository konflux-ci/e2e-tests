package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
)

const (
	DEFAULT_KEYCLOAK_INSTANCE_NAME = "keycloak"
	DEFAULT_KEYCLOAK_NAMESPACE     = "dev-sso"

	DEFAULT_TOOLCHAIN_INSTANCE_NAME = "api"
	DEFAULT_TOOLCHAIN_NAMESPACE     = "toolchain-host-operator"
)

// Obtains user's keycloak token and generates new kubeconfig for this user against toolchain proxy endpoint.
// Configurable via env variables (default in brackets): USER_KUBE_CONFIG_PATH["$(pwd)/user.kubeconfig"], "KC_USERNAME"["user1"],
// KC_PASSWORD["user1"], KC_CLIENT_ID["sandbox-public"], KEYCLOAK_URL[obtained dynamically - route `keycloak` in `dev-sso` namespace],
// TOOLCHAIN_API_URL[obtained dynamically - route `api` in `toolchain-host-operator` namespace]
func (k *CustomClient) GenerateSandboxUserKubeconfig() (kubeconfigPath string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	klog.Info("waiting for keycloak instance to be ready")
	if err := k.waitForKeyckloakToBeReady(); err != nil {
		return "", err
	}

	kubeconfigPath = utils.GetEnv(constants.USER_USER_KUBE_CONFIG_PATH_ENV, fmt.Sprintf("%s/tmp/user.kubeconfig", wd))
	keycloakUsername := utils.GetEnv(constants.KEYCLOAK_USERNAME_ENV, constants.DEFAULT_KEYCLOAK_USERNAME)

	keycloakUrl, err := utils.GetEnvOrFunc(constants.KEYCLOAK_URL_ENV, k.getKeycloakUrl)
	fmt.Println(keycloakUrl)
	if err != nil {
		return "", err
	}

	token, err := GetKeycloakToken(keycloakUrl, keycloakUsername, utils.GetEnv(constants.KEYCLOAK_USER_PASSWORD_ENV, constants.DEFAULT_KEYCLOAK_PASSWORD), utils.GetEnv(constants.KEYCLOAK_CLIENT_ID_ENV, constants.DEFAULT_KEYCLOAK_CLIENT_ID))
	if err != nil {
		return "", err
	}

	toolchainApiUrl, err := utils.GetEnvOrFunc(constants.TOOLCHAIN_API_URL_ENV, k.getToolchainApiUrl)
	if err != nil {
		return "", err
	}

	kubeconfig := api.NewConfig()
	kubeconfig.Clusters[toolchainApiUrl] = &api.Cluster{
		Server:                toolchainApiUrl,
		InsecureSkipTLSVerify: true,
	}
	kubeconfig.Contexts[fmt.Sprintf("%s/%s/%s", keycloakUsername, toolchainApiUrl, keycloakUsername)] = &api.Context{
		Cluster:   toolchainApiUrl,
		Namespace: keycloakUsername,
		AuthInfo:  fmt.Sprintf("%s/%s", keycloakUsername, toolchainApiUrl),
	}
	kubeconfig.AuthInfos[fmt.Sprintf("%s/%s", keycloakUsername, toolchainApiUrl)] = &api.AuthInfo{
		Token: token,
	}
	kubeconfig.CurrentContext = fmt.Sprintf("%s/%s/%s", keycloakUsername, toolchainApiUrl, keycloakUsername)
	err = clientcmd.WriteToFile(*kubeconfig, kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("error writing sandbox user kubeconfig to %s path: %v", kubeconfigPath, err)
	}

	return kubeconfigPath, nil
}

// Creates preapproved userSignup CR cluster and waits for its reconcilliation
func (k *CustomClient) RegisterSandboxUser() error {
	userSignup := getUserSignupSpecs(utils.GetEnv(constants.KEYCLOAK_USERNAME_ENV, constants.DEFAULT_KEYCLOAK_USERNAME))

	klog.Infof("Creating: %v+\n", userSignup)

	if err := k.KubeRest().Create(context.TODO(), userSignup); err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			klog.Infof("User already exists: %v+\n", userSignup)

			return nil
		}
		return err
	}

	return utils.WaitUntil(func() (done bool, err error) {
		err = k.KubeRest().Get(context.TODO(), types.NamespacedName{
			Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
			Name:      utils.GetEnv(constants.KEYCLOAK_USERNAME_ENV, constants.DEFAULT_KEYCLOAK_USERNAME),
		}, userSignup)

		if err != nil {
			return false, err
		}

		klog.Infof("Waiting. %+v\n", userSignup)
		for _, condition := range userSignup.Status.Conditions {
			if condition.Type == "Complete" && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}, 2*time.Minute)
}

// Wait for the keycloak instance to be ready
func (k *CustomClient) waitForKeyckloakToBeReady() error {
	return utils.WaitUntil(func() (done bool, err error) {
		sets, err := k.KubeInterface().AppsV1().StatefulSets(DEFAULT_KEYCLOAK_NAMESPACE).Get(context.Background(), DEFAULT_KEYCLOAK_INSTANCE_NAME, v1.GetOptions{})

		if err != nil {
			return false, err
		}

		if sets.Status.ReadyReplicas == *sets.Spec.Replicas {
			return true, nil
		}

		return false, nil
	}, 10*time.Minute)
}
