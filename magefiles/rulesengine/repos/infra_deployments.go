package repos

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
)

// InfraDeploymentsDefaultRule runs the default test suites for infra-deployments PRs.
var InfraDeploymentsDefaultRule = rulesengine.Rule{Name: "Infra Deployments Default Test Execution",
	Description: "Run the default test suites when an Infra-deployments PR includes changes to files outside of the specified components.",
	Condition: rulesengine.None{
		&InfraDeploymentsEnterpriseControllerComponentChangeRule,
		&InfraDeploymentsPipelineServiceComponentChangeRule,
		rulesengine.ConditionFunc(CheckNoFilesChanged)},

	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteInfraDeploymentsDefaultTestAction)}}

// ExecuteInfraDeploymentsDefaultTestAction excutes all the e2e-tests and component suites
func ExecuteInfraDeploymentsDefaultTestAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "konflux"
	return ExecuteTestAction(rctx)
}

// InfraDeploymentsComponentsRule defines rules of test suites running of each changed component
var InfraDeploymentsComponentsRule = rulesengine.Rule{Name: "Infra-deployments PR Components File Diff Execution",
	Description: "Runs specific tests of changed component by infra-deployments PR.",
	Condition: rulesengine.Any{
		&InfraDeploymentsEnterpriseControllerComponentChangeRule,
		&InfraDeploymentsPipelineServiceComponentChangeRule},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		// Adding "konflux" to the label filter when component is updated
		AddLabelToLabelFilter(rctx, "konflux")
		return nil

	}),
		rulesengine.ActionFunc(ExecuteTestAction)}}

var InfraDeploymentsEnterpriseControllerComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Enterprise Controller component File Change Rule",
	Description: "Map Enterprise Controller tests files when EC component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/enterprise-contract/**/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "ec")
		return nil
	})}}

var InfraDeploymentsPipelineServiceComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Pipeline Service component File Change Rule",
	Description: "Map pipeline service tests files when files in pipeline-service are changed",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
		return len(rctx.DiffFiles.FilterByDirGlob("components/pipeline-service/**/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{
		rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
			AddLabelToLabelFilter(rctx, "pipeline-service")
			return nil
		}),
	},
}

var InfraDeploymentsRulesCatalog = rulesengine.RuleCatalog{InfraDeploymentsDefaultRule, InfraDeploymentsComponentsRule}

// AddLabelToLabelFilter ensures the given label is added to the LabelFilter of rctx
func AddLabelToLabelFilter(rctx *rulesengine.RuleCtx, label string) {
	if !strings.Contains(rctx.LabelFilter, label) {
		if rctx.LabelFilter == "" {
			rctx.LabelFilter = label
		} else {
			rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, label)
		}
	}
}
