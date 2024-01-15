package main

import (
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
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/klog/v2"

	"sigs.k8s.io/yaml"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	gh "github.com/google/go-github/v44/github"
	"github.com/magefile/mage/sh"
	"github.com/redhat-appstudio/e2e-tests/magefiles/installation"
	"github.com/redhat-appstudio/e2e-tests/magefiles/testspecs"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/github"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/slack"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/sprayproxy"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/redhat-appstudio/image-controller/pkg/quay"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const (
	quayApiUrl       = "https://quay.io/api/v1"
	gitopsRepository = "GitOps Repository"
)

var (
	requiredBinaries = []string{"jq", "kubectl", "oc", "yq", "git", "helm"}
	artifactDir      = utils.GetEnv("ARTIFACT_DIR", ".")
	openshiftJobSpec = &OpenshiftJobSpec{}
	pr               = &PullRequestMetadata{}
	jobName          = utils.GetEnv("JOB_NAME", "")
	// can be periodic, presubmit or postsubmit
	jobType                    = utils.GetEnv("JOB_TYPE", "")
	reposToDeleteDefaultRegexp = "jvm-build|e2e-dotnet|build-suite|e2e|pet-clinic-e2e|test-app|e2e-quayio|petclinic|test-app|integ-app|^dockerfile-|new-|^python|my-app|^test-|^multi-component"
	repositoriesWithWebhooks   = []string{"devfile-sample-hello-world", "hacbs-test-project"}
	// determine whether CI will run tests that require to register SprayProxy
	// in order to run tests that require PaC application
	requiresSprayProxyRegistering bool

	requiresMultiPlatformTests bool

	sprayProxyConfig       *sprayproxy.SprayProxyConfig
	quayTokenNotFoundError = "DEFAULT_QUAY_ORG_TOKEN env var was not found"
)

func (CI) parseJobSpec() error {
	jobSpecEnvVarData := os.Getenv("JOB_SPEC")

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

	pr.Organization = openshiftJobSpec.Refs.Organization
	pr.RepoName = openshiftJobSpec.Refs.Repo
	pr.CommitSHA = openshiftJobSpec.Refs.Pulls[0].SHA
	pr.Number = openshiftJobSpec.Refs.Pulls[0].Number

	prUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", pr.Organization, pr.RepoName, pr.Number)
	pr.RemoteName, pr.BranchName, err = getRemoteAndBranchNameFromPRLink(prUrl)
	if err != nil {
		return err
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

	if openshiftJobSpec.Refs.Repo == "e2e-tests" {
		if err := gitCheckoutRemoteBranch(pr.RemoteName, pr.CommitSHA); err != nil {
			return err
		}
	} else {
		if ci.isPRPairingRequired("e2e-tests") {
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
	return nil
}

func (Local) TestE2E() error {
	return RunE2ETests()
}

// Deletes autogenerated repositories from redhat-appstudio-qe Github org.
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
			if r.MatchString(*repo.Name) || *repo.Description == gitopsRepository {
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
		return fmt.Errorf(quayTokenNotFoundError)
	}
	quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "redhat-appstudio-qe")

	quayClient := quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayOrgToken, quayApiUrl)
	return cleanupQuayReposAndRobots(quayClient, quayOrg)
}

// Deletes Quay Tags older than 7 days in `test-images` repository
func (Local) CleanupQuayTags() error {
	quayOrgToken := os.Getenv("DEFAULT_QUAY_ORG_TOKEN")
	if quayOrgToken == "" {
		return fmt.Errorf(quayTokenNotFoundError)
	}
	quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "redhat-appstudio-qe")

	quayClient := quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayOrgToken, quayApiUrl)
	return cleanupQuayTags(quayClient, quayOrg, "test-images")
}

// Deletes the private repos with prefix "build-e2e" or "rhtap-demo"
func (Local) CleanupPrivateRepos() error {
	repoNamePrefixes := []string{"build-e2e", "rhtap-demo"}
	quayOrgToken := os.Getenv("DEFAULT_QUAY_ORG_TOKEN")
	if quayOrgToken == "" {
		return fmt.Errorf(quayTokenNotFoundError)
	}
	quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "redhat-appstudio-qe")

	quayClient := quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayOrgToken, quayApiUrl)
	return cleanupPrivateRepos(quayClient, quayOrg, repoNamePrefixes)
}

