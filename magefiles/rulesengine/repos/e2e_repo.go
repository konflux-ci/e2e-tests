package repos

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"k8s.io/klog"
)

var NonTestFilesRule = rulesengine.Rule{Name: "E2E Default PR Test Exectuion",
	Description: "Runs all suites when any non test files are modified in the e2e-repo PR",
	Condition: rulesengine.Any{
		rulesengine.ConditionFunc(CheckPkgFilesChanged),
		rulesengine.ConditionFunc(CheckMageFilesChanged),
		rulesengine.ConditionFunc(CheckCmdFilesChanged),
		rulesengine.ConditionFunc(CheckNoFilesChanged),
		rulesengine.ConditionFunc(CheckTektonFilesChanged),
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteDefaultTestAction)},
}

var TestFilesOnlyRule = rulesengine.Rule{Name: "E2E PR Test File Diff Execution",
	Description: "Runs specific tests when test files are the only changes in the e2e-repo PR",
	Condition: rulesengine.All{
		rulesengine.None{
			rulesengine.ConditionFunc(CheckPkgFilesChanged),
			rulesengine.ConditionFunc(CheckMageFilesChanged),
			rulesengine.ConditionFunc(CheckCmdFilesChanged),
			rulesengine.ConditionFunc(CheckNoFilesChanged),
			rulesengine.ConditionFunc(CheckTektonFilesChanged),
		},
		rulesengine.Any{
			&BuildTestFileChangeOnlyRule,
			&BuildTemplateScenarioFileChangeRule,
			&BuildNonTestFileChangeRule,
			&KonfluxDemoConfigsFileOnlyChangeRule,
			&KonfluxDemoTestFileChangedRule,
			&ReleaseTestHelperFilesChangeOnlyRule,
			&ReleaseTestTestFilesChangeRule,
			&IntegrationTestsConstFileChangeRule,
			&IntegrationTestsFileChangeRule,
			&EcTestFileChangeRule,
		},
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteTestAction)}}

func CheckTektonFilesChanged(rctx *rulesengine.RuleCtx) (bool, error) {

	return len(rctx.DiffFiles.FilterByDirString("integration-tests/")) != 0 || len(rctx.DiffFiles.FilterByDirString(".tekton/")) != 0, nil

}

var BuildTestFileChangeOnlyRule = rulesengine.Rule{Name: "E2E PR Build Test File Change Only Rule",
	Description: "Map build tests files when build test file are only changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirString("tests/build/build_templates_scenario.go")) == 0 &&
			len(rctx.DiffFiles.FilterByDirString("tests/build/const.go")) == 0 &&
			len(rctx.DiffFiles.FilterByDirString("tests/build/source_build.go")) == 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/build/*.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}
		return nil

	})}}

var BuildTemplateScenarioFileChangeRule = rulesengine.Rule{Name: "E2E PR Build Template Scenario File Changed Rule",
	Description: "Map build tests files when build template scenario file is changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirString("tests/build/build_templates_scenario.go")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		foundInDiff := false
		for _, file := range rctx.DiffFiles.FilterByDirString("tests/build/build_templates.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)
			foundInDiff = true

		}

		if !foundInDiff {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, "tests/build/build_templates.go")

		}

		return nil

	})}}

var BuildNonTestFileChangeRule = rulesengine.Rule{Name: "E2E PR Build Test Helper Files Change Rule",
	Description: "Map build tests files when const.go or source_build.go file is changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/build/const.go")) != 0 || len(rctx.DiffFiles.FilterByDirGlob("tests/build/source_build.go")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/build/*.go") {

			if strings.Contains(file.Name, "source_build.go") || strings.Contains(file.Name, "const.go") || strings.Contains(file.Name, "scenarios.go") {

				continue

			}
			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}

		return nil

	})}}

var ReleaseTestTestFilesChangeRule = rulesengine.Rule{Name: "E2E PR Release Test File Change Rule",
	Description: "Map release test files if they are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/release/*/*.go")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/release/*/*.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}

		return nil

	})}}

var ReleaseTestHelperFilesChangeOnlyRule = rulesengine.Rule{Name: "E2E PR Release Test Helper File CHange Rule",
	Description: "Map release tests files when only the release helper go files in root of release directory are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/release/*.go")) != 0 && len(rctx.DiffFiles.FilterByDirGlob("tests/release/*/*.go")) == 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		matched, err := filepath.Glob("tests/release/*/*.go")
		if err != nil {

			return err
		}
		for _, matched := range matched {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, matched)
		}

		return nil

	})}}

var KonfluxDemoTestFileChangedRule = rulesengine.Rule{Name: "E2E PR Konflux-Demo Test File Diff Map",
	Description: "Map demo tests files when konflux-demo test files are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/*-demo/*.go")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/*-demo/*-demo.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}
		return nil

	})}}

var KonfluxDemoConfigsFileOnlyChangeRule = rulesengine.Rule{Name: "E2E PR Konflux-Demo Config File Change Only Rule",
	Description: "Map demo tests files when konflux-demo config.go|type.go test files are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/*-demo/*.go")) == 0 && len(rctx.DiffFiles.FilterByDirGlob("tests/*-demo/*/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		matched, err := filepath.Glob("tests/*-demo/*-demo.go")
		if err != nil {

			return err

		}
		for _, matched := range matched {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, matched)
		}
		return nil

	})}}

