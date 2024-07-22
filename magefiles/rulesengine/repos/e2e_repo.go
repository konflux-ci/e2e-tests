package repos

import (
	"strings"
	"strconv"
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
	Condition: rulesengine.None{rulesengine.ConditionFunc(CheckPkgFilesChanged),
		rulesengine.ConditionFunc(CheckMageFilesChanged),
		rulesengine.ConditionFunc(CheckCmdFilesChanged),
		rulesengine.ConditionFunc(CheckNoFilesChanged)},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteFocusedFileAction)}}

var ReleaseCatalogPairedRule = rulesengine.Rule{Name: "Release Catalog PR paired Test Execution",
	Description: "Runs release catalog tests except for the fbc tests on release-service-catalog repo when PR paired and not a rehearsal job",
	Condition: rulesengine.All{rulesengine.ConditionFunc(releaseCatalogRepoCondition),
		rulesengine.ConditionFunc(isPaired),
		rulesengine.None{
			rulesengine.ConditionFunc(isRehearse),
		},
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteReleasePairedAction)}}

var ReleaseServiceCatalogRule = rulesengine.Rule{Name: "Release Service Catalog Test Execution",
	Description: "Runs all release catalog tests on release-service-catalog repo on PR/rehearsal jobs",
	Condition: rulesengine.All{rulesengine.ConditionFunc(releaseCatalogRepoCondition),
		rulesengine.Any{
			rulesengine.None{rulesengine.ConditionFunc(isPaired),},
			rulesengine.ConditionFunc(isRehearse),
		},
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteReleaseCatalogAction)}}

var E2ETestRulesCatalog = rulesengine.RuleCatalog{NonTestFilesRule, TestFilesOnlyRule, ReleaseServiceCatalogRule, ReleaseCatalogPairedRule}

var isRehearse = func(rctx *rulesengine.RuleCtx) bool {

	return strings.Contains(rctx.JobName, "rehearse")
}

var isPaired = func(rctx *rulesengine.RuleCtx) bool {

	if boolValue, err := strconv.ParseBool(rctx.IsPaired); err == nil && boolValue {
		return true
	}
	return false
}

func releaseCatalogRepoCondition(rctx *rulesengine.RuleCtx) bool {

	return rctx.RepoName == "release-service-catalog"
}

func CheckNoFilesChanged(rctx *rulesengine.RuleCtx) bool {

	return len(rctx.DiffFiles) == 0 || len(rctx.DiffFiles.FilterByStatus("D")) == 0
}

func CheckPkgFilesChanged(rctx *rulesengine.RuleCtx) bool {

	for _, file := range rctx.DiffFiles {

		switch {
		case strings.Contains(file.Name, "pkg/"):
			return true
		}

	}

	return false

}

func CheckMageFilesChanged(rctx *rulesengine.RuleCtx) bool {

	for _, file := range rctx.DiffFiles {

		switch {
		case strings.Contains(file.Name, "magefile/"):
			return true
		}

	}

	return false

}

func CheckCmdFilesChanged(rctx *rulesengine.RuleCtx) bool {

	for _, file := range rctx.DiffFiles {

		switch {

		case strings.Contains(file.Name, "cmd/"):
			return true
		}

	}

	return false

}

func ExecuteDefaultTestAction(rctx *rulesengine.RuleCtx) error {

	rctx.LabelFilter = "!upgrade-create && !upgrade-verify && !upgrade-cleanup && !release-pipelines"

	return ExecuteTestAction(rctx)

}

func ExecuteFocusedFileAction(rctx *rulesengine.RuleCtx) error {

	for _, file := range rctx.DiffFiles.FilterByStatus("D") {

		rctx.FocusFiles = append(rctx.FocusFiles, file.Name)

	}

	return ExecuteTestAction(rctx)

}

func ExecuteReleasePairedAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "release-pipelines && !fbc-tests"
	return ExecuteTestAction(rctx)
}

func ExecuteReleaseCatalogAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "release-pipelines"
	return ExecuteTestAction(rctx)
}
