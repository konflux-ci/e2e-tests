package engine

import (
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine/repos"
)

var MageEngine = rulesengine.RuleEngine{
	"tests": {
		"e2e-repo":          repos.E2ETestRulesCatalog,
		"infra-deployments": repos.InfraDeploymentsTestRulesCatalog,
		"release-catalog":   repos.ReleaseServiceCatalogTestRulesCatalog,
	},
	"demo": {
		"local-workflow": repos.DemoCatalog,
	},

	"ci": {
		"e2e-repo":          testselection.E2ECIChainCatalog,
		"infra-deployments": testselection.InfraDeploymentsCIChainCatalog,
	},
}