var IntegrationTestsFileChangeRule = rulesengine.Rule{Name: "E2E PR Integration TestFile Change Rule",
	Description: "Map integration tests files when integration test files are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/integration-*/*.go")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/integration-*/*.go") {

			if strings.Contains(file.Name, "const.go") {

				continue

			}

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}

		return nil

	})}}

var IntegrationTestsConstFileChangeRule = rulesengine.Rule{Name: "E2E PR Integration TestFile Change Rule",
	Description: "Map all integration tests files when integration const files are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/integration-*/const.go")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		matched, err := filepath.Glob("tests/integration-*/*.go")
		if err != nil {

			return err

		}
		for _, matched := range matched {

			if strings.Contains(matched, "const.go") {

				continue

			}

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, matched)
		}

		return nil

	})}}

var EcTestFileChangeRule = rulesengine.Rule{Name: "E2E PR EC Test File Change Rule",
	Description: "Map EC tests files when EC test files are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
		return len(rctx.DiffFiles.FilterByDirGlob("tests/enterprise-*/*.go")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/enterprise-*/*.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}

		return nil

	})}}

var E2ERepoPrepareBranchRule = rulesengine.Rule{Name: "E2E prepare branch for CI",
	Description: "Checkout the e2e-tests repo using commit SHA in CI",
	Condition: rulesengine.All{
		rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
			return rctx.RepoName == "e2e-tests", nil
		}),
		rulesengine.None{
			rulesengine.ConditionFunc(IsPeriodicJob),
			rulesengine.ConditionFunc(IsRehearseJob),
		},
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		if rctx.DryRun {

			for _, arg := range [][]string{
				{"remote", "add", rctx.PrRemoteName, fmt.Sprintf("https://github.com/%s/e2e-tests.git", rctx.PrRemoteName)},
				{"fetch", rctx.PrRemoteName},
				{"checkout", rctx.PrCommitSha},
				{"pull", "--rebase", "upstream", "main"},
			} {
				klog.Infof("git %s", arg)
			}

			return nil
		}

		return GitCheckoutRemoteBranch(rctx.PrRemoteName, rctx.PrCommitSha)
	})},
}

var E2ERepoSetRequiredSettingsForPRRule = rulesengine.Rule{Name: "Set Required Settings for E2E Repo PR Paired Job",
	Description: "Set up required infra-deployments variables for e2e-tests repo PR paired job before bootstrap ",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
		if rctx.DryRun {

			if true {
				klog.Infof("Found infra deployments branch %s for author %s", rctx.PrBranchName, rctx.PrRemoteName)
				return true, nil
			} else {
				return false, fmt.Errorf("cannot determine infra-deployments Github branches for author %s: none. will stick with the redhat-appstudio/infra-deployments main branch for running tests", rctx.PrRemoteName)
			}
		}

		return IsPRPairingRequired("infra-deployments", rctx.PrRemoteName, rctx.PrBranchName), nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		if rctx.DryRun {

			klog.Infof("Set INFRA_DEPLOYMENTS_ORG: %s", rctx.PrRemoteName)
			klog.Infof("Set INFRA_DEPLOYMENTS_BRANCH: %s", rctx.PrBranchName)
			return nil
		}

		klog.Infof("pairing with infra-deployments org %q and branch %q", rctx.PrRemoteName, rctx.PrBranchName)
		os.Setenv("INFRA_DEPLOYMENTS_ORG", rctx.PrRemoteName)
		os.Setenv("INFRA_DEPLOYMENTS_BRANCH", rctx.PrBranchName)
		return nil
	})},
}

var E2ERepoCIRuleChain = rulesengine.Rule{Name: "E2E Repo CI Workflow Rule Chain",
	Description: "Execute the full workflow for e2e-tests repo in CI",
	Condition: rulesengine.All{
		&E2ERepoSetDefaultSettingsRule,
		rulesengine.Any{&E2ERepoSetRequiredSettingsForPRRule, rulesengine.None{&E2ERepoSetRequiredSettingsForPRRule}},
		&PreflightInstallGinkgoRule,
		&BootstrapClusterRuleChain,
		rulesengine.Any{&NonTestFilesRule, &TestFilesOnlyRule}},
}

var E2ERepoSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for E2E Repo Jobs",
	Description: "Set multiplatform and SprayProxy settings to true for e2e-tests jobs before bootstrap",
	Condition: rulesengine.Any{
		IsE2ETestsRepoPR,
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		var err error

		rctx.RequiresMultiPlatformTests = true
		rctx.RequiresSprayProxyRegistering = true
		klog.Info("multi-platform tests and require sprayproxy registering are set to TRUE")

		rctx.DiffFiles, err = utils.GetChangedFiles(rctx.RepoName)
		return err
	})},
}

var E2ECIChainCatalog = rulesengine.RuleCatalog{E2ERepoCIRuleChain}

var E2ETestRulesCatalog = rulesengine.RuleCatalog{NonTestFilesRule, TestFilesOnlyRule}

var IsE2ETestsRepoPR = rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
	klog.Info("checking if repository is e2e-tests")
	return rctx.RepoName == "e2e-tests", nil
})

func CheckNoFilesChanged(rctx *rulesengine.RuleCtx) (bool, error) {

	return len(rctx.DiffFiles) == 0, nil
}

func CheckPkgFilesChanged(rctx *rulesengine.RuleCtx) (bool, error) {

	return len(rctx.DiffFiles.FilterByDirString("pkg/")) != 0, nil

}

func CheckMageFilesChanged(rctx *rulesengine.RuleCtx) (bool, error) {

	return len(rctx.DiffFiles.FilterByDirString("magefiles/")) != 0, nil

}

func CheckCmdFilesChanged(rctx *rulesengine.RuleCtx) (bool, error) {

	return len(rctx.DiffFiles.FilterByDirString("cmd/")) != 0, nil

}

func ExecuteDefaultTestAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "!upgrade-create && !upgrade-verify && !upgrade-cleanup && !release-pipelines"
	return ExecuteTestAction(rctx)

}

func dedupeAppendFiles(files []string, file string) []string {

	for _, f := range files {

		if f == file {
			return files
		}
	}

	return append(files, file)
}
