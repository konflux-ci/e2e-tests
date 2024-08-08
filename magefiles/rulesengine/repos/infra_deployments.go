package repos

import (
	"fmt"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
)

// Rule Catalog for the repo infra-deployments running all suites
var InfraDeploymentsTestRulesCatalog = rulesengine.RuleCatalog{
	rulesengine.Rule{Name: "Infra Deployments Default Test Execution",
		Description: "Run the default test suites which include the demo and components suites.",
		Condition:   rulesengine.ConditionFunc(CheckNoFilesChanged),
		Actions:     []rulesengine.Action{rulesengine.ActionFunc(ExecuteInfraDeploymentsDefaultTestAction)}},
}

// ExecuteInfraDeploymentsDefaultTestAction excutes all the e2e-tests and component suites
func ExecuteInfraDeploymentsDefaultTestAction(rctx *rulesengine.RuleCtx) error {
	rctx.LabelFilter = "konflux-demo,release-service,jvm-build-service,image-controller,integration-service,ec,build-templates,multi-platform,build-service"
	return ExecuteTestAction(rctx)
}

// Rule Catalog for the infra-deployments repo running specicfc suites
var InfraDeploymentsTestFilesOnlyRule = rulesengine.Rule{Name: "E2E PR Test File Diff Execution",
	Description: "Runs specific tests when test files are the only changes in the e2e-repo PR",
	Condition: rulesengine.All{rulesengine.Any{
		&InfraDeploymentsIntegrationTestFileChangeRule,
		&InfraDeploymentsImageControllerTestFileChangeRule,
		&InfraDeploymentsMultiPlatformTestFileChangeRule,
		&InfraDeploymentsBuildTemplatesTestFileChangeRule,
		&InfraDeploymentsBuildServiceTestFileChangeRule,
		&InfraDeploymentsReleaseServiceTestFileChangeRule,
		&InfraDeploymentsEnterpriseControllerTestFileChangeRule,
		&InfraDeploymentsJVMTestFileChangeRule}},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(ExecuteTestAction)}}

var InfraDeploymentsIntegrationTestFileChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Integration component File Change Rule",
	Description: "Map Integration tests files when Integration component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("components/integration/*")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, "integration-service")

		return nil

	})}}

var InfraDeploymentsEnterpriseControllerTestFileChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Enterprise Controller component File Change Rule",
	Description: "Map Enterprise Controller tests files when EC component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("components/enterprise-contract/*")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, "ec")

		return nil

	})}}

var InfraDeploymentsJVMTestFileChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Jvm-build-service component File Change Rule",
	Description: "Map jvm-build-service tests files when Jvm-build-service component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("components/jvm-build-service/*")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, "jvm-build-service")

		return nil

	})}}

var InfraDeploymentsImageControllerTestFileChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Image Controller component File Change Rule",
	Description: "Map image-controller tests files when Image Controller component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("components/image-controller/*")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, "image-controller")

		return nil

	})}}

var InfraDeploymentsMultiPlatformTestFileChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Multi Controller component File Change Rule",
	Description: "Map multi platform tests files when Multi Controller component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("components/multi-platform-controller/*")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, "multi-platform")

		return nil

	})}}

var InfraDeploymentsBuildTemplatesTestFileChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Build-templates component File Change Rule",
	Description: "Map build-templates tests files when Build-templates component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("components/build-templates/*")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, "build-templates")

		return nil

	})}}

var InfraDeploymentsBuildServiceTestFileChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Build service component File Change Rule",
	Description: "Map build service tests files when Build service component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("components/build-service/*")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, "build-service")

		return nil

	})}}

var InfraDeploymentsReleaseServiceTestFileChangeRule = rulesengine.Rule{Name: "Infra-deployments PR Release service component File Change Rule",
	Description: "Map release service tests files when Release service component files are changed in the infra-deployments PR",
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		return len(rctx.DiffFiles.FilterByDirGlob("components/release/*")) != 0
	}),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		rctx.LabelFilter = fmt.Sprintf("%s,%s", rctx.LabelFilter, "release-service")

		return nil

	})}}

// CheckNoComponentsFilesChanged checks if in repo infra-deployments changed files other than components
func CheckNoComponentsFilesChanged(rctx *rulesengine.RuleCtx) bool {
	// List of component directories to monitor for changes
	componentDirs := []string{
		"components/build-service/",
		"components/image-controller/",
		"components/integration/",
		"components/release/",
		"components/enterprise-contract/",
		"components/jvm-build-service/",
		"components/multi-platform/",
		"components/build-templates/",
	}
	// Check if any of the component directories have changes
	for _, dir := range componentDirs {
		if len(rctx.DiffFiles.FilterByDirString(dir)) != 0 {
			return false
		}
	}
	// If none of the component directories have changes, return true
	return true
}
