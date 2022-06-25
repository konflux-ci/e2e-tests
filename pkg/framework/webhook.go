package framework

import (
	"os"

	"github.com/averageflow/gohooks/v2/gohooks"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

	"path/filepath"
)

// Config struct for webhook config
type Config struct {
	WebhookConfig `yaml:"webhookConfig"`
}

// Webhook config struct
type WebhookConfig struct {
	SaltSecret        string `yaml:"saltSecret"`
	WebhookTarget     string `yaml:"webhookTarget"`
	RepositoryURL     string `yaml:"repositoryURL"`
	RepositoryWebhook `yaml:"repository"`
}

// RepositoryWebhook config struct
type RepositoryWebhook struct {
	FullName   string `yaml:"fullName"`
	PullNumber string `yaml:"pullNumber"`
}

// Webhook struct for sending
type Webhook struct {
	Path          string `json:"path"`
	RepositoryURL string `json:"repository_url"`
	Repository    `json:"repository"`
}

// Repository struct for sending
type Repository struct {
	FullName   string `json:"full_name"`
	PullNumber string `json:"pull_number"`
}

// Send webhook
func SendWebhook(webhookConfig string) {
	cfg, err := LoadConfig(webhookConfig)
	if err != nil {
		klog.Fatal(err)
	}
	path, err := os.Executable()
	if err != nil {
		klog.Info(err)
	}

	//Create webhook
	hook := &gohooks.GoHook{}
	w := Webhook{Path: path}
	w.RepositoryURL = cfg.WebhookConfig.RepositoryURL
	w.Repository.FullName = cfg.WebhookConfig.RepositoryWebhook.FullName
	w.Repository.PullNumber = cfg.WebhookConfig.RepositoryWebhook.PullNumber
	saltSecret := cfg.WebhookConfig.SaltSecret
	hook.Create(w, path, saltSecret)

	//Send webhook
	resp, err := hook.Send(cfg.WebhookConfig.WebhookTarget)
	if err != nil {
		klog.Fatal("Error sending webhook: ", err)
	}
	klog.Info("Webhook response: ", resp)
}

// LoadConfig returns a decoded Config struct
func LoadConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(filepath.Clean(configPath))
	if err != nil {
		return nil, err
	}

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	if err := file.Close(); err != nil {
		klog.Fatal("Error closing file: %s\n", err)
	}

	return config, nil
}
