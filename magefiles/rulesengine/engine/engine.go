package engine

import (
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine/testselection"
)

var MageEngine = rulesengine.RuleEngine{
	"tests": {
		"e2e-repo":          testselection.E2ETestRulesCatalog,
		"infra-deployments": testselection.InfraDeploymentsTestRulesCatalog,
	},
	"demo": {
		"local-workflow": testselection.DemoCatalog,
	},
}
