package config

// Set of tests to run in appstudio
type TestSpec struct {
	// The test name corresponding to an application
	Name string `yaml:"name"`

	// Indicate if a test can be skipped, by default is true
	Skip bool `yaml:"skip,omitempty"`

	// Indicate if a test is to be run against stage
	Stage bool `yaml:"stage,omitempty"`

	// Name of the application created in the cluster
	ApplicationName string `yaml:"applicationName"`

	// Set of components with own specs
	Components []ComponentSpec `yaml:"components"`
}

// Set k8s resource specific properties
type K8sSpec struct {
	// If set, will scale the replicas to the desired number
	// This is a pointer to distinguish between explicit zero and not specified.
	Replicas int32 `yaml:"replicas,omitempty"`
}

// Specs for a specific component to create in AppStudio
type ComponentSpec struct {
	// Test Default build (using PaC)
	BuildSpec *BuildSpec `yaml:"build"`

	// The component name which will be created
	Name string `yaml:"name"`

	// It indicates if the component comes from a private source like quay or github.
	Private bool `yaml:"private"`

	// Indicate the container value
	ContainerSource string `yaml:"containerSource,omitempty"`

	// Component language
	Language string `yaml:"language"`

	// Repository URL from where component will be created
	GitSourceUrl string `yaml:"gitSourceUrl,omitempty"`

	// Repository branch
	GitSourceRevision string `yaml:"gitSourceRevision,omitempty"`

	// Relative path inside the repository containing the component
	GitSourceContext string `yaml:"gitSourceContext,omitempty"`

	GitSourceDefaultBranchName string `yaml:"gitSourceDefaultBranchName,omitempty"`

	// Relative path of the docker file in the repository
	DockerFilePath string `yaml:"dockerFilePath,omitempty"`

	// An endpoint where the framework can ping to see if a component was deployed successfully
	HealthEndpoint string `yaml:"healthz"`

	// Set k8s resource specific properties
	K8sSpec *K8sSpec `yaml:"spec,omitempty"`

	// Skip the deployment of a component in case not needed to deploy
	SkipDeploymentCheck bool `yaml:"skipDeploy,omitempty"`
}

type TestScenarioSpec struct {
	GitURL      string
	GitRevision string
	TestPath    string
}

type BuildSpec struct {
	// Integration test config
	TestScenario TestScenarioSpec `yaml:"testScenario,omitempty"`
}
