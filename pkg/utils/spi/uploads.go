package spi

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UploadWithK8sSecret returns the requested Secret object
func (s *SPIController) UploadWithK8sSecret(secretName, namespace, spiTokenName, providerURL, username, tokenData string) (*v1.Secret, error) {
	k8sSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      secretName,
			Labels: map[string]string{
				"spi.appstudio.redhat.com/upload-secret": "token",
			},
		},
		Type: "Opaque",
		StringData: map[string]string{
			"tokenData": tokenData,
		},
	}
	if spiTokenName != "" {
		k8sSecret.StringData["spiTokenName"] = spiTokenName
	}
	if providerURL != "" {
		k8sSecret.StringData["providerUrl"] = providerURL
	}
	if username != "" {
		k8sSecret.StringData["userName"] = username
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()
	err := s.KubeRest().Create(ctx, k8sSecret)
	if err != nil {
		return nil, err
	}
	return k8sSecret, nil
}

// Perform http POST call to upload a token at the given upload URL
func (s *SPIController) UploadWithRestEndpoint(uploadURL string, oauthCredentials string, bearerToken string) (int, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, err := http.NewRequest("POST", uploadURL, bytes.NewBuffer([]byte(oauthCredentials)))
	if err != nil {
		return 0, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", string(bearerToken)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp.StatusCode, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
