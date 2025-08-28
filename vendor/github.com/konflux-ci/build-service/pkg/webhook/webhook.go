package webhook

import (
	"encoding/json"
	"strings"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log logr.Logger = ctrl.Log.WithName("webhook")

type WebhookURLLoader interface {
	Load(repositoryUrl string) string
}

type ConfigWebhookURLLoader struct {
	// Prefix to target url mapping
	mapping map[string]string
}

func NewConfigWebhookURLLoader(mapping map[string]string) ConfigWebhookURLLoader {
	return ConfigWebhookURLLoader{mapping: mapping}
}

/*
	Load implements WebhookURLLoader.

Find the longest prefix match of `repositoryUrl“ and the keys of `mapping`,
and return the value of that key.
*/
func (c ConfigWebhookURLLoader) Load(repositoryUrl string) string {
	longestPrefixLen := 0
	matchedTarget := ""
	for prefix, target := range c.mapping {
		if strings.HasPrefix(repositoryUrl, prefix) && len(prefix) > longestPrefixLen {
			longestPrefixLen = len(prefix)
			matchedTarget = target
		}
	}

	// Provide a default using the empty string
	if matchedTarget == "" {
		if val, ok := c.mapping[""]; ok {
			matchedTarget = val
		}
	}

	return matchedTarget
}

var _ WebhookURLLoader = ConfigWebhookURLLoader{}

type FileReader func(name string) ([]byte, error)

// Load the prefix to target url from a file
func LoadMappingFromFile(path string, fileReader FileReader) (map[string]string, error) {
	if path == "" {
		log.Info("Webhook config was not provided")
		return map[string]string{}, nil
	}

	content, err := fileReader(path)
	if err != nil {
		return nil, err
	}

	var mapping map[string]string
	err = json.Unmarshal(content, &mapping)
	if err != nil {
		return nil, err
	}

	log.Info("Using webhook config", "config", mapping)

	return mapping, nil
}
