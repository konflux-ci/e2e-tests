package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	gh "github.com/google/go-github/v44/github"
	"github.com/konflux-ci/e2e-tests/magefiles/installation"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/magefile/mage/sh"
	gtypes "github.com/onsi/ginkgo/v2/types"
	"k8s.io/klog"
)

func ExecuteTestAction(rctx *rulesengine.RuleCtx) error {

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

func IsPeriodicJob(rctx *rulesengine.RuleCtx) bool {

	return rctx.JobType == "periodic"
}

func IsRehearseJob(rctx *rulesengine.RuleCtx) bool {

	return strings.Contains(rctx.JobName, "rehearse")
}

func IsSprayProxyEnabled(rctx *rulesengine.RuleCtx) bool {

	return rctx.RequiresSprayProxyRegistering
}

func IsLoadTestJob(rctx *rulesengine.RuleCtx) bool {

	return strings.Contains(rctx.JobName, "-load-test")
}

func IsSprayProxyHostSet(rctx *rulesengine.RuleCtx) bool {

	if rctx.DryRun {
		klog.Info("env var QE_SPRAYPROXY_HOST is set")
		return true
	}

	if os.Getenv("QE_SPRAYPROXY_HOST") == "" {
		klog.Errorf("env var QE_SPRAYPROXY_HOST is not set")
		return false
	}

	return true
}

func IsSprayProxyTokenSet(rctx *rulesengine.RuleCtx) bool {

	if rctx.DryRun {
		klog.Info("env var QE_SPRAYPROXY_TOKEN is set")
		return true
	}

	if os.Getenv("QE_SPRAYPROXY_TOKEN") == "" {
		klog.Errorf("env var QE_SPRAYPROXY_TOKEN is not set")
		return false
	}

	return true
}

func GitCheckoutRemoteBranch(remoteName, branchName string) error {
	var git = sh.RunCmd("git")
	for _, arg := range [][]string{
		{"remote", "add", remoteName, fmt.Sprintf("https://github.com/%s/e2e-tests.git", remoteName)},
		{"fetch", remoteName},
		{"checkout", branchName},
		{"pull", "--rebase", "upstream", "main"},
	} {
		if err := git(arg...); err != nil {
			return fmt.Errorf("error when checkout out remote branch %s from remote %s: %v", branchName, remoteName, err)
		}
	}
	return nil
}

func IsPRPairingRequired(repoForPairing string, remoteName string, branchName string) bool {
	var pullRequests []gh.PullRequest

	url := fmt.Sprintf("https://api.github.com/repos/redhat-appstudio/%s/pulls?per_page=100", repoForPairing)
	if err := sendHttpRequestAndParseResponse(url, "GET", &pullRequests); err != nil {
		klog.Infof("cannot determine %s Github branches for author %s: %v. will stick with the redhat-appstudio/%s main branch for running tests", repoForPairing, remoteName, err, repoForPairing)
		return false
	}

	for _, pull := range pullRequests {
		if pull.GetHead().GetRef() == branchName && pull.GetUser().GetLogin() == remoteName {
			return true
		}
	}

	return false
}

func IsPrelightChecked(rctx *rulesengine.RuleCtx) bool {

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

func sendHttpRequestAndParseResponse(url, method string, v interface{}) error {
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error when sending request to '%s': %+v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error when reading the response body from URL '%s': %+v", url, err)
	}
	if res.StatusCode > 299 {
		return fmt.Errorf("unexpected status code: %d, response body: %s", res.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("error when unmarshalling the response body from URL '%s': %+v", url, err)
	}

	return nil
}

func InstallKonflux() error {
	ic, err := installation.NewAppStudioInstallController()
	if err != nil {
		return fmt.Errorf("failed to initialize installation controller: %+v", err)
	}

	if err := ic.InstallAppStudioPreviewMode(); err != nil {
		return err
	}

	return nil
}

func retry(f func() error, attempts int, delay time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			klog.Infof("got an error: %+v - will retry in %v", err, delay)
			time.Sleep(delay)
		}
		err = f()
		if err != nil {
			continue
		} else {
			return nil
		}
	}
	return fmt.Errorf("reached maximum number of attempts (%d). error: %+v", attempts, err)
}

//Common Rules that can be used to chain into more specific repo rules

var PrepareBranchRule = rulesengine.Rule{Name: "Prepare E2E branch for CI",
	Description: "Checkout the e2e-tests repo for CI when the PR is paired with e2e-tests repo",
	Condition: rulesengine.All{rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {

		if rctx.DryRun {

			if true {
				klog.Infof("Found e2e-tests branch %s for author %s", rctx.PrBranchName, rctx.PrRemoteName)
				return true
			} else {
				klog.Infof("cannot determine e2e-tests Github branches for author %s: none. will stick with the redhat-appstudio/e2e-tests main branch for running tests", rctx.PrRemoteName)
				return false
			}
		}

		return IsPRPairingRequired("e2e-tests", rctx.PrRemoteName, rctx.PrBranchName)
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

		return GitCheckoutRemoteBranch(rctx.PrRemoteName, rctx.PrBranchName)
	})}}

var PreflightInstallGinkgoRule = rulesengine.Rule{Name: "Prelfight Check",
	Description: "Check the envroniment has all the minimal pre-req variables/tools installed and install ginkgo.",
	Condition:   rulesengine.ConditionFunc(IsPrelightChecked),
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		if rctx.DryRun {
			klog.Info("Installing Ginkgo Test Runner")
			klog.Info("Running command: go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo")
			klog.Info("Ginkgo Installation Complete.")
			return nil
		}
		return sh.RunV("go", "install", "-mod=mod", "github.com/onsi/ginkgo/v2/ginkgo")
	}),
	},
}

var InstallKonfluxRule = rulesengine.Rule{Name: "Install Konflux",
	Description: "Install Konflux in preview mode on a cluster.",
	Condition: rulesengine.None{rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) bool {
		return os.Getenv("SKIP_BOOTSTRAP") == "true"
	})},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		if rctx.DryRun {
			klog.Info("Installing Konflux in Preview mode.")
			klog.Info("Konflux Installation Complete.")
			return nil
		}
		return retry(InstallKonflux, 2, 10*time.Second)
	}),
	},
}

var RegisterKonfluxToSprayProxyRule = rulesengine.Rule{Name: "Register SprayProxy",
	Description: "Register Konflux with a SprayProxy.",
	Condition: rulesengine.All{rulesengine.ConditionFunc(IsSprayProxyEnabled),
		rulesengine.ConditionFunc(IsSprayProxyHostSet),
		rulesengine.ConditionFunc(IsSprayProxyTokenSet)},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		if rctx.DryRun {
			klog.Info("Registering Konflux to SprayProxy.")
			klog.Info("Registration Complete.")
			return nil
		}
		return nil
	}),
	},
}

var BootstrapClusterRuleChain = rulesengine.Rule{Name: "BoostrapCluster RuleChain",
	Description: "Rule Chain that installs Konflux in preview mode and when required registers it with a SprayProxy",
	Condition:   rulesengine.All{rulesengine.Any{&InstallKonfluxRule}, rulesengine.Any{&RegisterKonfluxToSprayProxyRule}},
}
