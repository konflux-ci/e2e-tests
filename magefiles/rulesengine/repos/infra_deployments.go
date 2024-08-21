package repos

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
)

// Default Rule of repo infra-deployments running konflux-demo suite.
var InfraDeploymentsDefaultRule = rulesengine.Rule{Name: "Infra Deployments Default Test Execution",
	Description: "Run the Konflux-demo suite tests when an Infra-deployments PR includes changes to files outside of the specified components.",
	Condition: rulesengine.None{
		&InfraDeploymentsIntegrationComponentChangeRule,
		&InfraDeploymentsImageControllerComponentChangeRule,
		&InfraDeploymentsMultiPlatformComponentChangeRule,
		&InfraDeploymentsBuildTemplatesComponentChangeRule,
		&InfraDeploymentsBuildServiceComponentChangeRule,
		&InfraDeploymentsReleaseServiceComponentChangeRule,
		&InfraDeploymentsEnterpriseControllerComponentChangeRule,
		&InfraDeploymentsJVMComponentChangeRule,
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
		&InfraDeploymentsIntegrationComponentChangeRule,
		&InfraDeploymentsImageControllerComponentChangeRule,
		&InfraDeploymentsMultiPlatformComponentChangeRule,
		&InfraDeploymentsBuildTemplatesComponentChangeRule,
		&InfraDeploymentsBuildServiceComponentChangeRule,
		&InfraDeploymentsReleaseServiceComponentChangeRule,
		&InfraDeploymentsEnterpriseControllerComponentChangeRule,
		&InfraDeploymentsBuildServiceTemplatesComponentChangeRule,
		&InfraDeploymentsJVMComponentChangeRule},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		// Adding "konflux" to the label filter when component is updated
		AddLabelToLabelFilter(rctx, "konflux")
		return nil

	}),
		rulesengine.ActionFunc(ExecuteTestAction)}}

var InfraDeploymentsIntegrationComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Integration component File Change Rule",
	Description: "Map Integration tests files when Integration component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/integration/**/*")) != 0, nil

	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "integration-service")
		return nil
	})}}

var InfraDeploymentsEnterpriseControllerComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Enterprise Controller component File Change Rule",
	Description: "Map Enterprise Controller tests files when EC component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/enterprise-contract/**/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "ec")
		return nil
	})}}

var InfraDeploymentsJVMComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Jvm-build-service component File Change Rule",
	Description: "Map jvm-build-service tests files when Jvm-build-service component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/jvm-build-service/**/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "jvm-build-service")
		return nil
	})}}

var InfraDeploymentsImageControllerComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Image Controller component File Change Rule",
	Description: "Map image-controller tests files when Image Controller component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/image-controller/**/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "image-controller")
		return nil
	})}}

var InfraDeploymentsMultiPlatformComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Multi Controller component File Change Rule",
	Description: "Map multi platform tests files when Multi Controller component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/multi-platform-controller/**/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "multi-platform")
		return nil
	})}}

var InfraDeploymentsBuildTemplatesComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Build-templates component File Change Rule",
	Description: "Map build-templates tests files when Build-templates component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/build-service/base/build-pipeline-config/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "build-templates")
		return nil
	})}}

var InfraDeploymentsBuildServiceComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Build service component File Change Rule",
	Description: "Map build service tests files when Build service component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/build-service/**/*")) != 0 &&
			len(rctx.DiffFiles.FilterByDirGlob("components/build-service/base/build-pipeline-config/*")) == 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "build-service")
		return nil
	})}}

var InfraDeploymentsBuildServiceTemplatesComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Build service component and templates File Change Rule",
	Description: "Map build service tests files when Build service component and templates files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/build-service/**/*")) != 0 &&
			len(rctx.DiffFiles.FilterByDirGlob("components/build-service/base/build-pipeline-config/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "build-service")
		return nil
	})}}

var InfraDeploymentsReleaseServiceComponentChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Release service component File Change Rule",
	Description: "Map release service tests files when Release service component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		return len(rctx.DiffFiles.FilterByDirGlob("components/release/**/*")) != 0, nil
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {
		AddLabelToLabelFilter(rctx, "release-service")
		return nil
	})}}

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
