package repos

import (
	"strings"
	"strconv"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
)

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

var ReleaseServiceCatalogTestRulesCatalog = rulesengine.RuleCatalog{ReleaseServiceCatalogRule, ReleaseCatalogPairedRule}

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

func ExecuteReleasePairedAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "release-pipelines && !fbc-tests"
	return ExecuteTestAction(rctx)
}

func ExecuteReleaseCatalogAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "release-pipelines"
	return ExecuteTestAction(rctx)
}