func (ci CI) TestE2E() error {
	var testFailure bool

	if err := ci.init(); err != nil {
		return fmt.Errorf("error when running ci init: %v", err)
	}

	if err := PreflightChecks(); err != nil {
		return fmt.Errorf("error when running preflight checks: %v", err)
	}

	if err := ci.setRequiredEnvVars(); err != nil {
		return fmt.Errorf("error when setting up required env vars: %v", err)
	}

	if err := retry(BootstrapCluster, 2, 10*time.Second); err != nil {
		return fmt.Errorf("error when bootstrapping cluster: %v", err)
	}

	if requiresMultiPlatformTests {
		if err := setupMultiPlatformTests(); err != nil {
			return err
		}
	}

	if requiresSprayProxyRegistering {
		err := registerPacServer()
		if err != nil {
			os.Setenv(constants.SKIP_PAC_TESTS_ENV, "true")
			if alertErr := HandleErrorWithAlert(fmt.Errorf("failed to register SprayProxy: %+v", err), slack.ErrorSeverityLevelError); alertErr != nil {
				return alertErr
			}
		}
	}

	if err := RunE2ETests(); err != nil {
		testFailure = true
	}

	if requiresSprayProxyRegistering && sprayProxyConfig != nil {
		err := unregisterPacServer()
		if err != nil {
			if alertErr := HandleErrorWithAlert(fmt.Errorf("failed to unregister SprayProxy: %+v", err), slack.ErrorSeverityLevelInfo); alertErr != nil {
				klog.Warning(alertErr)
			}
		}
	}

	if err := ci.sendWebhook(); err != nil {
		klog.Infof("error when sending webhook: %v", err)
	}

	if testFailure {
		return fmt.Errorf("error when running e2e tests - see the log above for more details")
	}

	return nil
}

