package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	DEFAULT_KEYCLOAK_INSTANCE_NAME = "keycloak"
	DEFAULT_KEYCLOAK_NAMESPACE     = "dev-sso"
)

type KeycloakAuth struct {
	// An access token is a token delivered by they keycloak server, and which allows an application to access to a resource
	AccessToken string `json:"access_token"`

	//refresh token is subject to SSO Session Idle timeout (30mn -default) and SSO Session Max lifespan (10hours-default) whereas offline token never expires
	RefreshToken string `json:"refresh_token"`
}

// GetKeycloakToken return a token for admins
func (k *SandboxController) GetKeycloakToken(clientID string, userName string, password string, realm string) (keycloakAuth *KeycloakAuth, err error) {
	data := url.Values{
		"client_id":  {clientID},
		"username":   {userName},
		"password":   {password},
		"grant_type": {"password"},
	}

	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/auth/realms/%s/protocol/openid-connect/token", k.KeycloakUrl, realm), strings.NewReader(data.Encode()))

	if err != nil {
		klog.Errorf("failed to get token from keycloak: %v", err)
		return nil, err
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := k.HttpClient.Do(request)

	if err != nil || response.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get keycloak token, realm: %s, userName: %s, client-id: %s statusCode: %d, url: %s", realm, userName, clientID, response.StatusCode, k.KeycloakUrl)
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&keycloakAuth)

	return keycloakAuth, err
}

/*
RegisterKeyclokUser create a username in keycloak service and return if succeed or not
curl --location --request POST 'https://<keycloak-route>/auth/admin/realms/testrealm/users' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJyS2VyZnczU2tzV2hBUF9TeUJuMDRaRm5Pa09ITVFRRmpnOGhjaG12X3VVIn0.eyJleHAiOjE2NzQ3NTkyODksImlhdCI6MTY3NDc1OTIyOSwianRpIjoiY2ZjNzNjMjEtZDU5Mi00MmI4LTk4MWMtYjc5MjAxNzI3OTJhIiwiaXNzIjoiaHR0cHM6Ly9rZXljbG9hay1kZXYtc3NvLmFwcHMuY2x1c3Rlci05cm05ei45cm05ei5zYW5kYm94MTE3OS5vcGVudGxjLmNvbS9hdXRoL3JlYWxtcy9tYXN0ZXIiLCJzdWIiOiI4ODcxMmJjOS1kZDBiLTQxNTktOGE1Ny1mZTRjMDlmYzBhM2IiLCJ0eXAiOiJCZWFyZXIiLCJhenAiOiJhZG1pbi1jbGkiLCJzZXNzaW9uX3N0YXRlIjoiM2I3MDM5NmItMzk4Yy00MjQyLTljZDMtZGJlYjM0ZWVjYmE0IiwiYWNyIjoiMSIsInNjb3BlIjoicHJvZmlsZSBlbWFpbCIsInNpZCI6IjNiNzAzOTZiLTM5OGMtNDI0Mi05Y2QzLWRiZWIzNGVlY2JhNCIsImVtYWlsX3ZlcmlmaWVkIjpmYWxzZSwicHJlZmVycmVkX3VzZXJuYW1lIjoiYWRtaW4ifQ.GBHKQC0VZk4nEWVXDYC-Npk5Z503xlkDNbcrgd9nRTWcLZdD6HmgKnvGgoVYBssiSQyBYnAAqVQLGslbENjtohOlU4UxV0-Tsr2OpJUlKP0oMBVcna745UHAxU2JcVraVR4UkiryZbAOTJyUYKdhszqmfkGWPukTAo4lB2GO7HdfyU1UAwp8mzfLQ6WWV-LmeFjUUpwGOUed3Ztoa4DMBnVNFp7WHqoFyPO6xSTqq59ai__bJ8_8W7KfUTI6Rmfcno-6_9PtWFC8_bvs8bRBV7Xs8j4wn-7Y2-f9WTGC8EfUTacVGTf1ma-lBUEzWKodc7XH_5O18Huko3eS3RMDTA' \

	--data-raw "{
	                   "firstName":"user1",
	                   "lastName":"user1",
	                   "username":"user1",
	                   "enabled":"true",
	                   "email":"user1@test.com",
	                   "credentials":[
	                                   {
	                                      "type":"password",
	                                      "value":"user1",
	                                      "temporary":"false"
	                                   }
	                                 ]
	                 }"
*/
func (k *SandboxController) RegisterKeyclokUser(userName string, keycloakToken string, realm string) (user *KeycloakUser, err error) {
	user = k.getKeycloakUserSpec(userName)
	payload, err := json.Marshal(user)
	if err != nil {
		return user, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/auth/admin/realms/%s/users", k.KeycloakUrl, realm), bytes.NewReader(payload))
	if err != nil {
		return user, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", keycloakToken))

	resp, err := k.HttpClient.Do(req)
	if err != nil || resp.StatusCode != 201 {
		return user, fmt.Errorf("failed to create keycloak users. Status code %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	return user, err
}

// Return specs for a given user
func (k *SandboxController) getKeycloakUserSpec(userName string) *KeycloakUser {
	return &KeycloakUser{
		FirstName: userName,
		LastName:  userName,
		Username:  userName,
		Enabled:   "true",
		Email:     fmt.Sprintf("%s@test.com", userName),
		Credentials: []KeycloakUserCredentials{
			{
				Type:      "password",
				Value:     userName,
				Temporary: "false",
			},
		},
	}
}

// Add a valid description
func (s *SandboxController) IsKeycloakRunning() error {
	return utils.WaitUntil(func() (done bool, err error) {
		sets, err := s.KubeClient.AppsV1().StatefulSets(DEFAULT_KEYCLOAK_NAMESPACE).Get(context.Background(), DEFAULT_KEYCLOAK_INSTANCE_NAME, metav1.GetOptions{})

		if err != nil {
			klog.Infof("keycloak instance is not ready. Please check keycloak deployment: %v", err)

			return false, err
		}

		if sets.Status.ReadyReplicas == *sets.Spec.Replicas {
			return true, nil
		}
		klog.Info("keycloak instance is not ready. Please check keycloak deployment")

		return false, nil
	}, 5*time.Minute)
}

// Add a valid description
func (s *SandboxController) GetKeycloakAdminSecret() (adminPassword string, err error) {
	keycloakAdminSecret, err := s.KubeClient.CoreV1().Secrets(DEFAULT_KEYCLOAK_NAMESPACE).Get(context.Background(), DEFAULT_KEYCLOAK_ADMIN_SECRET, metav1.GetOptions{})

	if err != nil {
		return "", fmt.Errorf("failed to fetch keycloak secret from namespace: %s, secretName: %s", DEFAULT_KEYCLOAK_NAMESPACE, DEFAULT_KEYCLOAK_ADMIN_SECRET)
	}

	adminPassword = string(keycloakAdminSecret.Data[SECRET_KEY])
	if adminPassword == "" {
		return "", fmt.Errorf("admin password dont exist in secret %s", DEFAULT_KEYCLOAK_ADMIN_SECRET)
	}

	return adminPassword, nil
}

func (s *SandboxController) KeycloakUserExists(realm string, token string, username string) bool {
	///{realm}/users?username=toto
	///admin/realms/{my-realm}/users?search={username}
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/auth/admin/realms/%s/users?search=%s", s.KeycloakUrl, realm, username), strings.NewReader(""))

	if err != nil {
		klog.Errorf("failed to get user: %v", err)
		return false
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	response, err := s.HttpClient.Do(request)

	if err != nil || response.StatusCode != 200 {
		return false
	}
	defer response.Body.Close()

	return true
}
