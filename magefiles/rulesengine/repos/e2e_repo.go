package repos

import (
	"path/filepath"
	"strings"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
)

var NonTestFilesRule = rulesengine.Rule{Name: "E2E Default PR Test Exectuion",
	Description: "Runs all suites when any non test files are modified in the e2e-repo PR",
	Condition: rulesengine.Any{rulesengine.ConditionFunc(CheckPkgFilesChanged),
		rulesengine.ConditionFunc(CheckMageFilesChanged),
		rulesengine.ConditionFunc(CheckCmdFilesChanged),
		rulesengine.ConditionFunc(CheckNoFilesChanged)},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteDefaultTestAction)}}

var TestFilesOnlyRule = rulesengine.Rule{Name: "E2E PR Test File Diff Execution",
	Description: "Runs specific tests when test files are the only changes in the e2e-repo PR",
	Condition: rulesengine.All{rulesengine.None{rulesengine.ConditionFunc(CheckPkgFilesChanged),
		rulesengine.ConditionFunc(CheckMageFilesChanged),
		rulesengine.ConditionFunc(CheckCmdFilesChanged),
		rulesengine.ConditionFunc(CheckNoFilesChanged)}, rulesengine.Any{
		&BuildTestFileChangeOnlyRule,
		&BuildTemplateScenarioFileChangeRule,
		&BuildNonTestFileChangeRule,
		&KonfluxDemoConfigsFileOnlyChangeRule,
		&KonfluxDemoTestFileChangedRule,
		&ReleaseTestHelperFilesChangeOnlyRule,
		&ReleaseTestTestFilesChangeRule,
		&ReleaseConstFileChangeBuildTestRule,
		&IntegrationTestsConstFileChangeRule,
		&IntegrationTestsFileChangeRule,
		&EcTestFileChangeRule}},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteTestAction)}}

var BuildTestFileChangeOnlyRule = rulesengine.Rule{Name: "E2E PR Build Test File Change Only Rule",
	Description: "Map build tests files when build test file are only changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirString("tests/build/build_templates_scenario.go")) == 0 &&
			len(rctx.DiffFiles.FilterByDirString("tests/build/const.go")) == 0 &&
			len(rctx.DiffFiles.FilterByDirString("tests/build/source_build.go")) == 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/build/*.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}
		return nil

	})}}

var BuildTemplateScenarioFileChangeRule = rulesengine.Rule{Name: "E2E PR Build Template Scenario File Changed Rule",
	Description: "Map build tests files when build template scenario file is changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirString("tests/build/build_templates_scenario.go")) != 0
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
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/build/const.go")) != 0 || len(rctx.DiffFiles.FilterByDirGlob("tests/build/source_build.go")) != 0
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
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/release/*/*.go")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/release/*/*.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}

		return nil

	})}}

var ReleaseTestHelperFilesChangeOnlyRule = rulesengine.Rule{Name: "E2E PR Release Test Helper File CHange Rule",
	Description: "Map release tests files when only the release helper go files in root of release directory are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/release/*.go")) != 0 && len(rctx.DiffFiles.FilterByDirGlob("tests/release/*/*.go")) == 0
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

var ReleaseConstFileChangeBuildTestRule = rulesengine.Rule{Name: "E2E PR Release Test Const File Dependency Change Rule",
	Description: "Map build tests files when release const.go file is changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirString("tests/release/const.go")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, "tests/build/build.go")

		return nil

	})}}

