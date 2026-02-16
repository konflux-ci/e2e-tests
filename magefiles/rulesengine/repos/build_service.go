package repos

import (
	"os"
	"strings"

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
		rulesengine.Any{rulesengine.None{&BootstrapClusterWithSprayProxyRuleChain}, &BootstrapClusterWithSprayProxyRuleChain},
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

		rctx.LabelFilter = BuildLabelFilter("build-service")
		klog.Infof("setting test label filter: '%s'", rctx.LabelFilter)

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

// BuildLabelFilter constructs a Ginkgo label filter by combining the base label
// with any extra labels from the E2E_EXTRA_LABEL_FILTER environment variable.
//
// E2E_EXTRA_LABEL_FILTER accepts any valid Ginkgo label expression that will be
// AND-ed with the base label. Examples:
//
//	E2E_EXTRA_LABEL_FILTER=github                        -> "build-service && github"
//	E2E_EXTRA_LABEL_FILTER=gitlab                        -> "build-service && gitlab"
//	E2E_EXTRA_LABEL_FILTER=multi-component               -> "build-service && multi-component"
//	E2E_EXTRA_LABEL_FILTER=github && pac-build           -> "build-service && github && pac-build"
//	E2E_EXTRA_LABEL_FILTER=github || gitlab              -> "build-service && (github || gitlab)"
//	E2E_EXTRA_LABEL_FILTER=""                            -> "build-service" (no extra filter)
//
// This allows CI jobs to target specific test subsets without changing any test code.
func BuildLabelFilter(baseLabel string) string {
	extra := strings.TrimSpace(os.Getenv("E2E_EXTRA_LABEL_FILTER"))
	if extra == "" {
		return baseLabel
	}

	// If the extra filter contains logical operators, wrap it in parentheses
	// to preserve correct precedence when combined with &&
	if strings.Contains(extra, "||") {
		return baseLabel + " && (" + extra + ")"
	}
	return baseLabel + " && " + extra
}
