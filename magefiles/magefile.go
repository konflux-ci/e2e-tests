package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"k8s.io/klog/v2"

	"sigs.k8s.io/yaml"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	gh "github.com/google/go-github/v44/github"
	"github.com/konflux-ci/e2e-tests/magefiles/installation"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine/engine"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine/repos"
	"github.com/konflux-ci/e2e-tests/magefiles/upgrade"
	"github.com/konflux-ci/e2e-tests/pkg/clients/github"
	"github.com/konflux-ci/e2e-tests/pkg/clients/gitlab"
	"github.com/konflux-ci/e2e-tests/pkg/clients/slack"
	"github.com/konflux-ci/e2e-tests/pkg/clients/sprayproxy"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/testspecs"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	"github.com/konflux-ci/image-controller/pkg/quay"
	"github.com/magefile/mage/sh"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	gl "github.com/xanzy/go-gitlab"
)

const (
	quayApiUrl       = "https://quay.io/api/v1"
	gitopsRepository = "GitOps Repository"
)

var (
	requiredBinaries = []string{"jq", "kubectl", "oc", "yq", "git"}
	artifactDir      = utils.GetEnv("ARTIFACT_DIR", ".")
	openshiftJobSpec = &OpenshiftJobSpec{}
	pr               = &PullRequestMetadata{}
	konfluxCI        = os.Getenv("KONFLUX_CI")
	jobName          = utils.GetEnv("JOB_NAME", "")
	// can be periodic, presubmit or postsubmit
	jobType                    = utils.GetEnv("JOB_TYPE", "")
	reposToDeleteDefaultRegexp = "jvm-build|e2e-dotnet|build-suite|e2e|pet-clinic-e2e|test-app|e2e-quayio|petclinic|test-app|integ-app|^dockerfile-|new-|^python|my-app|^test-|^multi-component|^devfile-sample-hello-world-\\S{6}$|^build-nudge-parent-\\S{6}$|^build-nudge-child-\\S{6}$"
	repositoriesWithWebhooks   = []string{"devfile-sample-hello-world", "hacbs-test-project", "secret-lookup-sample-repo-two"}
	// determine whether CI will run tests that require to register SprayProxy
	// in order to run tests that require PaC application
	requiresSprayProxyRegistering bool

	requiresMultiPlatformTests bool
	platforms                  = []string{"linux/arm64", "linux/s390x", "linux/ppc64le"}

	sprayProxyConfig       *sprayproxy.SprayProxyConfig
	quayTokenNotFoundError = "DEFAULT_QUAY_ORG_TOKEN env var was not found"

	konfluxCiSpec = &KonfluxCISpec{}

	rctx = &rulesengine.RuleCtx{}
)

func (CI) parseJobSpec() error {
	jobSpecEnvVarData := os.Getenv("JOB_SPEC")

	if konfluxCI == "true" {
		if err := json.Unmarshal([]byte(jobSpecEnvVarData), konfluxCiSpec); err != nil {
			return fmt.Errorf("error when parsing openshift job spec data: %v", err)
		}
		return nil
	}

	if err := json.Unmarshal([]byte(jobSpecEnvVarData), openshiftJobSpec); err != nil {
		return fmt.Errorf("error when parsing openshift job spec data: %v", err)
	}
	return nil
}

func (ci CI) init() error {
	var err error

	if jobType == "periodic" || strings.Contains(jobName, "rehearse") {
		return nil
	}

	if err = ci.parseJobSpec(); err != nil {
		return err
	}

	if konfluxCI == "true" {
		pr.Organization = konfluxCiSpec.KonfluxGitRefs.GitOrg
		// Workaround to fix the incompatibility between test-metadata task v0.1 and v0.3
		if pr.Organization == "" {
			pr.Organization = konfluxCiSpec.KonfluxGitRefs.Org
		}
		pr.RepoName = konfluxCiSpec.KonfluxGitRefs.GitRepo
		// Workaround to fix the incompatibility between test-metadata task v0.1 and v0.3
		if pr.RepoName == "" {
			pr.RepoName = konfluxCiSpec.KonfluxGitRefs.Repo
		}
		pr.CommitSHA = konfluxCiSpec.KonfluxGitRefs.CommitSha
		pr.Number = konfluxCiSpec.KonfluxGitRefs.PullRequestNumber
	} else {
		pr.Organization = openshiftJobSpec.Refs.Organization
		pr.RepoName = openshiftJobSpec.Refs.Repo
		pr.CommitSHA = openshiftJobSpec.Refs.Pulls[0].SHA
		pr.Number = openshiftJobSpec.Refs.Pulls[0].Number
	}

	if konfluxCiSpec.KonfluxGitRefs.EventType != "push" {
		prUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", pr.Organization, pr.RepoName, pr.Number)
		pr.RemoteName, pr.BranchName, err = getRemoteAndBranchNameFromPRLink(prUrl)
		if err != nil {
			return fmt.Errorf("cannot get remote name and branch name for PR URL %q: %+v", prUrl, err)
		}
	} else if konfluxCiSpec.KonfluxGitRefs.EventType == "push" && konfluxCiSpec.KonfluxGitRefs.GitRepo == "release-service-catalog" {
		pr.RemoteName = "konflux-ci"
		pr.BranchName = "staging"
	}

	rctx = rulesengine.NewRuleCtx()

	rctx.Parallel = true
	rctx.OutputDir = artifactDir
	rctx.JUnitReport = "e2e-report.xml"
	rctx.JSONReport = "e2e-report.json"

	rctx.RepoName = pr.RepoName
	rctx.JobName = jobName
	rctx.JobType = jobType
	rctx.PrRemoteName = pr.RemoteName
	rctx.PrBranchName = pr.BranchName
	rctx.PrCommitSha = pr.CommitSHA
	rctx.PrNum = pr.Number

	if konfluxCI == "true" {
		rctx.TektonEventType = konfluxCiSpec.KonfluxGitRefs.EventType
	}

	return nil
}

