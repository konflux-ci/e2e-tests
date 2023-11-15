package sprayproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	routev1 "github.com/openshift/api/route/v1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/clients/kubernetes"
	"k8s.io/apimachinery/pkg/types"
)

func NewSprayProxyConfig(url string, token string) (*SprayProxyConfig, error) {
	pacHost, err := getPaCHost()
	if err != nil {
		return nil, fmt.Errorf("failed to get PaC host: %+v", err)
	}
	return &SprayProxyConfig{
		BaseURL: url,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// #nosec G402
					InsecureSkipVerify: true,
				},
			},
		},
		PaCHost: pacHost,
		Token:   token,
	}, nil
}

func (s *SprayProxyConfig) RegisterServer() (string, error) {
	result, err := s.sendRequest(http.MethodPost)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (s *SprayProxyConfig) UnregisterServer() (string, error) {
	result, err := s.sendRequest(http.MethodDelete)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (s *SprayProxyConfig) GetServers() (string, error) {
	result, err := s.sendRequest(http.MethodGet)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (s *SprayProxyConfig) sendRequest(httpMethod string) (string, error) {
	requestURL := s.BaseURL + "/backends"

	data := make(map[string]string)
	data["url"] = s.PaCHost
	bytesData, _ := json.Marshal(data)

	req, err := http.NewRequest(httpMethod, requestURL, bytes.NewReader(bytesData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.Token))

	res, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to access SprayProxy server with status code: %d and body: %s", res.StatusCode, string(body))
	}

	defer res.Body.Close()

	return string(body), err
}

func getPaCHost() (string, error) {
	k8sClient, err := kubeCl.NewAdminKubernetesClient()
	if err != nil {
		return "", err
	}

	namespaceName := types.NamespacedName{
		Name:      pacRouteName,
		Namespace: pacNamespace,
	}

	route := &routev1.Route{}
	err = k8sClient.KubeRest().Get(context.Background(), namespaceName, route)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s", route.Spec.Host), nil
}
