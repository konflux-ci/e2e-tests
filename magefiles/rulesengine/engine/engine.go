package engine

import (
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine/repos"
)

var MageEngine = rulesengine.RuleEngine{
	"tests": {
		"e2e-repo":                repos.E2ETestRulesCatalog,
		"infra-deployments":       repos.InfraDeploymentsRulesCatalog,
	},
	"demo": {
		"local-workflow": repos.DemoCatalog,
	},

	"ci": {
		"e2e-repo":                repos.E2ECIChainCatalog,
		"release-service":         repos.ReleaseServiceCICatalog,
		"release-service-catalog": repos.ReleaseServiceCatalogCICatalog,
		"integration-service":     repos.IntegrationServiceCICatalog,
		"image-controller":        repos.ImageControllerCICatalog,
		"build-service":           repos.BuildServiceCICatalog,
		// TODO: to be implemented in a follow-up PR
		//"infra-deployments": repos.InfraDeploymentsCIChainCatalog,
	},
}