func (ci CI) PrepareE2EBranch() error {
	if jobType == "periodic" || strings.Contains(jobName, "rehearse") {
		return nil
	}

	if err := ci.init(); err != nil {
		return err
	}

	if pr.RepoName == "e2e-tests" {
		if err := gitCheckoutRemoteBranch(pr.RemoteName, pr.CommitSHA); err != nil {
			return err
		}
	} else {
		if isPRPairingRequired("e2e-tests") {
			if err := gitCheckoutRemoteBranch(pr.RemoteName, pr.BranchName); err != nil {
				return err
			}
		}
	}

	return nil
}

func (Local) PrepareCluster() error {
	if err := PreflightChecks(); err != nil {
		return fmt.Errorf("error when running preflight checks: %v", err)
	}
	if err := BootstrapCluster(); err != nil {
		return fmt.Errorf("error when bootstrapping cluster: %v", err)
	}
	if err := SetupCustomBundle(); err != nil {
		return fmt.Errorf("error while setting up custom bundle: %v", err)
	}
	return nil
}

func (Local) TestE2E() error {
	return RunE2ETests()
}

// Deletes autogenerated or test generated repositories from redhat-appstudio-qe Github org.
// Env vars to configure this target: REPO_REGEX (optional), DRY_RUN (optional) - defaults to false
// Remove all repos which with 1 day lifetime. By default will delete gitops repositories from redhat-appstudio-qe
func (Local) CleanupGithubOrg() error {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		return fmt.Errorf("env var GITHUB_TOKEN is not set")
	}
	dryRun, err := strconv.ParseBool(utils.GetEnv("DRY_RUN", "true"))
	if err != nil {
		return fmt.Errorf("unable to parse DRY_RUN env var\n\t%s", err)
	}

	// Get all repos
	githubOrgName := utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
	ghClient, err := github.NewGithubClient(githubToken, githubOrgName)
	if err != nil {
		return err
	}
	repos, err := ghClient.GetAllRepositories()
	if err != nil {
		return err
	}
	var reposToDelete []*gh.Repository

	// Filter repos by regex & time check
	r, err := regexp.Compile(utils.GetEnv("REPO_REGEX", reposToDeleteDefaultRegexp))
	if err != nil {
		return fmt.Errorf("unable to compile regex: %s", err)
	}
	for _, repo := range repos {
		// Add only repos older than 24 hours
		dayDuration, _ := time.ParseDuration("24h")
		if time.Since(repo.GetCreatedAt().Time) > dayDuration {
			// Add only repos matching the regex
			if r.MatchString(repo.GetName()) || repo.GetDescription() == gitopsRepository {
				reposToDelete = append(reposToDelete, repo)
			}
		}
	}

	if dryRun {
		klog.Info("Dry run enabled. Listing repositories that would be deleted:")
	}

	// Delete repos
	for _, repo := range reposToDelete {
		if dryRun {
			klog.Infof("\t%s", repo.GetName())
		} else {
			err := ghClient.DeleteRepository(repo)
			if err != nil {
				klog.Warningf("error deleting repository: %s\n", err)
			}
		}
	}
	if dryRun {
		klog.Info("If you really want to delete these repositories, run `DRY_RUN=false [REGEXP=<regexp>] mage local:cleanupGithubOrg`")
	}
	return nil
}

// Deletes Quay repos and robot accounts older than 24 hours with prefixes `has-e2e` and `e2e-demos`, uses env vars DEFAULT_QUAY_ORG and DEFAULT_QUAY_ORG_TOKEN
func (Local) CleanupQuayReposAndRobots() error {
	quayOrgToken := os.Getenv("DEFAULT_QUAY_ORG_TOKEN")
	if quayOrgToken == "" {
		return fmt.Errorf("%s", quayTokenNotFoundError)
	}
	quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "redhat-appstudio-qe")

	quayClient := quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayOrgToken, quayApiUrl)
	return cleanupQuayReposAndRobots(quayClient, quayOrg)
}

// Deletes Quay Tags older than 7 days in `test-images` repository
func (Local) CleanupQuayTags() error {
	quayOrgToken := os.Getenv("DEFAULT_QUAY_ORG_TOKEN")
	if quayOrgToken == "" {
		return fmt.Errorf("%s", quayTokenNotFoundError)
	}
	quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "redhat-appstudio-qe")

	quayClient := quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayOrgToken, quayApiUrl)
	return cleanupQuayTags(quayClient, quayOrg, "test-images")
}

// Deletes the private repos whose names match prefixes as stored in `repoNamePrefixes` array
func (Local) CleanupPrivateRepos() error {
	repoNamePrefixes := []string{"build-e2e", "konflux", "multi-platform", "jvm-build-service"}
	quayOrgToken := os.Getenv("DEFAULT_QUAY_ORG_TOKEN")
	if quayOrgToken == "" {
		return fmt.Errorf("%s", quayTokenNotFoundError)
	}
	quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "redhat-appstudio-qe")

	quayClient := quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayOrgToken, quayApiUrl)
	return cleanupPrivateRepos(quayClient, quayOrg, repoNamePrefixes)
}

func (ci CI) Bootstrap() error {
	if err := ci.init(); err != nil {
		return fmt.Errorf("error when running ci init: %v", err)
	}

	if err := BootstrapCluster(); err != nil {
		return fmt.Errorf("error when bootstrapping cluster: %v", err)
	}
	return nil
}

func (ci CI) PerformOpenShiftUpgrade() error {
	if err := upgrade.PerformUpgrade(); err != nil {
		return err
	}
	return nil
}

