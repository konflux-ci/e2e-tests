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

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	gh "github.com/google/go-github/v44/github"
	"github.com/konflux-ci/e2e-tests/magefiles/installation"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/konflux-ci/e2e-tests/pkg/clients/slack"
	"github.com/konflux-ci/e2e-tests/pkg/clients/sprayproxy"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	"github.com/magefile/mage/sh"
	gtypes "github.com/onsi/ginkgo/v2/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"

	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func ExecuteTestAction(rctx *rulesengine.RuleCtx) error {

	/* This is so that we don't have ginkgo add the prefixes to
	the command args i.e. '--ginkgo.xx' || '--test.xx' || '--go.xx'
	we let ginkgo handle that when we actually run the ginkgo cmd.
	We just want the user ginkgo CLI flags we can pass to ginkgo command */

	if rctx.DryRun {
		rctx.Parallel = false
	} else {
		// Set the number of parallel test processes
		rctx.CLIConfig.Procs = 20
	}

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

func IsPeriodicJob(rctx *rulesengine.RuleCtx) (bool, error) {

	return rctx.JobType == "periodic", nil
}

func IsRehearseJob(rctx *rulesengine.RuleCtx) (bool, error) {

	return strings.Contains(rctx.JobName, "rehearse"), nil
}

func IsSprayProxyRequired(rctx *rulesengine.RuleCtx) (bool, error) {

	return rctx.RequiresSprayProxyRegistering, nil
}

func IsMultiPlatformConfigRequired(rctx *rulesengine.RuleCtx) (bool, error) {

	return rctx.RequiresMultiPlatformTests, nil
}

func IsLoadTestJob(rctx *rulesengine.RuleCtx) (bool, error) {

	return strings.Contains(rctx.JobName, "-load-test"), nil
}

func IsSprayProxyHostSet(rctx *rulesengine.RuleCtx) (bool, error) {

	if rctx.DryRun {
		klog.Info("checking if env var QE_SPRAYPROXY_HOST is set")
		return true, nil
	}

	if os.Getenv("QE_SPRAYPROXY_HOST") == "" {
		return false, fmt.Errorf("env var QE_SPRAYPROXY_HOST is not set")
	}

	return true, nil
}

func IsSprayProxyTokenSet(rctx *rulesengine.RuleCtx) (bool, error) {

	if rctx.DryRun {
		klog.Info("checking if env var QE_SPRAYPROXY_TOKEN is set")
		return true, nil
	}

	if os.Getenv("QE_SPRAYPROXY_TOKEN") == "" {

		return false, fmt.Errorf("env var QE_SPRAYPROXY_TOKEN is not set")
	}

	return true, nil
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

func IsPrelightChecked(rctx *rulesengine.RuleCtx) (bool, error) {

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
			return false, fmt.Errorf("required env vars containing secrets (%s) not defined or empty", strings.Join(missingEnv, ","))
		}

		for _, binaryName := range rctx.RequiredBinaries {
			if err := sh.Run("which", binaryName); err != nil {
				return false, fmt.Errorf("binary %s not found in PATH - please install it first", binaryName)
			}
		}
	}

	return true, nil
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
	Condition: rulesengine.All{rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {

		if rctx.DryRun {

			if true {
				klog.Infof("Found e2e-tests branch %s for author %s", rctx.PrBranchName, rctx.PrRemoteName)
				return true, nil
			}
		}

		return IsPRPairingRequired("e2e-tests", rctx.PrRemoteName, rctx.PrBranchName), nil
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

var PreflightInstallGinkgoRule = rulesengine.Rule{Name: "Preflight Check",
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
	Condition: rulesengine.ConditionFunc(func(rctx *rulesengine.RuleCtx) (bool, error) {
		return os.Getenv("SKIP_BOOTSTRAP") != "true", nil
	}),
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
	Condition: rulesengine.Any{
		rulesengine.ConditionFunc(IsSprayProxyRequired),
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		if rctx.DryRun {
			klog.Info("Registering Konflux to SprayProxy.")
			klog.Info("Registration Complete.")
			return nil
		}

		err := registerPacServer()
		if err != nil {
			os.Setenv(constants.SKIP_PAC_TESTS_ENV, "true")
			if alertErr := HandleErrorWithAlert(fmt.Errorf("failed to register SprayProxy: %+v", err), slack.ErrorSeverityLevelError); alertErr != nil {
				return alertErr
			}
		}
		return nil
	}),
	},
}

var SetupMultiPlatformTestsRule = rulesengine.Rule{Name: "Setup multi-platform tests",
	Description: "Configure tekton tasks for multi-platform tests",
	Condition: rulesengine.Any{
		rulesengine.ConditionFunc(IsMultiPlatformConfigRequired),
	},
	Actions: []rulesengine.Action{rulesengine.ActionFunc(func(rctx *rulesengine.RuleCtx) error {

		if rctx.DryRun {
			klog.Info("Setting up multi platform tests.")
			klog.Info("Multi platform tests configured.")
			return nil
		}

		return SetupMultiPlatformTests()
	}),
	},
}

var BootstrapClusterRuleChain = rulesengine.Rule{Name: "BoostrapCluster RuleChain",
	Description: "Rule Chain that installs Konflux in preview mode and when required, registers it with a SprayProxy and sets up MP tests",
	Condition:   rulesengine.All{&InstallKonfluxRule, &RegisterKonfluxToSprayProxyRule, &SetupMultiPlatformTestsRule},
}

func registerPacServer() error {
	var err error
	var pacHost string
	sprayProxyConfig, err := newSprayProxy()
	if err != nil {
		return fmt.Errorf("failed to set up SprayProxy credentials: %+v", err)
	}

	pacHost, err = sprayproxy.GetPaCHost()
	if err != nil {
		return fmt.Errorf("failed to get PaC host: %+v", err)
	}
	_, err = sprayProxyConfig.RegisterServer(pacHost)
	if err != nil {
		return fmt.Errorf("error when registering PaC server %s to SprayProxy server %s: %+v", pacHost, sprayProxyConfig.BaseURL, err)
	}
	klog.Infof("Registered PaC server: %s", pacHost)
	// for debugging purposes
	err = printRegisteredPacServers(sprayProxyConfig)
	if err != nil {
		klog.Error(err)
	}
	return nil
}

func newSprayProxy() (*sprayproxy.SprayProxyConfig, error) {
	var sprayProxyUrl, sprayProxyToken string
	if sprayProxyUrl = os.Getenv("QE_SPRAYPROXY_HOST"); sprayProxyUrl == "" {
		return nil, fmt.Errorf("env var QE_SPRAYPROXY_HOST is not set")
	}
	if sprayProxyToken = os.Getenv("QE_SPRAYPROXY_TOKEN"); sprayProxyToken == "" {
		return nil, fmt.Errorf("env var QE_SPRAYPROXY_TOKEN is not set")
	}
	return sprayproxy.NewSprayProxyConfig(sprayProxyUrl, sprayProxyToken)
}

func printRegisteredPacServers(cfg *sprayproxy.SprayProxyConfig) error {
	servers, err := cfg.GetServers()
	if err != nil {
		return fmt.Errorf("failed to get registered PaC servers from SprayProxy: %+v", err)
	}
	klog.Infof("The PaC servers registered in Sprayproxy: %v", servers)
	return nil
}

func HandleErrorWithAlert(err error, errLevel slack.ErrorSeverityLevel) error {
	klog.Warning(err.Error() + " - this issue will be reported to a dedicated Slack channel")

	if slackErr := slack.ReportIssue(err.Error(), errLevel); slackErr != nil {
		return fmt.Errorf("failed report an error (%s) to a Slack channel: %s", err, slackErr)
	}
	return nil
}

func SetupMultiPlatformTests() error {
	var platforms = []string{"linux/arm64", "linux/s390x", "linux/ppc64le"}

	klog.Infof("going to create new Tekton bundle remote-build for the purpose of testing multi-platform-controller PR")
	var err error
	var defaultBundleRef string
	var tektonObj runtime.Object

	for _, platformType := range platforms {
		tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
		quayOrg := utils.GetEnv(constants.DEFAULT_QUAY_ORG_ENV, constants.DefaultQuayOrg)
		newMultiPlatformBuilderPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
		var newRemotePipeline, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newMultiPlatformBuilderPipelineImg, tag))
		var newPipelineYaml []byte

		if err = utils.CreateDockerConfigFile(os.Getenv("QUAY_TOKEN")); err != nil {
			return fmt.Errorf("failed to create docker config file: %+v", err)
		}
		if defaultBundleRef, err = tekton.GetDefaultPipelineBundleRef(constants.BuildPipelineConfigConfigMapYamlURL, "docker-build"); err != nil {
			return fmt.Errorf("failed to get the pipeline bundle ref: %+v", err)
		}
		if tektonObj, err = tekton.ExtractTektonObjectFromBundle(defaultBundleRef, "pipeline", "docker-build"); err != nil {
			return fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
		}
		dockerPipelineObject := tektonObj.(*tektonapi.Pipeline)

		var currentBuildahTaskRef string
		for i := range dockerPipelineObject.PipelineSpec().Tasks {
			t := &dockerPipelineObject.PipelineSpec().Tasks[i]
			params := t.TaskRef.Params
			var lastBundle *tektonapi.Param
			var lastName *tektonapi.Param
			buildahTask := false
			for i, param := range params {
				if param.Name == "bundle" {
					lastBundle = &t.TaskRef.Params[i]
				} else if param.Name == "name" && param.Value.StringVal == "buildah" {
					lastName = &t.TaskRef.Params[i]
					buildahTask = true
				}
			}
			if buildahTask {
				currentBuildahTaskRef = lastBundle.Value.StringVal
				klog.Infof("Found current task ref %s", currentBuildahTaskRef)
				//TODO: current use pinned sha?
				lastBundle.Value = *tektonapi.NewStructuredValues("quay.io/redhat-appstudio-tekton-catalog/task-buildah-remote:0.1-ac185e95bbd7a25c1c4acf86995cbaf30eebedc4")
				lastName.Value = *tektonapi.NewStructuredValues("buildah-remote")
				t.Params = append(t.Params, tektonapi.Param{Name: "PLATFORM", Value: *tektonapi.NewStructuredValues("$(params.PLATFORM)")})
				dockerPipelineObject.Spec.Params = append(dockerPipelineObject.PipelineSpec().Params, tektonapi.ParamSpec{Name: "PLATFORM", Default: tektonapi.NewStructuredValues(platformType)})
				dockerPipelineObject.Name = "buildah-remote-pipeline"
				break
			}
		}
		if currentBuildahTaskRef == "" {
			return fmt.Errorf("failed to extract the Tekton Task from bundle: %+v", err)
		}
		if newPipelineYaml, err = yaml.Marshal(dockerPipelineObject); err != nil {
			return fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
		}

		keychain := authn.NewMultiKeychain(authn.DefaultKeychain)
		authOption := remoteimg.WithAuthFromKeychain(keychain)

		if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newRemotePipeline, authOption); err != nil {
			return fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
		}
		platform := strings.ToUpper(strings.Split(platformType, "/")[1])
		klog.Infof("SETTING ENV VAR %s to value %s\n", constants.CUSTOM_BUILDAH_REMOTE_PIPELINE_BUILD_BUNDLE_ENV+"_"+platform, newRemotePipeline.String())
		os.Setenv(constants.CUSTOM_BUILDAH_REMOTE_PIPELINE_BUILD_BUNDLE_ENV+"_"+platform, newRemotePipeline.String())
	}

	return nil
}

func SetEnvVarsForComponentImageDeployment(rctx *rulesengine.RuleCtx) error {
	componentImage := os.Getenv("COMPONENT_IMAGE")
	sp := strings.Split(componentImage, "@")
	if len(sp) != 2 {
		return fmt.Errorf("component image ref expected in format 'quay.io/<org>/<repository>@sha256:<sha256-value>', got %s", componentImage)
	}
	os.Setenv(fmt.Sprintf("%s_IMAGE_REPO", rctx.ComponentEnvVarPrefix), sp[0])
	os.Setenv(fmt.Sprintf("%s_IMAGE_TAG", rctx.ComponentEnvVarPrefix), rctx.ComponentImageTag)
	os.Setenv(fmt.Sprintf("%s_PR_OWNER", rctx.ComponentEnvVarPrefix), rctx.PrRemoteName)
	os.Setenv(fmt.Sprintf("%s_PR_SHA", rctx.ComponentEnvVarPrefix), rctx.PrCommitSha)
	return nil
}
