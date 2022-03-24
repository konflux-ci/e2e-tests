package framework

import (
	"log"
	"os"

	"github.com/averageflow/gohooks/v2/gohooks"
	"gopkg.in/yaml.v2"
)

func SendWebhook(webhookConfig string) {
	cfg, err := LoadConfig(webhookConfig)
	if err != nil {
		log.Fatal(err)
	}
	path, err := os.Executable()
	if err != nil {
		log.Println(err)
	}

	//Create webhook
	hook := &gohooks.GoHook{}
	w := Webhook{Path: path}
	w.RepositoryURL = cfg.WebhookConfig.RepositoryURL
	w.Repository.FullName = cfg.WebhookConfig.Repository.FullName
	saltSecret := cfg.WebhookConfig.SaltSecret
	hook.Create(w, path, saltSecret)

	//Send webhook
	resp, err := hook.Send(cfg.WebhookConfig.WebhookTarget)
	if err != nil {
		log.Fatal("Error sending webhook: ", err)
	}
	log.Println("Webhook response: ", resp)
}

// LoadConfig returns a decoded Config struct
func LoadConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// Config struct for webhook config
type Config struct {
	WebhookConfig struct {
		SaltSecret    string `yaml:"saltSecret"`
		WebhookTarget string `yaml:"webhookTarget"`
		RepositoryURL string `yaml:"repositoryURL"`
		Repository    struct {
			FullName string `yaml:"fullName"`
		} `yaml:"repository"`
	} `yaml:"webhookConfig"`
}

// Webhook struct for sending
type Webhook struct {
	Path          string `json:"path"`
	RepositoryURL string `json:"repository_url"`
	Repository    struct {
		FullName string `json:"full_name"`
	}
}