var KonfluxDemoTestFileChangedRule = rulesengine.Rule{Name: "E2E PR Konflux-Demo Test File Diff Map",
	Description: "Map demo tests files when konflux-demo test files are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/*-demo/*.go")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/*-demo/*-demo.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}
		return nil

	})}}

var KonfluxDemoConfigsFileOnlyChangeRule = rulesengine.Rule{Name: "E2E PR Konflux-Demo Config File Change Only Rule",
	Description: "Map demo tests files when konflux-demo config.go|type.go test files are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/*-demo/*.go")) == 0 && len(rctx.DiffFiles.FilterByDirGlob("tests/*-demo/*/*")) != 0
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
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/integration-*/*.go")) != 0
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
	Description: "Map integration tests files when integration const files are changed in the e2e-repo PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/integration-*/const.go")) != 0 && len(rctx.DiffFiles.FilterByDirGlob("tests/integration-*/*.go")) == 0
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
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("tests/enterprise-*/*.go")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		for _, file := range rctx.DiffFiles.FilterByDirGlob("tests/enterprise-*/*.go") {

			rctx.FocusFiles = dedupeAppendFiles(rctx.FocusFiles, file.Name)

		}

		return nil

	})}}

	var E2ETestRulesCatalog = rulesengine.RuleCatalog{NonTestFilesRule, TestFilesOnlyRule}

var E2ERepoPrepareBranchRule = rulesengine.Rule{Name: "E2E prepare branch for CI",
		Description: "Checkout the e2e-tests repo using commit SHA in CI",
		Condition: rulesengine.All{rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {
	
			return rctx.RepoName == "e2e-tests"
		}), rulesengine.None{rulesengine.ConditionFunc(IsPeriodicJob),
			rulesengine.ConditionFunc(IsRehearseJob)}},
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
	
var E2ERepoSetDefaultSettingsRule = rulesengine.Rule{Name: "General Required Settings for E2E Repo Jobs",
		Description: "Set multiplatform and SprayProxy settings to true for e2e-tests jobs before bootstrap",
		Condition:   rulesengine.Any{rulesengine.ConditionFunc(IsRehearseJob), rulesengine.None{rulesengine.ConditionFunc(IsRehearseJob)}},
		Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
	
			rctx.RequiresMultiPlatformTests = true
			rctx.RequiresSprayProxyRegistering = true
			klog.Info("multi-pltform tests and require sprayproxy registering are set to TRUE")
			return nil
		})},
	}
	
var E2ERepoSetRequiredSettingsForPRRule = rulesengine.Rule{Name: "Set Required Settings for E2E Repo PR Paired Job",
		Description: "Set up required infra-deployments variables for e2e-tests repo PR paired job before bootstrap ",
		Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {
	
			if rctx.DryRun {
	
				if true {
					klog.Infof("Found infra deployments branch %s for author %s", rctx.PrBranchName, rctx.PrRemoteName)
					return true
				} else {
					klog.Infof("cannot determine infra-deployments Github branches for author %s: none. will stick with the redhat-appstudio/infra-deployments main branch for running tests", rctx.PrRemoteName)
					return false
				}
			}
	
			return IsPRPairingRequired("infra-deployments", rctx.PrRemoteName, rctx.PrBranchName)
		}),
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
	
var E2ERepoSettingsRuleChain = rulesengine.Rule{Name: "Set Require Settings E2E Repo Rule Chain",
		Description: "Rule Chain that setups up the required settings for non load-tests job types for e2e-test repo before bootstrap",
		Condition: rulesengine.All{rulesengine.None{rulesengine.ConditionFunc(IsLoadTestJob)}, &E2ERepoSetDefaultSettingsRule,
			rulesengine.Any{&E2ERepoSetRequiredSettingsForPRRule}},
	}
	
var E2ERepoCIRuleChain = rulesengine.Rule{Name: "E2E Repo CI Workflow Rule Chain",
		Description: "Execute the full workflow for e2e-tests repo in CI",
		Condition: rulesengine.All{&E2ERepoPrepareBranchRule, &PreflightInstallGinkgoRule, &E2ERepoSettingsRuleChain, &BootstrapClusterRuleChain,
			rulesengine.Any{&NonTestFilesRule, &TestFilesOnlyRule}},
	}
	
var E2ECIChainCatalog = rulesengine.RuleCatalog{E2ERepoCIRuleChain}
	
var E2ETestRulesCatalog = rulesengine.RuleCatalog{NonTestFilesRule, TestFilesOnlyRule}

func CheckNoFilesChanged(rctx *rulesengine.RuleCtx) bool {

	return len(rctx.DiffFiles) == 0
}

func CheckPkgFilesChanged(rctx *rulesengine.RuleCtx) bool {

	return len(rctx.DiffFiles.FilterByDirString("pkg/")) != 0

}

func CheckMageFilesChanged(rctx *rulesengine.RuleCtx) bool {

	return len(rctx.DiffFiles.FilterByDirString("magefiles/")) != 0

}

func CheckCmdFilesChanged(rctx *rulesengine.RuleCtx) bool {

	return len(rctx.DiffFiles.FilterByDirString("cmd/")) != 0

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
