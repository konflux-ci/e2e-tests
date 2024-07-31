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
			GitSourceUrl:               fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), "hacbs-test-project"),
			GitSourceRevision:          "4df701406d34012034dd490fd38d779717582df7",
			GitSourceContext:           "",
			GitSourceDefaultBranchName: "main",
			DockerFilePath:             "Dockerfile",

			IntegrationTestScenario: IntegrationTestScenarioSpec{
				GitURL:      "https://github.com/konflux-ci/integration-examples.git",
				GitRevision: "843f455fe87a6d7f68c238f95a8f3eb304e65ac5",
				TestPath:    "pipelines/integration_resolver_pipeline_pass.yaml",
			},
		},
	},
}
