package config

type WorkflowSpec struct {
	PolarionID string     `yaml:"polarionId,omitempty"`
	Tests      []TestSpec `yaml:"tests"`
}

type TestSpec struct {
	Name            string          `yaml:"name"`
	ApplicationName string          `yaml:"applicationName"`
	Components      []ComponentSpec `yaml:"components"`
}

type ComponentSpec struct {
	Name             string `yaml:"name"`
	Type             string `yaml:"type"`
	DevfileSample    string `yaml:"devfileSample,omitempty"`
	ContainerSource  string `yaml:"containerSource,omitempty"`
	DockerFileSource string `yaml:"dockerFileSource,omitempty"`
}