func RunE2ETests() error {
	labelFilter := utils.GetEnv("E2E_TEST_SUITE_LABEL", "!upgrade-create && !upgrade-verify && !upgrade-cleanup && !release-pipelines && !verify-stage")
	return runTests(labelFilter, "e2e-report.xml")
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

func (ci CI) setRequiredEnvVars() error {

	// RHTAP Nightly E2E job
	// The job name is taken from https://github.com/openshift/release/blob/f03153fa4ad36c0e10050d977e7f0f7619d2163a/ci-operator/config/redhat-appstudio/infra-deployments/redhat-appstudio-infra-deployments-main.yaml#L59C7-L59C35
	if strings.Contains(jobName, "appstudio-e2e-tests-periodic") {
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
				testSuiteLabel = "e2e-demo,byoc"
			case strings.Contains(jobName, "release-service"):
				envVarPrefix = "RELEASE_SERVICE"
				imageTagSuffix = "release-service-image"
				testSuiteLabel = "release-service"
			case strings.Contains(jobName, "integration-service"):
				requiresSprayProxyRegistering = true
				envVarPrefix = "INTEGRATION_SERVICE"
				imageTagSuffix = "integration-service-image"
				testSuiteLabel = "integration-service"
			case strings.Contains(jobName, "jvm-build-service"):
				envVarPrefix = "JVM_BUILD_SERVICE"
				imageTagSuffix = "jvm-build-service-image"
				testSuiteLabel = "jvm-build"
				// Since CI requires to have default values for dependency images
				// (https://github.com/openshift/release/blob/master/ci-operator/step-registry/redhat-appstudio/e2e/redhat-appstudio-e2e-ref.yaml#L15)
				// we cannot let these env vars to have identical names in CI as those env vars used in tests
				// e.g. JVM_BUILD_SERVICE_REQPROCESSOR_IMAGE, otherwise those images they are referencing wouldn't
				// be always relevant for tests and tests would be failing
				os.Setenv(fmt.Sprintf("%s_REQPROCESSOR_IMAGE", envVarPrefix), os.Getenv("CI_JBS_REQPROCESSOR_IMAGE"))
				os.Setenv(fmt.Sprintf("%s_CACHE_IMAGE", envVarPrefix), os.Getenv("CI_JBS_CACHE_IMAGE"))

				klog.Infof("going to override default Tekton bundle s2i-java task for the purpose of testing jvm-build-service PR")
				var err error
				var defaultBundleRef string
				var tektonObj runtime.Object

				tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
				quayOrg := utils.GetEnv(constants.DEFAULT_QUAY_ORG_ENV, constants.DefaultQuayOrg)
				newS2iJavaTaskImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
				var newS2iJavaTaskRef, _ = name.ParseReference(fmt.Sprintf("%s:task-bundle-%s", newS2iJavaTaskImg, tag))
				newJavaBuilderPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
				var newJavaBuilderPipelineRef, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newJavaBuilderPipelineImg, tag))
				var newReqprocessorImage = os.Getenv("JVM_BUILD_SERVICE_REQPROCESSOR_IMAGE")
				var newTaskYaml, newPipelineYaml []byte

				if err = utils.CreateDockerConfigFile(os.Getenv("QUAY_TOKEN")); err != nil {
					return fmt.Errorf("failed to create docker config file: %+v", err)
				}
				if defaultBundleRef, err = tekton.GetDefaultPipelineBundleRef(constants.BuildPipelineSelectorYamlURL, "Java"); err != nil {
					return fmt.Errorf("failed to get the pipeline bundle ref: %+v", err)
				}
				if tektonObj, err = tekton.ExtractTektonObjectFromBundle(defaultBundleRef, "pipeline", "java-builder"); err != nil {
					return fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
				}
				javaPipelineObj := tektonObj.(*tektonapi.Pipeline)

				var currentS2iJavaTaskRef string
				for _, t := range javaPipelineObj.PipelineSpec().Tasks {
					params := t.TaskRef.Params
					var lastBundle *tektonapi.Param
					s2iTask := false
					for i, param := range params {
						if param.Name == "bundle" {
							lastBundle = &t.TaskRef.Params[i]
						} else if param.Name == "name" && param.Value.StringVal == "s2i-java" {
							s2iTask = true
						}
					}
					if s2iTask {
						currentS2iJavaTaskRef = lastBundle.Value.StringVal
						klog.Infof("Found current task ref %s", currentS2iJavaTaskRef)
						lastBundle.Value = *tektonapi.NewStructuredValues(newS2iJavaTaskRef.String())
						break
					}
				}
				if tektonObj, err = tekton.ExtractTektonObjectFromBundle(currentS2iJavaTaskRef, "task", "s2i-java"); err != nil {
					return fmt.Errorf("failed to extract the Tekton Task from bundle: %+v", err)
				}
				taskObj := tektonObj.(*tektonapi.Task)

				for i, s := range taskObj.Spec.Steps {
					if s.Name == "analyse-dependencies-java-sbom" {
						taskObj.Spec.Steps[i].Image = newReqprocessorImage
					}
				}

				if newTaskYaml, err = yaml.Marshal(taskObj); err != nil {
					return fmt.Errorf("error when marshalling a new task to YAML: %v", err)
				}
				if newPipelineYaml, err = yaml.Marshal(javaPipelineObj); err != nil {
					return fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
				}

				keychain := authn.NewMultiKeychain(authn.DefaultKeychain)
				authOption := remoteimg.WithAuthFromKeychain(keychain)

				if err = tekton.BuildAndPushTektonBundle(newTaskYaml, newS2iJavaTaskRef, authOption); err != nil {
					return fmt.Errorf("error when building/pushing a tekton task bundle: %v", err)
				}
				if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newJavaBuilderPipelineRef, authOption); err != nil {
					return fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
				}
				os.Setenv(constants.CUSTOM_JAVA_PIPELINE_BUILD_BUNDLE_ENV, newJavaBuilderPipelineRef.String())
			case strings.Contains(jobName, "build-service"):
				requiresSprayProxyRegistering = true
				envVarPrefix = "BUILD_SERVICE"
				imageTagSuffix = "build-service-image"
				testSuiteLabel = "build"
			case strings.Contains(jobName, "image-controller"):
				requiresSprayProxyRegistering = true
				envVarPrefix = "IMAGE_CONTROLLER"
				imageTagSuffix = "image-controller-image"
				testSuiteLabel = "image-controller"
			case strings.Contains(jobName, "remote-secret-service"):
				envVarPrefix = "REMOTE_SECRET"
				imageTagSuffix = "remote-secret-image"
				testSuiteLabel = "remote-secret"
			case strings.Contains(jobName, "spi-service"):
				envVarPrefix = "SPI_OPERATOR"
				imageTagSuffix = "spi-image"
				testSuiteLabel = "spi-suite"
				// spi also requires service-provider-integration-oauth image
				im := strings.Split(os.Getenv("CI_SPI_OAUTH_IMAGE"), "@")
				os.Setenv("SPI_OAUTH_IMAGE_REPO", im[0])
				os.Setenv("SPI_OAUTH_IMAGE_TAG", fmt.Sprintf("redhat-appstudio-%s", "spi-oauth-image"))
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
			os.Setenv("E2E_TEST_SUITE_LABEL", "e2e-demo,rhtap-demo,spi-suite,remote-secret,integration-service,ec,byoc,build-templates,multi-platform")
		} else if strings.Contains(jobName, "release-service-catalog") { // release-service-catalog jobs (pull, rehearsal)
			envVarPrefix := "RELEASE_SERVICE"
			// "rehearse" jobs metadata are not relevant for testing
			if !strings.Contains(jobName, "rehearse") {
				os.Setenv(fmt.Sprintf("%s_CATALOG_URL", envVarPrefix), fmt.Sprintf("https://github.com/%s/%s", pr.RemoteName, pr.RepoName))
				os.Setenv(fmt.Sprintf("%s_CATALOG_REVISION", envVarPrefix), pr.CommitSHA)
			}
			if os.Getenv("REL_IMAGE_CONTROLLER_QUAY_ORG") != "" {
				os.Setenv("IMAGE_CONTROLLER_QUAY_ORG", os.Getenv("REL_IMAGE_CONTROLLER_QUAY_ORG"))
			}
			if os.Getenv("REL_IMAGE_CONTROLLER_QUAY_TOKEN") != "" {
				os.Setenv("IMAGE_CONTROLLER_QUAY_TOKEN", os.Getenv("REL_IMAGE_CONTROLLER_QUAY_TOKEN"))
			}
			os.Setenv("E2E_TEST_SUITE_LABEL", "release-pipelines")
		} else { // openshift/release rehearse job for e2e-tests/infra-deployments repos
			requiresMultiPlatformTests = true
			requiresSprayProxyRegistering = true
		}
	} else { // e2e-tests repository PR
		requiresMultiPlatformTests = true
		requiresSprayProxyRegistering = true
		if ci.isPRPairingRequired("infra-deployments") {
			os.Setenv("INFRA_DEPLOYMENTS_ORG", pr.RemoteName)
			os.Setenv("INFRA_DEPLOYMENTS_BRANCH", pr.BranchName)
		}
	}

	return nil
}

