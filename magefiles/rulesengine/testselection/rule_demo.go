package testselection

import (
	"os"
	"strings"

	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/magefile/mage/sh"
	"k8s.io/klog"
)

// Demo of the magefile, PreflightChecks(), as a ConditionalFunc within the rule framework.
// But really we would actually register the real function with minor tweaks if we opt into framework
var isPrelightCheck = func(rctx *rulesengine.RuleCtx) bool {

	if rctx.DryRun {

		klog.Info("All Environment Variables have been set!")
		klog.Info("All tools and commands have been found!")
	} else {
		requiredEnv := []string{
			"GITHUB_TOKEN",
			"QUAY_TOKEN",
			"DEFAULT_QUAY_ORG",
			"DEFAULT_QUAY_ORG_TOKEN",
		}
		missingEnv := []string{}
		for _, env := range requiredEnv {
			if os.Getenv(env) == "" {
				missingEnv = append(missingEnv, env)
			}
		}
		if len(missingEnv) != 0 {
			klog.Errorf("required env vars containing secrets (%s) not defined or empty", strings.Join(missingEnv, ","))
			return false
		}

		for _, binaryName := range rctx.RequiredBinaries {
			if err := sh.Run("which", binaryName); err != nil {
				klog.Errorf("binary %s not found in PATH - please install it first", binaryName)
				return false
			}
		}
	}

	return true
}

// Demo of magefile, func BootstrapCluster(), as an ActionFunc within the rule framework
// But really we would actually register the real function with minor tweaks if we opt into framework
var bootstrapCluster = func(rctx *rulesengine.RuleCtx) error {

	klog.Info("BootStrap preparation")
	klog.Info("Installing Konflux in Preview mode.")
	klog.Info("Konflux Installation Complete.")

	return nil
}

// Demo of magefile, func , as an ActionFunc within the rule framework
// Pulling out the ginkgo install as a separate declarable action rather
// than burying it in a conditional check
var installGinkgo = func(rctx *rulesengine.RuleCtx) error {

	klog.Info("Installing Ginkgo Test Runner")
	klog.Info("Running command: go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo")
	klog.Info("Ginkgo Installation Complete.")

	return nil
}

// Demo of what the magefile, func (Local) PrepareCluster(), would look like as a Rule in the rule framework
// The rule encapsulates the businnes logic we already have of:
// WHEN environment has required prereqs THEN install ginkgo, boostrap cluster
var preflight_check_rule = rulesengine.Rule{Name: "Bootstrap a Cluster",
	Description: "Boostrap the cluster when the envroniment has all the pre-req environment variables/tools installed.",
	Condition:    rulesengine.ConditionFunc(isPrelightCheck),
	Actions:     []rulesengine.Action{rulesengine.ActionFunc(installGinkgo), rulesengine.ActionFunc(bootstrapCluster)},
}

//Demo of what the magefile, func (Local) PrepareCluster() AND func (Local) TestE2E(), would look like as a RuleChain in the rule framework
//We are using the Preflight Check rule created above and chaining it with the two test selection rules from the e2e_repo.go.
//The rule encapsulates the business logic we already have:
// WHEN environment has required prereqs THEN install ginkgo and boostrap cluster AND WHEN non-test files change THEN
// execute default test files OR WHEN test files are the only change THEN execute those specific test files.

var LocalE2EDemoRuleChain = rulesengine.Rule{Name: "Local Install and Test Run of e2e-repo",
	Description: "Install Konflux to a cluster and run tests based on file changes within the e2e-repo when executed from local system.",
	Condition:    rulesengine.All{&preflight_check_rule, rulesengine.Any{&NonTestFilesRule, &TestFilesOnlyRule}},
}

var DemoCatalog = rulesengine.RuleCatalog{LocalE2EDemoRuleChain}
