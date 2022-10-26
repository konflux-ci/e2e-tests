package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
)

var (
	grantType    = "refresh_token"
	clientId     = "cloud-services"
	redHatSSOUrl = "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"
)

type RedHatSSOToken struct {
	AccessToken string `json:"access_token"`
}

// Obtain access_token from redhat SSO
func GetRedHatSSOAccessToken() (*RedHatSSOToken, error) {
	rhSSO := &RedHatSSOToken{}
	client := &http.Client{}

	if GetEnv("OFFLINE_TOKEN", "") == "" {
		return nil, errors.New("OFFLINE_TOKEN environment is not defined")
	}

	var data = strings.NewReader(`grant_type=` + grantType + `&client_id=` + clientId + `&refresh_token=` + GetEnv(constants.OFFLINE_TOKEN_ENV, ""))

	req, err := http.NewRequest(http.MethodPost, redHatSSOUrl, data)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(rhSSO); err != nil {
		return nil, err
	}

	return rhSSO, nil
}
