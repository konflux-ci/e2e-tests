package config

type WorkflowSpec struct {
	Tests []TestSpec `yaml:"tests"`
}

type TestSpec struct {
	Name            string          `yaml:"name"`
	ApplicationName string          `yaml:"applicationName"`
	Components      []ComponentSpec `yaml:"components"`
}

type ComponentSpec struct {
	Name            string `yaml:"name"`
	Type            string `yaml:"type"`
	ContainerSource string `yaml:"containerSource,omitempty"`
	Devfilesource   string `yaml:"devfileSource,omitempty"`
	GitSourceUrl    string `yaml:"gitSourceUrl,omitempty"`
	HealthEndpoint  string `yaml:"healthz"`
}