func setupMultiPlatformTests() error {
	klog.Infof("going to create new Tekton bundle remote-build for the purpose of testing multi-platform-controller PR")
	var err error
	var defaultBundleRef string
	var tektonObj runtime.Object

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.DEFAULT_QUAY_ORG_ENV, constants.DefaultQuayOrg)
	newMultiPlatformBuilderPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)
	var newRemotePipeline, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newMultiPlatformBuilderPipelineImg, tag))
	var newPipelineYaml []byte

	if err = utils.CreateDockerConfigFile(os.Getenv("QUAY_TOKEN")); err != nil {
		return fmt.Errorf("failed to create docker config file: %+v", err)
	}
	if defaultBundleRef, err = tekton.GetDefaultPipelineBundleRef(constants.BuildPipelineSelectorYamlURL, "Docker build"); err != nil {
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
			lastBundle.Value = *tektonapi.NewStructuredValues("quay.io/redhat-appstudio-tekton-catalog/task-buildah-remote:0.1")
			lastName.Value = *tektonapi.NewStructuredValues("buildah-remote")
			t.Params = append(t.Params, tektonapi.Param{Name: "PLATFORM", Value: *tektonapi.NewStructuredValues("$(params.PLATFORM)")})
			dockerPipelineObject.Spec.Params = append(dockerPipelineObject.PipelineSpec().Params, tektonapi.ParamSpec{Name: "PLATFORM", Default: tektonapi.NewStructuredValues("linux/arm64")})
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
	os.Setenv(constants.CUSTOM_BUILDAH_REMOTE_PIPELINE_BUILD_BUNDLE_ENV, newRemotePipeline.String())
	return nil
}

func BootstrapCluster() error {
	envVars := map[string]string{}

	if os.Getenv("CI") == "true" && os.Getenv("REPO_NAME") == "e2e-tests" {
		// Some scripts in infra-deployments repo are referencing scripts/utils in e2e-tests repo
		// This env var allows to test changes introduced in "e2e-tests" repo PRs in CI
		envVars["E2E_TESTS_COMMIT_SHA"] = os.Getenv("PULL_PULL_SHA")
	}

	ic, err := installation.NewAppStudioInstallController()
	if err != nil {
		return fmt.Errorf("failed to initialize installation controller: %+v", err)
	}

	return ic.InstallAppStudioPreviewMode()
}

