package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	routev1 "github.com/openshift/api/route/v1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	SPRAYPROXY_NAMESPACE = "sprayproxy"
	ROUTE_NAME            = "sprayproxy-route"
	PAC_NAMESPACE = "openshift-pipelines"
	PAC_ROUTE_NAME = "pipelines-as-code-controller"
)

type SprayProxy struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewSprayProxy(url string, token string) *SprayProxy {
	return &SprayProxy{
		BaseURL: url,
		Token:   token,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// #nosec G402
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

func (s *SprayProxy) RegisterServer(hostUrl string) (string, error) {
	result, err := s.sendRequest(hostUrl, http.MethodPost)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (s *SprayProxy) UnegisterServer(hostUrl string) (string, error) {
	result, err := s.sendRequest(hostUrl, http.MethodDelete)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (s *SprayProxy) GetServers() (string, error) {
	result, err := s.sendRequest("", http.MethodGet)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (s *SprayProxy) sendRequest(hostUrl string, httpMethod string) (string, error) {
	requestURL := s.BaseURL + "/backends"

	data := make(map[string]string)
	data["url"] = hostUrl
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
		return "", fmt.Errorf("failed to access SprayProxy server with status code: %d and\nbody: %s", res.StatusCode, string(body))
	}

	defer res.Body.Close()

	return string(body), err
}

func GetPod() (*corev1.Pod, error) {
	k8sClient, err := kubeCl.NewAdminKubernetesClient()
	if err != nil {
		return nil, err
	}
	podList, err := k8sClient.KubeInterface().CoreV1().Pods(SPRAYPROXY_NAMESPACE).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return &pod, nil
		}
	}
	return nil, fmt.Errorf("no running pod found in namespace %s", SPRAYPROXY_NAMESPACE)
}

func GetPacRoute() (*routev1.Route, error) {
	k8sClient, err := kubeCl.NewAdminKubernetesClient()
	if err != nil {
		return nil, err
	}

	namespaceName := types.NamespacedName{
		Name:      PAC_ROUTE_NAME,
		Namespace: PAC_NAMESPACE,
	}

	route := &routev1.Route{}
	err = k8sClient.KubeRest().Get(context.TODO(), namespaceName, route)
	if err != nil {
		return &routev1.Route{}, err
	}
	return route, nil
}