func (ci CI) TestE2E() error {

	if err := ci.init(); err != nil {
		return fmt.Errorf("error when running ci init: %v", err)
	}

	// Eventually, when mage rules will be in place for all the repos, this functionality will be moved to individual repos where it is needed
	if err := SetupCustomBundle(); err != nil {
		return err
	}

	// Eventually we'll introduce mage rules for all repositories, so this condition won't be needed anymore
	if pr.RepoName == "e2e-tests" || pr.RepoName == "integration-service" ||
		pr.RepoName == "release-service" || pr.RepoName == "image-controller" ||
		pr.RepoName == "build-service" || pr.RepoName == "release-service-catalog" {
		return engine.MageEngine.RunRulesOfCategory("ci", rctx)
	}

	if err := PreflightChecks(); err != nil {
		return fmt.Errorf("error when running preflight checks: %v", err)
	}

	if os.Getenv("SKIP_BOOTSTRAP") != "true" {
		if err := retry(BootstrapCluster, 2, 10*time.Second); err != nil {
			return fmt.Errorf("error when bootstrapping cluster: %v", err)
		}
	} else {
		if err := setRequiredEnvVars(); err != nil {
			return fmt.Errorf("error when setting up required env vars: %v", err)
		}
	}

	if requiresMultiPlatformTests {
		if err := SetupMultiPlatformTests(); err != nil {
			return err
		}
	}

	if err := RunE2ETests(); err != nil {
		return fmt.Errorf("error when running e2e tests: %+v", err)
	}

	return nil
}

func (ci CI) UnregisterSprayproxy() {
	err := unregisterPacServer()
	if err != nil {
		if alertErr := HandleErrorWithAlert(fmt.Errorf("failed to unregister SprayProxy: %+v", err), slack.ErrorSeverityLevelInfo); alertErr != nil {
			klog.Warning(alertErr)
		}
	}
}

func RunE2ETests() error {
	var err error
	rctx.DiffFiles, err = utils.GetChangedFiles(rctx.RepoName)
	if err != nil {
		return err
	}
	switch rctx.RepoName {
	case "release-service-catalog":
		rctx.IsPaired = isPRPairingRequired("release-service")
		return engine.MageEngine.RunRules(rctx, "tests", "release-service-catalog")
	case "infra-deployments":
		return engine.MageEngine.RunRules(rctx, "tests", "infra-deployments")
	default:
		labelFilter := utils.GetEnv("E2E_TEST_SUITE_LABEL", "!upgrade-create && !upgrade-verify && !upgrade-cleanup && !release-pipelines")
		return runTests(labelFilter, "e2e-report.xml")
	}
}

func PreflightChecks() error {
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
		return fmt.Errorf("required env vars containing secrets (%s) not defined or empty", strings.Join(missingEnv, ","))
	}

	for _, binaryName := range requiredBinaries {
		if err := sh.Run("which", binaryName); err != nil {
			return fmt.Errorf("binary %s not found in PATH - please install it first", binaryName)
		}
	}

	if err := sh.RunV("go", "install", "-mod=mod", "github.com/onsi/ginkgo/v2/ginkgo"); err != nil {
		return err
	}

	return nil
}

func setRequiredEnvVars() error {
	// Load test jobs require no additional setup
	if strings.Contains(jobName, "-load-test") {
		return nil
	}
	// Konflux Nightly E2E job
	if strings.Contains(jobName, "-periodic") {
		requiresMultiPlatformTests = true
		requiresSprayProxyRegistering = true
		return nil
	}

	if openshiftJobSpec.Refs.Repo != "e2e-tests" {

		if strings.HasSuffix(jobName, "-service-e2e") || strings.Contains(jobName, "image-controller") {
			var envVarPrefix, imageTagSuffix, testSuiteLabel string
			sp := strings.Split(os.Getenv("COMPONENT_IMAGE"), "@")

			switch {
			case strings.Contains(jobName, "application-service"):
				requiresSprayProxyRegistering = true
				envVarPrefix = "HAS"
				imageTagSuffix = "has-image"
				testSuiteLabel = "konflux"
			case strings.Contains(jobName, "release-service"):
				envVarPrefix = "RELEASE_SERVICE"
				imageTagSuffix = "release-service-image"
				testSuiteLabel = "release-service"
				os.Setenv(fmt.Sprintf("%s_CATALOG_REVISION", envVarPrefix), "development")
			case strings.Contains(jobName, "integration-service"):
				requiresSprayProxyRegistering = true
				envVarPrefix = "INTEGRATION_SERVICE"
				imageTagSuffix = "integration-service-image"
				testSuiteLabel = "integration-service"
			case strings.Contains(jobName, "build-service"):
				requiresSprayProxyRegistering = true
				envVarPrefix = "BUILD_SERVICE"
				imageTagSuffix = "build-service-image"
				testSuiteLabel = "build-service"
			case strings.Contains(jobName, "image-controller"):
				requiresSprayProxyRegistering = true
				envVarPrefix = "IMAGE_CONTROLLER"
				imageTagSuffix = "image-controller-image"
				testSuiteLabel = "image-controller"
			case strings.Contains(jobName, "multi-platform-controller"):
				envVarPrefix = "MULTI_PLATFORM_CONTROLLER"
				imageTagSuffix = "multi-platform-controller"
				testSuiteLabel = "multi-platform"
				requiresMultiPlatformTests = true
			}

			os.Setenv(fmt.Sprintf("%s_IMAGE_REPO", envVarPrefix), sp[0])
			os.Setenv(fmt.Sprintf("%s_IMAGE_TAG", envVarPrefix), fmt.Sprintf("redhat-appstudio-%s", imageTagSuffix))
			// "rehearse" jobs metadata are not relevant for testing
			if !strings.Contains(jobName, "rehearse") {
				os.Setenv(fmt.Sprintf("%s_PR_OWNER", envVarPrefix), pr.RemoteName)
				os.Setenv(fmt.Sprintf("%s_PR_SHA", envVarPrefix), pr.CommitSHA)
			}
			// Allow pairing component repo PR + e2e-tests PR + infra-deployments PR
			if isPRPairingRequired("infra-deployments") {
				os.Setenv("INFRA_DEPLOYMENTS_ORG", pr.RemoteName)
				os.Setenv("INFRA_DEPLOYMENTS_BRANCH", pr.BranchName)
			}

			os.Setenv("E2E_TEST_SUITE_LABEL", testSuiteLabel)

		} else if openshiftJobSpec.Refs.Repo == "infra-deployments" {
			requiresMultiPlatformTests = true
			requiresSprayProxyRegistering = true
			os.Setenv("INFRA_DEPLOYMENTS_ORG", pr.RemoteName)
			os.Setenv("INFRA_DEPLOYMENTS_BRANCH", pr.BranchName)
			/* Disabling "build tests" temporary due:
			TODO: Enable when issues are done:
			https://issues.redhat.com/browse/RHTAPBUGS-992, https://issues.redhat.com/browse/RHTAPBUGS-991, https://issues.redhat.com/browse/RHTAPBUGS-989,
			https://issues.redhat.com/browse/RHTAPBUGS-978,https://issues.redhat.com/browse/RHTAPBUGS-956
			*/
			os.Setenv("E2E_TEST_SUITE_LABEL", "e2e-demo,konflux,integration-service,ec,build-templates,multi-platform")
		} else if strings.Contains(jobName, "release-service-catalog") { // release-service-catalog jobs (pull, rehearsal)
			envVarPrefix := "RELEASE_SERVICE"
			os.Setenv("E2E_TEST_SUITE_LABEL", "release-pipelines")
			// "rehearse" jobs metadata are not relevant for testing
			if !strings.Contains(jobName, "rehearse") {
				os.Setenv(fmt.Sprintf("%s_CATALOG_URL", envVarPrefix), fmt.Sprintf("https://github.com/%s/%s", pr.RemoteName, pr.RepoName))
				os.Setenv(fmt.Sprintf("%s_CATALOG_REVISION", envVarPrefix), pr.CommitSHA)
				if isPRPairingRequired("release-service") {
					os.Setenv(fmt.Sprintf("%s_IMAGE_REPO", envVarPrefix),
						"quay.io/redhat-user-workloads/rhtap-release-2-tenant/release-service/release-service")
					pairedSha := repos.GetPairedCommitSha("release-service", rctx)
					if pairedSha != "" {
						os.Setenv(fmt.Sprintf("%s_IMAGE_TAG", envVarPrefix), fmt.Sprintf("on-pr-%s", pairedSha))
					}
					os.Setenv(fmt.Sprintf("%s_PR_OWNER", envVarPrefix), pr.RemoteName)
					os.Setenv(fmt.Sprintf("%s_PR_SHA", envVarPrefix), pairedSha)
					os.Setenv("E2E_TEST_SUITE_LABEL", "release-pipelines && !fbc-tests")
				}
			}
			if os.Getenv("REL_IMAGE_CONTROLLER_QUAY_ORG") != "" {
				os.Setenv("IMAGE_CONTROLLER_QUAY_ORG", os.Getenv("REL_IMAGE_CONTROLLER_QUAY_ORG"))
			}
			if os.Getenv("REL_IMAGE_CONTROLLER_QUAY_TOKEN") != "" {
				os.Setenv("IMAGE_CONTROLLER_QUAY_TOKEN", os.Getenv("REL_IMAGE_CONTROLLER_QUAY_TOKEN"))
			}
		} else { // openshift/release rehearse job for e2e-tests/infra-deployments repos
			requiresMultiPlatformTests = true
			requiresSprayProxyRegistering = true
		}
	}

	return nil
}

