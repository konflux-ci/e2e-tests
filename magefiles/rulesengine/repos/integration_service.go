package repos

import (
	"os"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"k8s.io/klog"
)

var IntegrationServiceCICatalog = rulesengine.RuleCatalog{IntegrationServiceCIRule}

var IntegrationServiceCIRule = rulesengine.Rule{Name: "Integration-service repo CI Workflow Rule",
	Description: "Execute the full workflow for e2e-tests repo in CI",
	Condition: rulesengine.All{
		&IntegrationServiceRepoSetDefaultSettingsRule,
		rulesengine.Any{&InfraDeploymentsPRPairingRule, rulesengine.None{&InfraDeploymentsPRPairingRule}},
		&PreflightInstallGinkgoRule,
		&BootstrapClusterWithSprayProxyRuleChain,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteTestAction)},
}

var IntegrationServiceRepoSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for integration-service repository jobs",
	Description: "Set SprayProxy settings to true for integration-service jobs before bootstrap",
	Condition: rulesengine.Any{
		IsIntegrationServiceRepoPR,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		rctx.RequiresSprayProxyRegistering = true
		klog.Info("require sprayproxy registering is set to TRUE")

		rctx.LabelFilter = "integration-service"
		klog.Info("setting 'integration-service' test label")

		if rctx.DryRun {
			klog.Info("setting up env vars for deploying component image")
			return nil
		}
		rctx.ComponentEnvVarPrefix = "INTEGRATION_SERVICE"
		// TODO keep only "KONFLUX_CI" option once we migrate off openshift-ci
		if os.Getenv("KONFLUX_CI") != "true" {
			rctx.ComponentImageTag = "redhat-appstudio-integration-service-image"
		}
		return SetEnvVarsForComponentImageDeployment(rctx)
	})},
}

var IsIntegrationServiceRepoPR = rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
	klog.Info("checking if repository is integration-service")
	return rctx.RepoName == "integration-service", nil
})
