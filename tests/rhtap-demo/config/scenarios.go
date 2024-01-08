package config

import (
	"fmt"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

// All multiple components scenarios are supported in the next jira: https://issues.redhat.com/browse/DEVHAS-305
const (
	MultiComponentWithoutDockerFileAndDevfile     = "multi-component scenario with components without devfile or dockerfile"
	MultiComponentWithAllSupportedImportScenarios = "multi-component scenario with all supported import components"
	MultiComponentWithDevfileAndDockerfile        = "multi-component scenario with components with devfile or dockerfile or both"
	MultiComponentWithUnsupportedRuntime          = "multi-component scenario with a component with a supported runtime and another unsuported"
)

var TestScenarios = []TestSpec{
	{
		Name:            "Maven project - Simple and Advanced build",
		ApplicationName: "rhtap-demo-app",
		Skip:            false,
		Components: []ComponentSpec{
			{
				Name:                       "rhtap-demo-component",
				Language:                   "Java",
				GitSourceUrl:               fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), "hacbs-test-project"),
				GitSourceRevision:          "34da5a8f51fba6a8b7ec75a727d3c72ebb5e1274",
				GitSourceContext:           "",
				GitSourceDefaultBranchName: "main",
				HealthEndpoint:             "/",
				SkipDeploymentCheck:        false,
				AdvancedBuildSpec: &AdvancedBuildSpec{
					TestScenario: TestScenarioSpec{
						GitURL:      "https://github.com/redhat-appstudio/integration-examples.git",
						GitRevision: "843f455fe87a6d7f68c238f95a8f3eb304e65ac5",
						TestPath:    "pipelines/integration_resolver_pipeline_pass.yaml",
					},
				},
			},
		},
	},
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
			},
		},
	},
	{
		Name:            "DEVHAS-234: creates an application with python component from RHTAP samples",
		ApplicationName: "e2e-python-personal",
		Components: []ComponentSpec{
			{
				Name:                "component-python-flask",
				ContainerSource:     "",
				Language:            "Python",
				SkipDeploymentCheck: true,
				GitSourceUrl:        "https://github.com/devfile-samples/devfile-sample-python-basic.git",
				GitSourceRevision:   "",
				GitSourceContext:    "",
				HealthEndpoint:      "/",
			},
		},
	},
	{
		Name:            "DEVHAS-234: creates an application with dotnet component from RHTAP samples",
		ApplicationName: "e2e-dotnet",
		// https://redhat-appstudio.github.io/docs.appstudio.io/Documentation/main/getting-started/get-started/#choosing-a-bundled-sample
		// Seems like RHTAP dont support yet a dotnet sample. Disabling for not this tests.
		Skip: true,
		Components: []ComponentSpec{
			{
				Name:              "dotnet-component",
				ContainerSource:   "",
				Language:          "dotNet",
				GitSourceUrl:      "https://github.com/devfile-samples/devfile-sample-dotnet60-basic",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/",
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
			},
		},
	},
	{
		Name:            "DEVHAS-337: creates quarkus application from a private repository which contain a devfile",
		ApplicationName: "private-devfile",
		Components: []ComponentSpec{
			{
				Private:           true,
				Name:              "quarkus-devfile",
				ContainerSource:   "",
				Language:          "Java",
				GitSourceUrl:      "https://github.com/redhat-appstudio-qe/private-quarkus-devfile-sample.git",
				GitSourceRevision: "",
				GitSourceContext:  "",
				HealthEndpoint:    "/hello-resteasy",
			},
		},
	},
	{
		Name:            "DEVHAS-337: creates golang application from a private repository which contain a devfile referencing a private Dockerfile URI",
		ApplicationName: "private-devfile",
		// Due to bug in build team fetching private stuffs lets skip this test:
		// Bug: https://issues.redhat.com/browse/RHTAPBUGS-912
		Skip: true,
		Components: []ComponentSpec{
			{
				Private:        true,
				Name:           "go-devfile-private",
				Language:       "Go",
				GitSourceUrl:   "https://github.com/redhat-appstudio-qe/devfile-sample-go-basic-private-dockerfile-full-private.git",
				HealthEndpoint: "/",
			},
		},
	},
	{
		Name:            "Private nested application with 2 golang components",
		ApplicationName: "mc-golang-nested",
		Components: []ComponentSpec{
			{
				Name:                "mc-golang-nested",
				SkipDeploymentCheck: true,
				Private:             true,
				GitSourceUrl:        "https://github.com/redhat-appstudio-qe/devfile-sample-go-basic-private-devfile-nested.git",
			},
		},
	},
	{
		Name:            "Application with a golang component with dockerfile but not devfile (private)",
		ApplicationName: "mc-golang-nested",
		Components: []ComponentSpec{
			{
				Name:                "mc-golang-nodevfile",
				SkipDeploymentCheck: true,
				Private:             true,
				GitSourceUrl:        "https://github.com/redhat-appstudio-qe/devfile-sample-go-basic-dockerfile-only-private.git",
			},
		},
	},
	{
		Name:            "Private component withoud devfile/docker",
		ApplicationName: "mc-golang-without",
		Components: []ComponentSpec{
			{
				Name:                "mc-golang-without",
				SkipDeploymentCheck: true,
				Private:             true,
				GitSourceUrl:        "https://github.com/redhat-appstudio-qe/devfile-sample-go-basic-dockerfile-empty-private.git",
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
}
