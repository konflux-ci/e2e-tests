package repos

import (
	"fmt"
	"os"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"k8s.io/klog"
)

var ReleaseServiceCICatalog = rulesengine.RuleCatalog{ReleaseServiceCIRule}

var ReleaseServiceCIRule = rulesengine.Rule{Name: "Release-service repo CI Workflow Rule",
	Description: "Execute the full workflow for release-service repo in CI",
	Condition: rulesengine.All{
		&ReleaseServiceRepoSetDefaultSettingsRule,
		rulesengine.Any{&InfraDeploymentsPRPairingRule, rulesengine.None{&InfraDeploymentsPRPairingRule}},
		&PreflightInstallGinkgoRule,
		&InstallKonfluxRule,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteTestAction)},
}

var ReleaseServiceRepoSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for release-service repository jobs",
	Description: "relese-service jobs default rule",
	Condition: rulesengine.Any{
		IsReleaseServiceRepoPR,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		rctx.LabelFilter = "release-service"
		klog.Info("setting 'release-service' test label")

		if rctx.DryRun {
			klog.Info("setting up env vars for deploying component image")
			return nil
		}
		rctx.ComponentEnvVarPrefix = "RELEASE_SERVICE"
		// TODO keep only "KONFLUX_CI" option once we migrate off openshift-ci
		if os.Getenv("KONFLUX_CI") == "true" {
			rctx.ComponentImageTag = fmt.Sprintf("on-pr-%s", rctx.PrCommitSha)
		} else {
			rctx.ComponentImageTag = "redhat-appstudio-release-service-image"
		}
		//This is env variable is specified for release service
		os.Setenv(fmt.Sprintf("%s_CATALOG_REVISION", rctx.ComponentEnvVarPrefix), "development")
		return SetEnvVarsForComponentImageDeployment(rctx)
	})},
}

var IsReleaseServiceRepoPR = rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
	klog.Info("checking if repository is release-service")
	return rctx.RepoName == "release-service", nil
})
