package config

import (
	"fmt"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
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
				DockerFilePath:             "Dockerfile",
				HealthEndpoint:             "/",
				SkipDeploymentCheck:        true,
				AdvancedBuildSpec: &AdvancedBuildSpec{
					TestScenario: TestScenarioSpec{
						GitURL:      "https://github.com/konflux-ci/integration-examples.git",
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
				DockerFilePath:    constants.DockerFilePath,
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
				DockerFilePath:      constants.DockerFilePath,
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
				DockerFilePath:    constants.DockerFilePath,
				HealthEndpoint:    "/",
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
				DockerFilePath:    constants.DockerFilePath,
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
				DockerFilePath:    "Dockerfile",
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
				DockerFilePath:    "src/main/docker/Dockerfile.jvm.staged",
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
				DockerFilePath:    constants.DockerFilePath,
				HealthEndpoint:    "/",
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
				DockerFilePath:    "src/main/docker/Dockerfile.jvm.staged",
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
				DockerFilePath: "https://raw.githubusercontent.com/redhat-appstudio-qe/devfile-sample-go-basic-private/main/docker/Dockerfile",
				HealthEndpoint: "/",
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
				DockerFilePath:      constants.DockerFilePath,
			},
		},
	},
	{
		Name:            "Stage Test - Simple Stage Test With SpringBoot Basic",
		ApplicationName: "rhtap-stage-demo-app",
		Skip:            false,
		Stage:           true,
		Components: []ComponentSpec{
			{
				Name:                "rhtap-stage-demo-component",
				ContainerSource:     "",
				Language:            "Java",
				GitSourceUrl:        "https://github.com/devfile-samples/devfile-sample-java-springboot-basic",
				GitSourceRevision:   "",
				GitSourceContext:    "",
				DockerFilePath:      constants.DockerFilePath,
				HealthEndpoint:      "/",
				SkipDeploymentCheck: true,
			},
		},
	},
}

func GetScenarios(isStage bool) []TestSpec {
	var StageScenarios []TestSpec
	var NormalScenarios []TestSpec

	for _, Scenario := range TestScenarios {
		if Scenario.Stage {
			StageScenarios = append(StageScenarios, Scenario)
		} else {
			NormalScenarios = append(NormalScenarios, Scenario)
		}
	}

	if isStage {
		return StageScenarios
	} else {
		return NormalScenarios
	}
}