func SetupCustomBundle() error {
	klog.Infof("setting up new custom bundle for testing...")
	customDockerBuildBundle, err := build.CreateCustomBuildBundle("docker-build")
	if err != nil {
		return err
	}
	// For running locally
	klog.Infof("To use the custom docker bundle locally, run below cmd:\n\n export CUSTOM_DOCKER_BUILD_PIPELINE_BUNDLE=%s\n\n", customDockerBuildBundle)
	// will be used in CI
	err = os.Setenv(constants.CUSTOM_DOCKER_BUILD_PIPELINE_BUNDLE_ENV, customDockerBuildBundle)
	if err != nil {
		return err
	}
	return nil
}

func SetupMultiPlatformTests() error {
	klog.Infof("going to create new Tekton bundle remote-build for the purpose of testing multi-platform-controller PR")
	var err error
	var defaultBundleRef string
	var tektonObj runtime.Object
	var authenticator authn.Authenticator

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

		if authenticator, err = utils.GetAuthenticatorForImageRef(newRemotePipeline, os.Getenv("QUAY_TOKEN")); err != nil {
			return fmt.Errorf("error when getting authenticator: %v", err)
		}
		authOption := remoteimg.WithAuth(authenticator)

		if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newRemotePipeline, authOption); err != nil {
			return fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
		}
		platform := strings.ToUpper(strings.Split(platformType, "/")[1])
		klog.Infof("SETTING ENV VAR %s to value %s\n", constants.CUSTOM_BUILDAH_REMOTE_PIPELINE_BUILD_BUNDLE_ENV+"_"+platform, newRemotePipeline.String())
		os.Setenv(constants.CUSTOM_BUILDAH_REMOTE_PIPELINE_BUILD_BUNDLE_ENV+"_"+platform, newRemotePipeline.String())
	}

	return nil
}

