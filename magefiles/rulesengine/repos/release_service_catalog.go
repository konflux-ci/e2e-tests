package repos

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"k8s.io/klog"
)

var ReleaseServiceCatalogCICatalog = rulesengine.RuleCatalog{ReleaseServiceCatalogCIPairedRule, ReleaseServiceCatalogCIRule}

var ReleaseServiceCatalogCIPairedRule = rulesengine.Rule{Name: "Release-service-catalog repo CI Workflow Paired Rule",
	Description: "Execute the Paired workflow for release-service-catalog repo in CI",
	Condition: rulesengine.All{
		rulesengine.ConditionFunc(isPaired),
		rulesengine.None{
			rulesengine.ConditionFunc(isRehearse),
		},
		&ReleaseServiceCatalogRepoSetDefaultSettingsRule,
		rulesengine.Any{&InfraDeploymentsPRPairingRule, rulesengine.None{&InfraDeploymentsPRPairingRule}},
		&PreflightInstallGinkgoRule,
		&InstallKonfluxRule,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteReleaseCatalogPairedAction)},
}

var ReleaseServiceCatalogCIRule = rulesengine.Rule{Name: "Release-service-catalog repo CI Workflow Rule",
	Description: "Execute the full workflow for release-service-catalog repo in CI",
	Condition: rulesengine.All{
		rulesengine.Any{
			rulesengine.None{rulesengine.ConditionFunc(isPaired)},
			rulesengine.ConditionFunc(isRehearse),
		},
		&ReleaseServiceCatalogRepoSetDefaultSettingsRule,
		rulesengine.Any{&InfraDeploymentsPRPairingRule, rulesengine.None{&InfraDeploymentsPRPairingRule}},
		&PreflightInstallGinkgoRule,
		&InstallKonfluxRule,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteReleaseCatalogAction)},
}

var ReleaseServiceCatalogRepoSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for release-service-catalog repository jobs",
	Description: "relese-service-catalog jobs default rule",
	Condition: rulesengine.Any{
		IsReleaseServiceCatalogRepoPR,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		rctx.LabelFilter = "release-service-catalog"
		klog.Info("setting 'release-service-catalog' test label")

		if rctx.DryRun {
			klog.Info("setting up env vars for deploying component image")
			return nil
		}
		rctx.ComponentEnvVarPrefix = "RELEASE_SERVICE"

		//This is env variable is specified for release service catalog
		os.Setenv(fmt.Sprintf("%s_CATALOG_URL", rctx.ComponentEnvVarPrefix), fmt.Sprintf("https://github.com/%s/%s", rctx.PrRemoteName, rctx.RepoName))
		if rctx.PrRemoteName == "konflux-ci" {
			os.Setenv(fmt.Sprintf("%s_CATALOG_REVISION", rctx.ComponentEnvVarPrefix), rctx.PrBranchName)
		} else {
			os.Setenv(fmt.Sprintf("%s_CATALOG_REVISION", rctx.ComponentEnvVarPrefix), rctx.PrCommitSha)
		}
		os.Setenv("DEPLOY_ONLY", "application-api dev-sso enterprise-contract has pipeline-service integration internal-services release")

		if rctx.IsPaired && !strings.Contains(rctx.JobName, "rehearse") {
			os.Setenv(fmt.Sprintf("%s_IMAGE_REPO", rctx.ComponentEnvVarPrefix),
				"quay.io/redhat-user-workloads/rhtap-release-2-tenant/release-service/release-service")
			pairedSha := GetPairedCommitSha("release-service", rctx)
			if pairedSha != "" {
				os.Setenv(fmt.Sprintf("%s_IMAGE_TAG", rctx.ComponentEnvVarPrefix), fmt.Sprintf("on-pr-%s", pairedSha))
			}
			os.Setenv(fmt.Sprintf("%s_PR_OWNER", rctx.ComponentEnvVarPrefix), rctx.PrRemoteName)
			os.Setenv(fmt.Sprintf("%s_PR_SHA", rctx.ComponentEnvVarPrefix), pairedSha)
		}
		return nil
	})},
}

var IsReleaseServiceCatalogRepoPR = rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
	klog.Info("checking if repository is release-service-catalog")
	return rctx.RepoName == "release-service-catalog", nil
})

var isRehearse = func(rctx *rulesengine.RuleCtx) (bool, error) {

	return strings.Contains(rctx.JobName, "rehearse"), nil
}

var isPaired = func(rctx *rulesengine.RuleCtx) (bool, error) {
	rctx.IsPaired = IsPRPairingRequired("release-service", rctx.PrRemoteName, rctx.PrBranchName)
	return rctx.IsPaired, nil
}

func ExecuteReleaseCatalogPairedAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "release-pipelines && !fbc-tests && !multiarch-advisories && !rh-advisories && !release-to-github && !rh-push-to-redhat-io && !rhtap-service-push"
	return ExecuteTestAction(rctx)
}

func ExecuteReleaseCatalogAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "release-pipelines"
	rctx.Timeout = 2*time.Hour + 30*time.Minute
	return ExecuteTestAction(rctx)
}
