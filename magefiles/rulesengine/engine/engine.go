package engine

import (
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine/repos"
)

var MageEngine = rulesengine.RuleEngine{
	"tests": {
		"e2e-repo":                repos.E2ETestRulesCatalog,
		"infra-deployments":       repos.InfraDeploymentsRulesCatalog,
		"release-service-catalog": repos.ReleaseServiceCatalogTestRulesCatalog,
	},
	"demo": {
		"local-workflow": repos.DemoCatalog,
	},

	"ci": {
		"e2e-repo":            repos.E2ECIChainCatalog,
		"release-service":     repos.ReleaseServiceCICatalog,
		"integration-service": repos.IntegrationServiceCICatalog,
		// TODO: to be implemented in a follow-up PR
		//"infra-deployments": repos.InfraDeploymentsCIChainCatalog,
	},
}