func SetupBundleForBuildTasksDockerfilesRepo() {
	var err error
	var defaultBundleRef string
	var tektonObj runtime.Object
	var newPipelineYaml []byte
	var sourceImage string
	klog.Info("creating new tekton bundle for the purpose of testing build-task-dockerfiles group PR")

	sourceImage = utils.GetEnv("SOURCE_BUILD_IMAGE", "")
	if sourceImage == "" {
		klog.Error("SOURCE_BUILD_IMAGE env is not set")
		return
	}

	if defaultBundleRef, err = tekton.GetDefaultPipelineBundleRef(constants.BuildPipelineConfigConfigMapYamlURL, "docker-build"); err != nil {
		klog.Errorf("failed to get the pipeline bundle ref: %+v", err)
		return
	}
	if tektonObj, err = tekton.ExtractTektonObjectFromBundle(defaultBundleRef, "pipeline", "docker-build"); err != nil {
		klog.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
		return
	}
	dockerPipelineObject := tektonObj.(*tektonapi.Pipeline)

	// Update build-source-image param value to true
	for i := range dockerPipelineObject.PipelineSpec().Params {
		if dockerPipelineObject.PipelineSpec().Params[i].Name == "build-source-image" {
			dockerPipelineObject.PipelineSpec().Params[i].Default.StringVal = "true"
		}
	}
	// Update the source-build task image reference to SOURCE_BUILD_IMAGE
	var currentSourceTaskBundle string
	for i := range dockerPipelineObject.PipelineSpec().Tasks {
		t := &dockerPipelineObject.PipelineSpec().Tasks[i]
		params := t.TaskRef.Params
		var lastBundle *tektonapi.Param
		sourceTask := false
		for i, param := range params {
			if param.Name == "bundle" {
				lastBundle = &t.TaskRef.Params[i]
			} else if param.Name == "name" && param.Value.StringVal == "source-build" {
				sourceTask = true
			}
		}
		if sourceTask {
			currentSourceTaskBundle = lastBundle.Value.StringVal
			klog.Infof("found current source build task bundle: %s", currentSourceTaskBundle)
			newSourceTaskBundle := createNewTaskBundleAndPush(currentSourceTaskBundle, "source-build", "build", sourceImage)
			klog.Infof("created new source build task bundle: %s", newSourceTaskBundle)
			lastBundle.Value = *tektonapi.NewStructuredValues(newSourceTaskBundle)
			break
		}
	}
	if currentSourceTaskBundle == "" {
		klog.Errorf("failed to extract the Source Build Task from bundle: %+v", err)
		return
	}

	if newPipelineYaml, err = yaml.Marshal(dockerPipelineObject); err != nil {
		klog.Errorf("error when marshalling a new pipeline to YAML: %v", err)
		return
	}
	keychain := authn.NewMultiKeychain(authn.DefaultKeychain)
	authOption := remoteimg.WithAuthFromKeychain(keychain)

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.DEFAULT_QUAY_ORG_ENV, constants.DefaultQuayOrg)
	newSourceBuildPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newSourceBuildPipeline, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newSourceBuildPipelineImg, tag))

	if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newSourceBuildPipeline, authOption); err != nil {
		klog.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
		return
	}
	// This output is consumed by the integration pipeline of build-task-dockerfiles repo, not printing it will break the CI
	fmt.Printf("custom_pipeline_bundle=%s\n", newSourceBuildPipeline.String())
}

func createNewTaskBundleAndPush(currentBuildahTaskBundle, taskName, stepName, stepImage string) string {
	var newTaskYaml []byte
	tektonObj, err := tekton.ExtractTektonObjectFromBundle(currentBuildahTaskBundle, "task", constants.BuildPipelineType(taskName))
	if err != nil {
		klog.Errorf("failed to extract the Tekton Task from bundle: %+v", err)
		return ""
	}
	taskObject := tektonObj.(*tektonapi.Task)
	for i := range taskObject.Spec.Steps {
		if taskObject.Spec.Steps[i].Name == stepName {
			klog.Infof("current %q task in step %q has step image: %q", taskName, stepName, taskObject.Spec.Steps[i].Image)
			taskObject.Spec.Steps[i].Image = stepImage
		}
	}

	if newTaskYaml, err = yaml.Marshal(taskObject); err != nil {
		klog.Errorf("error when marshalling a new pipeline to YAML: %v", err)
		return ""
	}

	keychain := authn.NewMultiKeychain(authn.DefaultKeychain)
	authOption := remoteimg.WithAuthFromKeychain(keychain)

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.DEFAULT_QUAY_ORG_ENV, constants.DefaultQuayOrg)
	newTaskImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newTask, _ = name.ParseReference(fmt.Sprintf("%s:task-bundle-%s", newTaskImg, tag))

	if err = tekton.BuildAndPushTektonBundle(newTaskYaml, newTask, authOption); err != nil {
		klog.Errorf("error when building/pushing a tekton task bundle: %v", err)
		return ""
	}
	return newTask.String()
}

func BootstrapCluster() error {

	if os.Getenv("CI") == "true" || konfluxCI == "true" {
		if err := setRequiredEnvVars(); err != nil {
			return fmt.Errorf("error when setting up required env vars: %v", err)
		}
	}

	ic, err := installation.NewAppStudioInstallController()
	if err != nil {
		return fmt.Errorf("failed to initialize installation controller: %+v", err)
	}

	if err := ic.InstallAppStudioPreviewMode(); err != nil {
		return err
	}

	if os.Getenv("CI") == "true" || konfluxCI == "true" && requiresSprayProxyRegistering {
		err := registerPacServer()
		if err != nil {
			os.Setenv(constants.SKIP_PAC_TESTS_ENV, "true")
			if alertErr := HandleErrorWithAlert(fmt.Errorf("failed to register SprayProxy: %+v", err), slack.ErrorSeverityLevelError); alertErr != nil {
				return alertErr
			}
		}
	}
	return nil
}

func isPRPairingRequired(repoForPairing string) bool {
	var pullRequests []gh.PullRequest

	url := fmt.Sprintf("https://api.github.com/repos/redhat-appstudio/%s/pulls?per_page=100", repoForPairing)
	if err := sendHttpRequestAndParseResponse(url, "GET", &pullRequests); err != nil {
		klog.Infof("cannot determine %s Github branches for author %s: %v. will stick with the redhat-appstudio/%s main branch for running tests", repoForPairing, pr.RemoteName, err, repoForPairing)
		return false
	}

	for _, pull := range pullRequests {
		if pull.GetHead().GetRef() == pr.BranchName && pull.GetUser().GetLogin() == pr.RemoteName {
			return true
		}
	}

	return false
}

