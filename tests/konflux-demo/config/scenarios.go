package config

import (
	"fmt"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

var ApplicationSpecs = []ApplicationSpec{
	{
		Name:            "Maven project - Default build",
		ApplicationName: "konflux-demo-app",
		Skip:            false,
		ComponentSpec: ComponentSpec{
			Name:                       "konflux-demo-component",
			Language:                   "Java",
			GitSourceUrl:               fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), "hacbs-test-project-konflux-demo"),
			GitSourceRevision:          "79402df023e646c5ad108abc879ad1b28799cbc4",
			GitSourceContext:           "",
			GitSourceDefaultBranchName: "main",
			DockerFilePath:             "Dockerfile",
			BuildPipelineType:          constants.DockerBuildOciTA,
			IntegrationTestScenario: IntegrationTestScenarioSpec{
				GitURL:      "https://github.com/konflux-ci/integration-examples.git",
				GitRevision: "843f455fe87a6d7f68c238f95a8f3eb304e65ac5",
				TestPath:    "pipelines/integration_resolver_pipeline_pass.yaml",
			},
		},
	},
}
var UpstreamAppSpecs = []ApplicationSpec{
	{
		Name:            "Test local instance of konflux-ci - docker-build-oci-ta pipeline",
		ApplicationName: "konflux-ci-upstream-docker-build-oci-ta",
		Skip:            false,
		ComponentSpec: ComponentSpec{
			Name:                       "konflux-ci-upstream",
			GitSourceUrl:               fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), "testrepo"),
			GitSourceRevision:          "47517b7ad6a09ada952f3de7eb8da729ffbf3d6d",
			GitSourceDefaultBranchName: "main",
			DockerFilePath:             "Dockerfile",
			BuildPipelineType:          constants.DockerBuildOciTA,
			IntegrationTestScenario: IntegrationTestScenarioSpec{
				GitURL:      fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), "testrepo"),
				GitRevision: "47517b7ad6a09ada952f3de7eb8da729ffbf3d6d",
				TestPath:    "integration-tests/testrepo-integration.yaml",
			},
		},
	},
}
