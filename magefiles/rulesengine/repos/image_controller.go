package repos

import (
	"os"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"k8s.io/klog"
)

var ImageControllerCICatalog = rulesengine.RuleCatalog{ImageControllerCIRule}

var ImageControllerCIRule = rulesengine.Rule{Name: "image-controller repo CI Workflow Rule",
	Description: "Execute the full workflow for e2e-tests repo in CI",
	Condition: rulesengine.All{
		&ImageControllerRepoSetDefaultSettingsRule,
		rulesengine.Any{&InfraDeploymentsPRPairingRule, rulesengine.None{&InfraDeploymentsPRPairingRule}},
		&PreflightInstallGinkgoRule,
		rulesengine.Any{rulesengine.None{&BootstrapClusterWithSprayProxyRuleChain}, &BootstrapClusterWithSprayProxyRuleChain},
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteTestAction)},
}

var ImageControllerRepoSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for image-controller repository jobs",
	Description: "Set SprayProxy settings to true for image-controller jobs before bootstrap",
	Condition: rulesengine.Any{
		IsImageControllerRepoPR,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		rctx.RequiresSprayProxyRegistering = true
		klog.Info("require sprayproxy registering is set to TRUE")

		rctx.LabelFilter = "image-controller"
		klog.Info("setting 'image-controller' test label")

		if rctx.DryRun {
			klog.Info("setting up env vars for deploying component image")
			return nil
		}
		rctx.ComponentEnvVarPrefix = "IMAGE_CONTROLLER"
		// TODO keep only "KONFLUX_CI" option once we migrate off openshift-ci
		if os.Getenv("KONFLUX_CI") != "true" {
			rctx.ComponentImageTag = "redhat-appstudio-image-controller-image"
		}
		return SetEnvVarsForComponentImageDeployment(rctx)
	})},
}

var IsImageControllerRepoPR = rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
	klog.Info("checking if repository is image-controller")
	return rctx.RepoName == "image-controller", nil
})