// Generates ginkgo test suite files under the cmd/ directory.
func GenerateTestSuiteFile(packageName string) error {

	var templatePath = "templates/test_suite_cmd.tmpl"
	var templatePackageFile = fmt.Sprintf("cmd/%s_test.go", packageName)

	klog.Infof("Creating new test suite file %s.\n", templatePackageFile)
	//var caser = cases.Title(language.English)

	templateData := map[string]string{"SuiteName": packageName}
	//data, _ := json.Marshal(template)
	err := renderTemplate(templatePackageFile, templatePath, templateData, false)

	if err != nil {
		klog.Errorf("failed to render template with: %s", err)
		return err
	}

	err = goFmt(templatePackageFile)
	if err != nil {

		klog.Errorf("%s", err)
		return err
	}

	return nil
}

// Remove all webhooks older than 1 day from GitHub repo.
// By default will delete webhooks from redhat-appstudio-qe
func CleanGitHubWebHooks() error {
	token := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
	if token == "" {
		return fmt.Errorf("empty GITHUB_TOKEN env. Please provide a valid github token")
	}

	githubOrg := utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
	gh, err := github.NewGithubClient(token, githubOrg)
	if err != nil {
		return err
	}
	for _, repo := range repositoriesWithWebhooks {
		webhookList, err := gh.ListRepoWebhooks(repo)
		if err != nil {
			return err
		}
		for _, wh := range webhookList {
			dayDuration, _ := time.ParseDuration("24h")
			if time.Since(wh.GetCreatedAt()) > dayDuration {
				klog.Infof("removing webhook: %s, git_organization: %s, git_repository: %s", wh.GetName(), githubOrg, repo)
				if err := gh.DeleteWebhook(repo, wh.GetID()); err != nil {
					return fmt.Errorf("failed to delete webhook: %v, repo: %s", wh.Name, repo)
				}
			}
		}
	}
	return nil
}

// Remove all webhooks older than 1 day from GitLab repo.
func CleanGitLabWebHooks() error {
	gcToken := utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, "")
	if gcToken == "" {
		return fmt.Errorf("empty GITLAB_BOT_TOKEN env variable")
	}
	gitlabURL := utils.GetEnv(constants.GITLAB_API_URL_ENV, constants.DefaultGitLabAPIURL)
	groupId := utils.GetEnv("GITLAB_GROUP_ID", constants.DefaultGilabGroupId) // default id is for konflux-qe group
	gc, err := gitlab.NewGitlabClient(gcToken, gitlabURL, groupId)
	if err != nil {
		return err
	}
	for projectName, projectID := range constants.GitLabProjectIdsMap {
		webhooks, _, err := gc.GetClient().Projects.ListProjectHooks(projectID, &gl.ListProjectHooksOptions{PerPage: 100})
		if err != nil {
			return fmt.Errorf("failed to list project hooks: %v", err)
		}
		// Delete webhooks that are older than 1 day
		for _, webhook := range webhooks {
			dayDuration, _ := time.ParseDuration("24h")
			if time.Since(*webhook.CreatedAt) > dayDuration {
				klog.Infof("[INFO] from project: %s, removing webhookURL: %s", projectName, webhook.URL)
				if _, err := gc.GetClient().Projects.DeleteProjectHook(projectID, webhook.ID); err != nil {
					return fmt.Errorf("failed to delete webhook (URL: %s): %v", webhook.URL, err)
				}
			}
		}
	}

	return nil
}

// Remove all the repos which matches GITLAB_REPO_REGEX or older than 1 day from GitLab
func CleanupGitLabRepos() error {
	dryRun, err := strconv.ParseBool(utils.GetEnv("DRY_RUN", "true"))
	if err != nil {
		return err
	}
	gcToken := utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, "")
	if gcToken == "" {
		return fmt.Errorf("empty GITLAB_BOT_TOKEN env variable")
	}
	gitlabURL := utils.GetEnv(constants.GITLAB_API_URL_ENV, constants.DefaultGitLabAPIURL)
	groupId := utils.GetEnv("GITLAB_GROUP_ID", constants.DefaultGilabGroupId) // default id is for konflux-qe group
	gc, err := gitlab.NewGitlabClient(gcToken, gitlabURL, groupId)
	if err != nil {
		return err
	}
	projects, err := gc.GetAllProjects()
	if err != nil {
		return err
	}
	// Filter repos by regex
	projectsToBeDeletedRegexp := "^devfile-sample-hello-world-\\S{6}$|^build-nudge-parent-\\S{6}$|^build-nudge-child-\\S{6}$"
	r, err := regexp.Compile(projectsToBeDeletedRegexp)
	if err != nil {
		return fmt.Errorf("unable to compile regex: %s", err)
	}

	projectsToBeDeleted := []string{}
	for _, project := range projects {
		// Add only repos older than 24 hours
		dayDuration, _ := time.ParseDuration("24h")
		if time.Since(*project.CreatedAt) > dayDuration {
			// Add only repos matching the regex
			if r.MatchString(project.Name) {
				projectsToBeDeleted = append(projectsToBeDeleted, project.Name)
			}
		}
	}
	if dryRun {
		klog.Info("Dry run enabled. Listing repositories that would be deleted:")
	}

	// Delete projects
	for _, projectName := range projectsToBeDeleted {
		if dryRun {
			klog.Infof("\t%s", projectName)
		} else {
			err := gc.DeleteRepositoryOnlyIfExists(projectName)
			if err != nil {
				klog.Warningf("error deleting project: %s\n", err)
			}
		}
	}
	if dryRun {
		klog.Info("If you really want to delete these projects, run `DRY_RUN=false ./mage CleanupGitLabRepos`")
	}
	return nil
}

// Generate a Text Outline file from a Ginkgo Spec
func GenerateTextOutlineFromGinkgoSpec(source string, destination string) error {

	gs := testspecs.NewGinkgoSpecTranslator()
	ts := testspecs.NewTextSpecTranslator()

	klog.Infof("Mapping outline from a Ginkgo test file, %s", source)
	outline, err := gs.FromFile(source)

	if err != nil {
		klog.Error("Failed to map Ginkgo test file")
		return err
	}

	klog.Infof("Mapping outline to a text file, %s", destination)
	err = ts.ToFile(destination, outline)
	if err != nil {
		klog.Error("Failed to map text file")
		return err
	}

	return err

}

