package repos

import (
	"fmt"
	"os"
	"os/exec"
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
		rulesengine.Any{rulesengine.None{&InstallKonfluxRule}, &InstallKonfluxRule},
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
		rulesengine.Any{rulesengine.None{&InstallKonfluxRule}, &InstallKonfluxRule},
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteReleaseCatalogAction)},
}

var ReleaseServiceCatalogRepoSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for release-service-catalog repository jobs",
	Description: "relese-service-catalog jobs default rule",
	Condition: rulesengine.Any{
		IsReleaseServiceCatalogRepoPR,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		testcases, err := selectReleasePipelinesTestCases(rctx.PrNum)
		if err != nil {
			rctx.LabelFilter = "release-pipelines"
			klog.Errorf("an error occurred in selectReleasePipelinesTestCases: %s", err)
		} else {
			rctx.LabelFilter = strings.ReplaceAll(testcases, " ", "||")
		}
		klog.Info("setting test label for release-pipelines: ", rctx.LabelFilter)

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

		// Failed at https://github.com/redhat-appstudio/infra-deployments/blob/2228e063a7fd8af4a95b24bb13ce7360cdc229f0/hack/preview.sh#L293C16-L293C38
		//os.Setenv("DEPLOY_ONLY", "application-api dev-sso enterprise-contract has pipeline-service integration internal-services release")

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
	rctx.LabelFilter += " && !fbc-release && !multiarch-advisories && !rh-advisories && !release-to-github && !rh-push-to-registry-redhat-io && !rhtap-service-push"
	return ExecuteTestAction(rctx)
}

func ExecuteReleaseCatalogAction(rctx *rulesengine.RuleCtx) error {
	rctx.Timeout = 2*time.Hour + 30*time.Minute
	return ExecuteTestAction(rctx)
}

func selectReleasePipelinesTestCases(prNum int) (string, error) {
	command := fmt.Sprintf("%s %s %d", "magefiles/rulesengine/scripts/find_release_pipelines_from_pr.sh", "konflux-ci/release-service-catalog", prNum)
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("failed in selectReleasePipelinesTestCases function for PR: %d", prNum)
		klog.Infof("the output from find_release_pipelines_from_pr: %s", string(output))
		return "", err
	}
	return string(output), nil
}
