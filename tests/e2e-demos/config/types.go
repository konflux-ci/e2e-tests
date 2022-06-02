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

/*
   - name: "component-a"
     type: "private"
     devfileSample: "https://somewhere"
     quayImage: "quay://"
     language: "java"

*/
type ComponentSpec struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}