// Generate a Ginkgo Spec file from a Text Outline file
func GenerateGinkgoSpecFromTextOutline(source string, destination string) error {
	return GenerateTeamSpecificGinkgoSpecFromTextOutline(source, testspecs.TestFilePath, destination)
}

// Generate a team specific file using specs in templates/specs.tmpl file and a provided team specific template
func GenerateTeamSpecificGinkgoSpecFromTextOutline(outlinePath, teamTmplPath, destinationPath string) error {
	gs := testspecs.NewGinkgoSpecTranslator()
	ts := testspecs.NewTextSpecTranslator()

	klog.Infof("Mapping outline from a text file, %s", outlinePath)
	outline, err := ts.FromFile(outlinePath)
	if err != nil {
		klog.Error("Failed to map text outline file")
		return err
	}

	klog.Infof("Mapping outline to a Ginkgo spec file, %s", destinationPath)
	err = gs.ToFile(destinationPath, teamTmplPath, outline)
	if err != nil {
		klog.Error("Failed to map Ginkgo spec file")
		return err
	}

	return err

}

// Print the outline of the Ginkgo spec
func PrintOutlineOfGinkgoSpec(specFile string) error {

	gs := testspecs.NewGinkgoSpecTranslator()
	klog.Infof("Mapping outline from a Ginkgo test file, %s", specFile)
	outline, err := gs.FromFile(specFile)

	if err != nil {
		klog.Errorf("failed to map ginkgo spec to outline: %s", err)
		return err
	}

	klog.Info("Printing outline:")
	fmt.Printf("%s\n", outline.ToString())

	return err

}

// Print the outline of the Text Outline
func PrintOutlineOfTextSpec(specFile string) error {

	ts := testspecs.NewTextSpecTranslator()

	klog.Infof("Mapping outline from a text file, %s", specFile)
	outline, err := ts.FromFile(specFile)
	if err != nil {
		klog.Error("Failed to map text outline file")
		return err
	}

	klog.Info("Printing outline:")
	fmt.Printf("%s\n", outline.ToString())

	return err

}

// Print the outline of the Ginkgo spec in JSON format
func PrintJsonOutlineOfGinkgoSpec(specFile string) error {

	gs := testspecs.NewGinkgoSpecTranslator()
	klog.Infof("Mapping outline from a Ginkgo test file, %s", specFile)
	outline, err := gs.FromFile(specFile)
	if err != nil {
		klog.Errorf("failed to map ginkgo spec to outline: %s", err)
		return err
	}
	data, err := json.Marshal(outline)
	if err != nil {
		println(fmt.Sprintf("error marshalling to json: %s", err))
	}
	fmt.Print(string(data))

	return err

}

// Append to the pkg/framework/describe.go the decorator function for new Ginkgo spec
func AppendFrameworkDescribeGoFile(specFile string) error {

	var node testspecs.TestSpecNode
	klog.Infof("Inspecting Ginkgo spec file, %s", specFile)
	node, err := testspecs.ExtractFrameworkDescribeNode(specFile)
	if err != nil {
		klog.Error("Failed to extract the framework node")
		return err
	}

	if reflect.ValueOf(node).IsZero() {
		klog.Info("Did not find a framework describe decorator function so nothing to append.")
		// we assume its a normal Ginkgo Spec file so that is fine
		return nil
	}
	outline := testspecs.TestOutline{node}
	tmplData := testspecs.NewTemplateData(outline, specFile)
	err = testspecs.RenderFrameworkDescribeGoFile(*tmplData)

	if err != nil {
		klog.Error("Failed to render the framework/describe.go")
		return err
	}

	return err

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

func registerPacServer() error {
	var err error
	var pacHost string
	sprayProxyConfig, err = newSprayProxy()
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
	err = printRegisteredPacServers()
	if err != nil {
		klog.Error(err)
	}
	return nil
}

func unregisterPacServer() error {
	var err error
	var pacHost string
	sprayProxyConfig, err = newSprayProxy()
	if err != nil {
		return fmt.Errorf("failed to set up SprayProxy credentials: %+v", err)
	}
	// for debugging purposes
	klog.Infof("Before unregistering pac server...")
	err = printRegisteredPacServers()
	if err != nil {
		klog.Error(err)
	}

	pacHost, err = sprayproxy.GetPaCHost()
	if err != nil {
		return fmt.Errorf("failed to get PaC host: %+v", err)
	}
	_, err = sprayProxyConfig.UnregisterServer(pacHost)
	if err != nil {
		return fmt.Errorf("error when unregistering PaC server %s from SprayProxy server %s: %+v", pacHost, sprayProxyConfig.BaseURL, err)
	}
	klog.Infof("Unregistered PaC servers: %v", pacHost)
	// for debugging purposes
	klog.Infof("After unregistering server...")
	err = printRegisteredPacServers()
	if err != nil {
		klog.Error(err)
	}
	return nil
}

func printRegisteredPacServers() error {
	servers, err := sprayProxyConfig.GetServers()
	if err != nil {
		return fmt.Errorf("failed to get registered PaC servers from SprayProxy: %+v", err)
	}
	klog.Infof("The PaC servers registered in Sprayproxy: %v", servers)
	return nil
}

// Run upgrade tests in CI
func (ci CI) TestUpgrade() error {
	var testFailure bool

	if err := ci.init(); err != nil {
		return fmt.Errorf("error when running ci init: %v", err)
	}

	if err := PreflightChecks(); err != nil {
		return fmt.Errorf("error when running preflight checks: %v", err)
	}

	if err := setRequiredEnvVars(); err != nil {
		return fmt.Errorf("error when setting up required env vars: %v", err)
	}

	if requiresMultiPlatformTests {
		if err := SetupMultiPlatformTests(); err != nil {
			return err
		}
	}

	if err := UpgradeTestsWorkflow(); err != nil {
		return fmt.Errorf("error when running upgrade tests: %v", err)
	}

	if testFailure {
		return fmt.Errorf("error when running upgrade tests - see the log above for more details")
	}

	return nil
}

// Run upgrade tests locally(bootstrap cluster, create workload, upgrade, verify)
func (Local) TestUpgrade() error {
	if err := PreflightChecks(); err != nil {
		klog.Errorf("error when running preflight checks: %s", err)
		return err
	}

	if err := UpgradeTestsWorkflow(); err != nil {
		klog.Errorf("error when running upgrade tests: %s", err)
		return err
	}

	return nil
}

func UpgradeTestsWorkflow() error {
	ic, err := BootstrapClusterForUpgrade()
	if err != nil {
		klog.Errorf("%s", err)
		return err
	}

	err = CheckClusterAfterUpgrade(ic)
	if err != nil {
		klog.Errorf("%s", err)
		return err
	}

	err = UpgradeCluster()
	if err != nil {
		klog.Errorf("%s", err)
		return err
	}

	err = CheckClusterAfterUpgrade(ic)
	if err != nil {
		klog.Errorf("%s", err)
		return err
	}

	return nil
}

func BootstrapClusterForUpgrade() (*installation.InstallAppStudio, error) {
	//Use main branch of infra-deployments in redhat-appstudio org as default version for upgrade
	os.Setenv("INFRA_DEPLOYMENTS_ORG", "redhat-appstudio")
	os.Setenv("INFRA_DEPLOYMENTS_BRANCH", "main")
	ic, err := installation.NewAppStudioInstallController()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize installation controller: %+v", err)
	}

	return ic, ic.InstallAppStudioPreviewMode()
}

