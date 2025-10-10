package build

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type RegistryAuth struct {
	Auth string `json:"auth"`
}

type DockerConfig struct {
	Auths map[string]RegistryAuth `json:"auths"`
}

func GetDockerAuth() (string, error) {
	rawDockerConfig := os.Getenv("QUAY_TOKEN")

	if rawDockerConfig == "" {
		return "", fmt.Errorf("docker config is empty")
	}

	var config DockerConfig
	if err := json.Unmarshal([]byte(rawDockerConfig), &config); err != nil {
		return "", fmt.Errorf("error parsing docker config JSON: %v", err)
	}

	var authToken string

	if auth, exists := config.Auths["quay.io"]; exists {
		log.Print("using quay.io auth")
		authToken = auth.Auth
	}

	if auth, exists := config.Auths["quay.io/konflux-ci"]; exists {
		log.Print("using quay.io/konflux-ci auth")
		authToken = auth.Auth
	}

	return authToken, nil
}
