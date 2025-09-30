package build

import (
	"encoding/base64"
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

	decodedDockerConfig, err := base64.StdEncoding.DecodeString(rawDockerConfig)
	if err != nil {
		log.Printf("error decoding docker config: %v", err)
		log.Print("will attempt to parse it as JSON")
	}

	var config DockerConfig
	if err := json.Unmarshal(decodedDockerConfig, &config); err != nil {
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
