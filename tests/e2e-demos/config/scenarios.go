package config

// All multiple components scenarios are supported in the next jira: https://issues.redhat.com/browse/DEVHAS-305
const (
	MultiComponentWithoutDockerFileAndDevfile     = "multi-component scenario with components without devfile or dockerfile"
	MultiComponentWithAllSupportedImportScenarios = "multi-component scenario with all supported import components"
	MultiComponentWithDevfileAndDockerfile        = "multi-component scenario with components with devfile or dockerfile or both"
	MultiComponentWithUnsupportedRuntime          = "multi-component scenario with a component with a supported runtime and another unsuported"
)

var TestScenarios = []TestSpec{
	{
		Name:            "DEVHAS-234: creates an application with springboot component from RHTAP samples",
		ApplicationName: "e2e-springboot",
		Components: []ComponentSpec{
			{
				Name:              "springboot-component",
				ContainerSource:   "",
				Language:          "Java",
				GitSourceUrl:      "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            "DEVHAS-234: creates an application with python component from RHTAP samples",
		ApplicationName: "e2e-python-personal",
		Components: []ComponentSpec{
			{
				Name:              "component-python-flask",
				ContainerSource:   "",
				Language:          "Python",
				GitSourceUrl:      "https://github.com/devfile-samples/devfile-sample-python-basic.git",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            "DEVHAS-234: creates an application with dotnet component from RHTAP samples",
		ApplicationName: "e2e-dotnet",
		Components: []ComponentSpec{
			{
				Name:              "dotnet-component",
				ContainerSource:   "",
				Language:          "dotNet",
				GitSourceUrl:      "https://github.com/devfile-samples/devfile-sample-dotnet60-basic",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            "DEVHAS-234: create an nodejs application without dockerfile",
		ApplicationName: "e2e-nodejs",
		Components: []ComponentSpec{
			{
				Name:              "nodejs-no-dockerfile",
				ContainerSource:   "",
				Language:          "JavaScript",
				GitSourceUrl:      "https://github.com/nodeshift-starters/nodejs-health-check.git",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/live",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
			{
				Name:              "nodejs-priv",
				Private:           true,
				ContainerSource:   "",
				Language:          "JavaScript",
				GitSourceUrl:      "https://github.com/redhat-appstudio-qe-bot/nodejs-health-check.git",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/live",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            "DEVHAS-234: create an golang application",
		ApplicationName: "e2e-golang",
		Components: []ComponentSpec{
			{
				Name:              "golang-dockerfile",
				ContainerSource:   "",
				Language:          "Go",
				GitSourceUrl:      "https://github.com/devfile-samples/devfile-sample-go-basic",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            "DEVHAS-234: create an nodejs application with dockerfile and devfile",
		ApplicationName: "e2e-nodejs",
		Components: []ComponentSpec{
			{
				Name:              "nodejs-dockerfile",
				ContainerSource:   "",
				Language:          "JavaScript",
				GitSourceUrl:      "https://github.com/nodeshift-starters/devfile-sample",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            "DEVHAS-234: create an application with quarkus component",
		ApplicationName: "quarkus",
		Components: []ComponentSpec{
			{
				Name:              "quarkus-devfile",
				ContainerSource:   "",
				Language:          "Java",
				GitSourceUrl:      "https://github.com/devfile-samples/devfile-sample-code-with-quarkus.git",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/hello-resteasy",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            "DEVHAS-234: create an application with branch and context dir",
		ApplicationName: "e2e-java",
		Components: []ComponentSpec{
			{
				Name:              "component-devfile-java-sample",
				ContainerSource:   "",
				Language:          "Java",
				GitSourceUrl:      "https://github.com/redhat-appstudio-qe/java-sample",
				GitSourceRevision: "testing",
				GitSourceContext:  "java/java",
				HealthEndpoint:    "/",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            "DEVHAS-234: creates quarkus application(with dockerfile but not devfile) which is not included in AppStudio starter stack",
		ApplicationName: "status-quarkus-io",
		Components: []ComponentSpec{
			{
				Name:                "status-quarkus-io",
				ContainerSource:     "",
				Language:            "Java",
				GitSourceUrl:        "https://github.com/quarkusio/status.quarkus.io.git",
				GitSourceRevision:   "",
				GitSourceContext:    "",
				HealthEndpoint:      "/",
				K8sSpec:             K8sSpec{Replicas: 0},
				SkipDeploymentCheck: true,
			},
		},
	},
	{
		Name:            "DEVHAS-234: creates nodejs application(without dockerfile and devfile) which is not included in AppStudio starter stack",
		ApplicationName: "nodejs-users",
		Components: []ComponentSpec{
			{
				Name:              "nodejs-user",
				ContainerSource:   "",
				Language:          "JavaScript",
				GitSourceUrl:      "https://github.com/redhat-appstudio-qe/simple-nodejs-app.git",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/users",
				K8sSpec:           K8sSpec{Replicas: 0},
			},
		},
	},
	{
		Name:            MultiComponentWithoutDockerFileAndDevfile,
		ApplicationName: "mc-quality-dashboard",
		// We need to skip for now deployment checks of quality dashboard until RHTAP support secrets
		Components: []ComponentSpec{
			{
				Name:                "mc-withdockerfile-withoutdevfile",
				SkipDeploymentCheck: true,
				GitSourceUrl:        "https://github.com/redhat-appstudio/quality-dashboard.git",
			},
		},
	},
	{
		Name:            MultiComponentWithDevfileAndDockerfile,
		ApplicationName: "mc-two-scenarios",
		Components: []ComponentSpec{
			{
				Name:         "mc-two-scenarios",
				GitSourceUrl: "https://github.com/redhat-appstudio-qe/rhtap-devfile-multi-component.git",
			},
		},
	},
	{
		Name:            MultiComponentWithAllSupportedImportScenarios,
		ApplicationName: "mc-three-scenarios",
		Components: []ComponentSpec{
			{
				Name:         "mc-three-scenarios",
				GitSourceUrl: "https://github.com/redhat-appstudio-qe/rhtap-three-component-scenarios.git",
			},
		},
	},
	{
		Name:            MultiComponentWithUnsupportedRuntime,
		ApplicationName: "mc-unsupported-runtime",
		Components: []ComponentSpec{
			{
				Name:         "mc-unsuported-runtime",
				GitSourceUrl: "https://github.com/redhat-appstudio-qe/rhtap-mc-unsuported-runtime.git",
			},
		},
	},
	{
		Name:            "DEVHAS-234: creates quarkus application(with dockerfile but not devfile) which is not included in AppStudio starter stack",
		ApplicationName: "status-quarkus-io",
		Components: []ComponentSpec{
			{
				Name:                "status-quarkus-io",
				ContainerSource:     "",
				Language:            "Java",
				GitSourceUrl:        "https://github.com/quarkusio/status.quarkus.io.git",
				GitSourceRevision:   "",
				GitSourceContext:    "",
				HealthEndpoint:      "/",
				K8sSpec:             K8sSpec{Replicas: 0},
				SkipDeploymentCheck: true,
			},
		},
	},
}
