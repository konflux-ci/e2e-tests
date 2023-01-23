package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	toolchainApi "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-e2e/testsupport/md5"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (k *CustomClient) GetOpenshiftRouteHost(namespace string, name string) (string, error) {
	route := &routev1.Route{}
	err := k.KubeRest().Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, route)
	if err != nil {
		return "", err
	}
	return route.Spec.Host, nil
}

func (k *CustomClient) getKeycloakUrl() (string, error) {
	keycloakHost, err := k.GetOpenshiftRouteHost(DEFAULT_KEYCLOAK_NAMESPACE, DEFAULT_KEYCLOAK_INSTANCE_NAME)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s", keycloakHost), nil
}

func (k *CustomClient) getToolchainApiUrl() (string, error) {
	toolchainHost, err := k.GetOpenshiftRouteHost(DEFAULT_TOOLCHAIN_NAMESPACE, DEFAULT_TOOLCHAIN_INSTANCE_NAME)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s:443", toolchainHost), nil
}

func getUserSignupSpecs(username string) *toolchainApi.UserSignup {
	return &toolchainApi.UserSignup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DEFAULT_KEYCLOAK_USERNAME,
			Namespace: DEFAULT_TOOLCHAIN_NAMESPACE,
			Annotations: map[string]string{
				"toolchain.dev.openshift.com/user-email": fmt.Sprintf("%s@user.us", username),
			},
			Labels: map[string]string{
				"toolchain.dev.openshift.com/email-hash": md5.CalcMd5(fmt.Sprintf("%s@user.us", username)),
			},
		},
		Spec: toolchainApi.UserSignupSpec{
			Userid:   constants.DEFAULT_KEYCLOAK_USERNAME,
			Username: constants.DEFAULT_KEYCLOAK_USERNAME,
			States: []toolchainApi.UserSignupState{
				toolchainApi.UserSignupStateApproved,
			},
		},
	}
}

func GetKeycloakToken(keycloakUrl, username, password, client_id string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	data := url.Values{}
	data.Set("client_id", client_id)
	data.Set("password", username)
	data.Set("username", password)
	data.Set("grant_type", "password")

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/auth/realms/testrealm/protocol/openid-connect/token", keycloakUrl), bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	fmt.Printf("Response: %+v\n", resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	token := struct {
		AccessToken string `json:"access_token"`
	}{}

	if err := json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}
