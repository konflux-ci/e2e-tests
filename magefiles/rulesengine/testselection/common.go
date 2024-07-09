package testselection

import (
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/magefile/mage/sh"
	gtypes "github.com/onsi/ginkgo/v2/types"
	"k8s.io/klog"
)

func ExecuteTestAction(rctx *rulesengine.RuleCtx, args ...any) error {

	/* This is so that we don't have ginkgo add the prefixes to
	the command args i.e. '--ginkgo.xx' || '--test.xx' || '--go.xx'
	we let ginkgo handle that when we actually run the ginkgo cmd.
	We just want the user ginkgo CLI flags we can pass to ginkgo command */

	var suiteConfig = rctx.SuiteConfig
	var reporterConfig = rctx.ReporterConfig
	var cliConfig = rctx.CLIConfig
	var goFlagsConfig = rctx.GoFlagsConfig

	var flagSet, err = gtypes.BuildRunCommandFlagSet(&suiteConfig, &reporterConfig, &cliConfig, &goFlagsConfig)

	if err != nil {
		return err
	}

	errs := gtypes.VetConfig(flagSet, suiteConfig, reporterConfig)
	if len(errs) > 0 {
		klog.Errorf("failed with %v", errs)
	}

	//We create a list of existing Ginkgo Flags
	var flags gtypes.GinkgoFlags
	flags = gtypes.SuiteConfigFlags
	flags = flags.CopyAppend(gtypes.ReporterConfigFlags...)
	flags = flags.CopyAppend(gtypes.GoRunFlags...)
	flags = flags.CopyAppend(gtypes.GinkgoCLIRunAndWatchFlags...)
	flags = flags.CopyAppend(gtypes.GinkgoCLIRunFlags...)

	//Build the bings based on what parameters were modified on struct
	bindings := map[string]interface{}{
		"S":  suiteConfig,
		"R":  reporterConfig,
		"Go": goFlagsConfig,
		"C":  cliConfig,
	}

	//Generate the user ginkgo CLI flags
	argsToRun, err := gtypes.GenerateFlagArgs(flags, bindings)

	if err != nil {
		klog.Error(err)
	}
	argsToRun = append(argsToRun, "./cmd", "--")
	return sh.RunV("ginkgo", argsToRun...)

}