func (CI) isPRPairingRequired(repoForPairing string) bool {
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

func (CI) sendWebhook() error {
	// AppStudio QE webhook configuration values will be used by default (if none are provided via env vars)
	const appstudioQESaltSecret = "123456789"
	const appstudioQEWebhookTargetURL = "https://hook.pipelinesascode.com/EyFYTakxEgEy"

	var repoURL string

	var repoOwner = os.Getenv("REPO_OWNER")
	var repoName = os.Getenv("REPO_NAME")
	var prNumber = os.Getenv("PULL_NUMBER")
	var saltSecret = utils.GetEnv("WEBHOOK_SALT_SECRET", appstudioQESaltSecret)
	var webhookTargetURL = utils.GetEnv("WEBHOOK_TARGET_URL", appstudioQEWebhookTargetURL)

	if strings.Contains(jobName, "hacbs-e2e-periodic") {
		// TODO configure webhook channel for sending HACBS test results
		klog.Infof("not sending webhook for HACBS periodic job yet")
		return nil
	}

	if jobType == "periodic" {
		repoURL = "https://github.com/redhat-appstudio/infra-deployments"
		repoOwner = "redhat-appstudio"
		repoName = "infra-deployments"
		prNumber = "periodic"
	} else if repoName == "e2e-tests" || repoName == "infra-deployments" {
		repoURL = openshiftJobSpec.Refs.RepoLink
	} else {
		klog.Infof("sending webhook for jobType %s, jobName %s is not supported", jobType, jobName)
		return nil
	}

	path, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error when sending webhook: %+v", err)
	}

	wh := Webhook{
		Path: path,
		Repository: Repository{
			FullName:   fmt.Sprintf("%s/%s", repoOwner, repoName),
			PullNumber: prNumber,
		},
		RepositoryURL: repoURL,
	}
	resp, err := wh.CreateAndSend(saltSecret, webhookTargetURL)
	if err != nil {
		return fmt.Errorf("error sending webhook: %+v", err)
	}
	klog.Infof("webhook response: %+v", resp)

	return nil
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

// Remove all webhooks which with 1 day lifetime. By default will delete webooks from redhat-appstudio-qe
func CleanWebHooks() error {
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
func GenerateGinkoSpecFromTextOutline(source string, destination string) error {

	gs := testspecs.NewGinkgoSpecTranslator()
	ts := testspecs.NewTextSpecTranslator()

	klog.Infof("Mapping outline from a text file, %s", source)
	outline, err := ts.FromFile(source)
	if err != nil {
		klog.Error("Failed to map text outline file")
		return err
	}

	klog.Infof("Mapping outline to a Ginkgo spec file, %s", destination)
	err = gs.ToFile(destination, outline)
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
	if sprayProxyConfig == nil {
		return fmt.Errorf("SprayProxy config is empty")
	}
	// for debugging purposes
	klog.Infof("Before unregistering pac server...")
	err := printRegisteredPacServers()
	if err != nil {
		klog.Error(err)
	}

	pacHost, err := sprayproxy.GetPaCHost()
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

	if err := ci.setRequiredEnvVars(); err != nil {
		return fmt.Errorf("error when setting up required env vars: %v", err)
	}

	if requiresMultiPlatformTests {
		if err := setupMultiPlatformTests(); err != nil {
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

	err = CreateWorkload()
	if err != nil {
		klog.Errorf("%s", err)
		return err
	}

	err = VerifyWorkload()
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

	err = VerifyWorkload()
	if err != nil {
		klog.Errorf("%s", err)
		return err
	}

	err = CleanWorkload()
	if err != nil {
		klog.Errorf("%s", err)
		return err
	}

	return nil
}

func BootstrapClusterForUpgrade() (*installation.InstallAppStudio, error) {
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
	// added --output-interceptor-mode=none to mitigate RHTAPBUGS-34
	return sh.RunV("ginkgo", "-p", "--output-interceptor-mode=none", "--timeout=90m", fmt.Sprintf("--output-dir=%s", artifactDir), "--junit-report="+junitReportFile, "--label-filter="+labelsToRun, "./cmd", "--", "--generate-rppreproc-report=true", fmt.Sprintf("--rp-preproc-dir=%s", artifactDir))
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
	_, err := http.Get(strings.TrimSpace(server))
	return err == nil
}
