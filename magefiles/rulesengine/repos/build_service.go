package repos

import (
	"os"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"k8s.io/klog"
)

var BuildServiceCICatalog = rulesengine.RuleCatalog{BuildServiceCIRule}

var BuildServiceCIRule = rulesengine.Rule{Name: "build-service repo CI Workflow Rule",
	Description: "Execute the full workflow for e2e-tests repo in CI",
	Condition: rulesengine.All{
		&BuildServiceRepoSetDefaultSettingsRule,
		rulesengine.Any{&InfraDeploymentsPRPairingRule, rulesengine.None{&InfraDeploymentsPRPairingRule}},
		&PreflightInstallGinkgoRule,
		&BootstrapClusterWithSprayProxyRuleChain,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteTestAction)},
}

var BuildServiceRepoSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for build-service repository jobs",
	Description: "Set SprayProxy settings to true for build-service jobs before bootstrap",
	Condition: rulesengine.Any{
		IsBuildServiceRepoPR,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		rctx.RequiresSprayProxyRegistering = true
		klog.Info("require sprayproxy registering is set to TRUE")

		rctx.LabelFilter = "build-service"
		klog.Info("setting 'build-service' test label")

		if rctx.DryRun {
			klog.Info("setting up env vars for deploying component image")
			return nil
		}
		rctx.ComponentEnvVarPrefix = "BUILD_SERVICE"
		// Option to execute the tests in Openshift CI
		if os.Getenv("KONFLUX_CI") != "true" {
			rctx.ComponentImageTag = "redhat-appstudio-build-service-image"
		}
		return SetEnvVarsForComponentImageDeployment(rctx)
	})},
}

var IsBuildServiceRepoPR = rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
	klog.Info("checking if repository is build-service")
	return rctx.RepoName == "build-service", nil
})
