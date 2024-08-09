package repos

import (
	"os"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"k8s.io/klog"
)

// Example rule for infra-deployments using anonymous functions embedded in the rule
var InfraDeploymentsTestRulesCatalog = rulesengine.RuleCatalog{InfraDeploymentsDefaultTestSelection}
var InfraDeploymentsCIChainCatalog = rulesengine.RuleCatalog{InfraDeploymentsCIRuleChainCatalog}

var InfraDeploymentsDefaultTestSelection = rulesengine.Rule{Name: "Infra Deployments Default Test Execution",
	Description: "Run the default test suites which include the demo and components suites.",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return rctx.RepoName == "infra-deployments"
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		rctx.LabelFilter = "e2e-demo,rhtap-demo,spi-suite,remote-secret,integration-service,ec,build-templates,multi-platform"
		return ExecuteTestAction(rctx)
	})}}

var InfraDeploymentsSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for Infra-Deployment Repo Jobs",
	Description: "Set multiplatform and SprayProxy settings to true for infra-deployments jobs before bootstrap",
	Condition: rulesengine.Any{rulesengine.ConditionFunc(IsPeriodicJob), rulesengine.ConditionFunc(IsRehearseJob),
		rulesengine.None{rulesengine.ConditionFunc(IsPeriodicJob), rulesengine.ConditionFunc(IsRehearseJob)}},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.RequiresMultiPlatformTests = true
		rctx.RequiresSprayProxyRegistering = true
		klog.Info("multi-pltform tests and require sprayproxy registering are set to TRUE")
		return nil
	})},
}

var InfraDeploymentsSetRequiredSettingsForPRRule = rulesengine.Rule{Name: "Set Required Settings for Infra-Deployment Repo PR Job",
	Description: "Set up required infra-deployments variables for infra-deployments repo PR job before bootstrap ",
	Condition:   rulesengine.None{rulesengine.ConditionFunc(IsPeriodicJob), rulesengine.ConditionFunc(IsRehearseJob)},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		if rctx.DryRun {

			klog.Infof("Set INFRA_DEPLOYMENTS_ORG: %s", rctx.PrRemoteName)
			klog.Infof("Set INFRA_DEPLOYMENTS_BRANCH: %s", rctx.PrBranchName)
			return nil
		}

		os.Setenv("INFRA_DEPLOYMENTS_ORG", rctx.PrRemoteName)
		os.Setenv("INFRA_DEPLOYMENTS_BRANCH", rctx.PrBranchName)
		return nil
	})},
}

var InfraDeploymentsSettingsRuleChain = rulesengine.Rule{Name: "Set Require Settings E2E Repo Rule Chain",
	Description: "Rule Chain that setups up the required settings for non load-tests job types for e2e-test repo before bootstrap",
	Condition: rulesengine.All{rulesengine.None{rulesengine.ConditionFunc(IsLoadTestJob)}, &InfraDeploymentsSetDefaultSettingsRule,
		rulesengine.Any{&InfraDeploymentsSetRequiredSettingsForPRRule}},
}

var InfraDeploymentsCIRuleChainCatalog = rulesengine.Rule{Name: "Infra-Deployments Repo CI Workflow Rule Chain",
	Description: "Execute the full workflow for e2e-tests repo in CI",
	Condition: rulesengine.All{&PrepareBranchRule, &PreflightInstallGinkgoRule,
		&InfraDeploymentsSettingsRuleChain, &BootstrapClusterRuleChain, &InfraDeploymentsDefaultTestSelection},
}