func UpgradeCluster() error {
	return MergePRInRemote(utils.GetEnv("UPGRADE_BRANCH", ""), utils.GetEnv("UPGRADE_FORK_ORGANIZATION", "redhat-appstudio"), "./tmp/infra-deployments")
}

func CheckClusterAfterUpgrade(ic *installation.InstallAppStudio) error {
	return ic.CheckOperatorsReady()
}

func CreateWorkload() error {
	return runTests("upgrade-create", "upgrade-create-report.xml")
}

func VerifyWorkload() error {
	return runTests("upgrade-verify", "upgrade-verify-report.xml")
}

func CleanWorkload() error {
	return runTests("upgrade-cleanup", "upgrade-verify-report.xml")
}

func runTests(labelsToRun string, junitReportFile string) error {
	ginkgoArgs := []string{"-p", "-v", "--output-interceptor-mode=none", "--no-color", "--fail-on-empty",
		"--timeout=90m", "--json-report=e2e-report.json", fmt.Sprintf("--output-dir=%s", artifactDir),
		"--junit-report=" + junitReportFile, "--label-filter=" + labelsToRun}

	if os.Getenv("GINKGO_PROCS") != "" {
		ginkgoArgs = append(ginkgoArgs, fmt.Sprintf("--procs=%s", os.Getenv("GINKGO_PROCS")))
	}

	if os.Getenv("E2E_BIN_PATH") != "" {
		ginkgoArgs = append(ginkgoArgs, os.Getenv("E2E_BIN_PATH"))
	} else {
		ginkgoArgs = append(ginkgoArgs, "./cmd")
	}

	ginkgoArgs = append(ginkgoArgs, "--")

	// added --output-interceptor-mode=none to mitigate RHTAPBUGS-34
	return sh.RunV("ginkgo", ginkgoArgs...)
}

func CleanupRegisteredPacServers() error {
	var err error
	sprayProxyConfig, err = newSprayProxy()
	if err != nil {
		return fmt.Errorf("failed to initialize SprayProxy config: %+v", err)
	}

	servers, err := sprayProxyConfig.GetServers()
	if err != nil {
		return fmt.Errorf("failed to get registered PaC servers from SprayProxy: %+v", err)
	}
	klog.Infof("Before cleaningup Pac servers, the registered PaC servers: %v", servers)

	for _, server := range strings.Split(servers, ",") {
		// Verify if the server is a valid host, if not, unregister it
		if !isValidPacHost(server) {
			_, err := sprayProxyConfig.UnregisterServer(strings.TrimSpace(server))
			if err != nil {
				return fmt.Errorf("error when unregistering PaC server %s from SprayProxy server %s: %+v", server, sprayProxyConfig.BaseURL, err)
			}
			klog.Infof("Cleanup invalid PaC server: %s", server)
		}
	}
	klog.Infof("After cleaningup Pac servers...")
	err = printRegisteredPacServers()
	if err != nil {
		klog.Error(err)
	}
	return nil
}

func isValidPacHost(server string) bool {
	httpClient := http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	_, err := httpClient.Get(strings.TrimSpace(server))
	return err == nil
}

func (Local) PreviewTestSelection() error {

	rctx := rulesengine.NewRuleCtx()
	files, err := utils.GetChangedFiles("e2e-tests")
	if err != nil {
		klog.Error(err)
		return err
	}
	rctx.DiffFiles = files
	rctx.DryRun = true

	err = engine.MageEngine.RunRules(rctx, "tests", "e2e-repo")

	if err != nil {
		return err
	}

	return nil
}

func (Local) RunRuleDemo() error {
	rctx := rulesengine.NewRuleCtx()
	files, err := utils.GetChangedFiles("e2e-tests")
	if err != nil {
		klog.Error(err)
		return err
	}
	rctx.DiffFiles = files
	rctx.DryRun = true

	err = engine.MageEngine.RunRulesOfCategory("demo", rctx)

	if err != nil {
		return err
	}

	return nil
}

func (Local) RunInfraDeploymentsRuleDemo() error {

	rctx := rulesengine.NewRuleCtx()
	rctx.Parallel = true
	rctx.OutputDir = artifactDir

	rctx.RepoName = "infra-deployments"
	rctx.JobName = ""
	rctx.JobType = ""
	rctx.DryRun = true

	files, err := utils.GetChangedFiles("infra-deployments")
	if err != nil {
		return err
	}
	rctx.DiffFiles = files

	// filtering the rule engine to load only infra-deployments rule catalog within the test category
	return engine.MageEngine.RunRules(rctx, "tests", "infra-deployments")
}
