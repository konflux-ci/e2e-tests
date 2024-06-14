package testselection

import "github.com/konflux-ci/e2e-tests/magefiles/rulesengine"

// Example rule for infra-deployments using anonymous functions embedded in the rule
var InfraDeploymentsTestRulesCatalog = rulesengine.RuleCatalog{
	rulesengine.Rule{Name: "Infra Deployments Default Test Execution",
		Description: "Run the default test suites which include the demo and components suites.",
		Condtion:    rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {
			if rctx.RepoName == "infra-deployments" {
				return true
			}
			return false
		}),
		Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
			rctx.LabelFilter = "e2e-demo,rhtap-demo,spi-suite,remote-secret,integration-service,ec,build-templates,multi-platform"
			return ExecuteTestAction(rctx)
		})}},

}