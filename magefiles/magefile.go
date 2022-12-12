package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/magefile/mage/sh"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	"k8s.io/klog/v2"
)

var (
	requiredBinaries = []string{"jq", "kubectl", "oc", "yq", "git"}
	artifactDir      = utils.GetEnv("ARTIFACT_DIR", ".")
	openshiftJobSpec = &OpenshiftJobSpec{}
	pr               = &PullRequestMetadata{}
	jobName          = utils.GetEnv("JOB_NAME", "")
	// can be periodic, presubmit or postsubmit
	jobType = utils.GetEnv("JOB_TYPE", "")
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

	pr.Author = openshiftJobSpec.Refs.Pulls[0].Author
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
		if err := gitCheckoutRemoteBranch(pr.Author, pr.CommitSHA); err != nil {
			return err
		}
	} else {
		if ci.isPRPairingRequired() {
			if err := gitCheckoutRemoteBranch(pr.Author, pr.BranchName); err != nil {
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

	if err := retry(ci.createOpenshiftUser, 3, 10*time.Second); err != nil {
		return fmt.Errorf("error when creating openshift user: %v", err)
	}
	if err := retry(BootstrapCluster, 2, 10*time.Second); err != nil {
		return fmt.Errorf("error when bootstrapping cluster: %v", err)
	}

	if err := RunE2ETests(); err != nil {
		testFailure = true
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
	cwd, _ := os.Getwd()

	return sh.RunV("ginkgo", "-p", "--timeout=90m", fmt.Sprintf("--output-dir=%s", artifactDir), "--junit-report=e2e-report.xml", "--v", "--progress", "--label-filter=$E2E_TEST_SUITE_LABEL", "./cmd", "--", fmt.Sprintf("--config-suites=%s/tests/e2e-demos/config/default.yaml", cwd), "--generate-rppreproc-report=true", fmt.Sprintf("--rp-preproc-dir=%s", artifactDir))
}

func PreflightChecks() error {
	if os.Getenv("GITHUB_TOKEN") == "" || os.Getenv("QUAY_TOKEN") == "" {
		return fmt.Errorf("required env vars containing secrets (QUAY_TOKEN, GITHUB_TOKEN) not defined or empty")
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

func (CI) setRequiredEnvVars() error {

	if strings.Contains(jobName, "hacbs-e2e-periodic") {
		os.Setenv("E2E_TEST_SUITE_LABEL", "HACBS")
		return nil
	} else if strings.Contains(jobName, "appstudio-e2e-deployment-periodic") {
		os.Setenv("E2E_TEST_SUITE_LABEL", "!HACBS")
		return nil
	}

	if openshiftJobSpec.Refs.Repo != "e2e-tests" {

		if strings.Contains(openshiftJobSpec.Refs.Repo, "-service") {
			var envVarPrefix, imageTagSuffix, testSuiteLabel string
			sp := strings.Split(os.Getenv("COMPONENT_IMAGE"), "@")

			switch openshiftJobSpec.Refs.Repo {
			case "application-service":
				envVarPrefix = "HAS"
				imageTagSuffix = "has-image"
				testSuiteLabel = "has"
			case "build-service":
				envVarPrefix = "BUILD_SERVICE"
				imageTagSuffix = "build-service-image"
				testSuiteLabel = "build"
			case "jvm-build-service":
				envVarPrefix = "JVM_BUILD_SERVICE"
				imageTagSuffix = "jvm-build-service-image"
				testSuiteLabel = "jvm-build"
			}

			os.Setenv(fmt.Sprintf("%s_IMAGE_REPO", envVarPrefix), sp[0])
			os.Setenv(fmt.Sprintf("%s_IMAGE_TAG", envVarPrefix), fmt.Sprintf("redhat-appstudio-%s", imageTagSuffix))
			os.Setenv(fmt.Sprintf("%s_PR_OWNER", envVarPrefix), openshiftJobSpec.Refs.Pulls[0].Author)
			os.Setenv(fmt.Sprintf("%s_PR_SHA", envVarPrefix), openshiftJobSpec.Refs.Pulls[0].SHA)
			os.Setenv("E2E_TEST_SUITE_LABEL", testSuiteLabel)

		} else if openshiftJobSpec.Refs.Repo == "infra-deployments" {

			os.Setenv("INFRA_DEPLOYMENTS_ORG", pr.RemoteName)
			os.Setenv("INFRA_DEPLOYMENTS_BRANCH", pr.BranchName)
		}

	}

	return nil
}

func (CI) createOpenshiftUser() error {
	tempKubeconfigPath := "/tmp/kubeconfig"
	os.Setenv("KUBECONFIG_TEST", tempKubeconfigPath)
	if err := sh.Run("./scripts/provision-openshift-user.sh"); err != nil {
		return err
	}
	os.Setenv("KUBECONFIG", tempKubeconfigPath)

	return nil
}

func BootstrapCluster() error {
	envVars := map[string]string{}

	if os.Getenv("CI") == "true" && os.Getenv("REPO_NAME") == "e2e-tests" {
		// Some scripts in infra-deployments repo are referencing scripts/utils in e2e-tests repo
		// This env var allows to test changes introduced in "e2e-tests" repo PRs in CI
		envVars["E2E_TESTS_COMMIT_SHA"] = os.Getenv("PULL_PULL_SHA")
	}

	return sh.RunWith(envVars, "./scripts/install-appstudio.sh")
}

func (CI) isPRPairingRequired() bool {
	var ghBranches []GithubBranch
	url := fmt.Sprintf("https://api.github.com/repos/%s/e2e-tests/branches", pr.RemoteName)
	if err := sendHttpRequestAndParseResponse(url, "GET", &ghBranches); err != nil {
		klog.Infof("cannot determine e2e-tests Github branches for author %s: %v. will stick with the redhat-appstudio/e2e-tests main branch for running tests", pr.Author, err)
		return false
	}

	for _, b := range ghBranches {
		if b.Name == pr.BranchName {
			return true
		}
	}

	return false
}

func (CI) sendWebhook() error {
	// AppStudio QE webhook configuration values will be used by default (if none are provided via env vars)
	const appstudioQESaltSecret = "123456789"
	const appstudioQEWebhookTargetURL = "https://smee.io/JgVqn2oYFPY1CF"

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

// Generates test cases for Polarion(polarion.xml) from test files for AppStudio project.
func GenerateTestCasesAppStudio() error {
	return sh.RunV("ginkgo", "--v", "--dry-run", "--label-filter=$E2E_TEST_SUITE_LABEL", "./cmd", "--", "--polarion-output-file=polarion.xml", "--generate-test-cases=true")
}

// I've attached to the Local struct for now since it felt like it fit but it can be decoupled later as a standalone func.
func (Local) GenerateTestSuiteFile() error {

	var templatePackageName = utils.GetEnv("TEMPLATE_SUITE_PACKAGE_NAME", "")
	var templatePath = "templates/test_suite_cmd.tmpl"
	var err error

	if !utils.CheckIfEnvironmentExists("TEMPLATE_SUITE_PACKAGE_NAME") {
		klog.Exitf("TEMPLATE_SUITE_PACKAGE_NAME not set or is empty")
	}

	var templatePackageFile = fmt.Sprintf("cmd/%s_test.go", templatePackageName)
	klog.Infof("Creating new test suite file %s.\n", templatePackageFile)
	data := TemplateData{SuiteName: templatePackageName}
	err = renderTemplate(templatePackageFile, templatePath, data, false)

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

// I've attached to the Local struct for now since it felt like it fit but it can be decoupled later as a standalone func.
func (Local) GenerateTestSpecFile() error {

	var templateDirName = utils.GetEnv("TEMPLATE_SUITE_PACKAGE_NAME", "")
	var templateSpecName = utils.GetEnv("TEMPLATE_SPEC_FILE_NAME", templateDirName)
	var templateAppendFrameworkDescribeBool = utils.GetEnv("TEMPLATE_APPEND_FRAMEWORK_DESCRIBE_FILE", "true")
	var templateJoinSuiteSpecNamesBool = utils.GetEnv("TEMPLATE_JOIN_SUITE_SPEC_FILE_NAMES", "false")
	var templatePath = "templates/test_spec_file.tmpl"
	var templateDirPath string
	var templateSpecFile string
	var err error
	var caser = cases.Title(language.English)

	if !utils.CheckIfEnvironmentExists("TEMPLATE_SUITE_PACKAGE_NAME") {
		klog.Exitf("TEMPLATE_SUITE_PACKAGE_NAME not set or is empty")
	}

	if !utils.CheckIfEnvironmentExists("TEMPLATE_SPEC_FILE_NAME") {
		klog.Infof("TEMPLATE_SPEC_FILE_NAME not set. Defaulting test spec file to value of `%s`.\n", templateSpecName)
	}

	if !utils.CheckIfEnvironmentExists("TEMPLATE_APPEND_FRAMEWORK_DESCRIBE_FILE") {
		klog.Infof("TEMPLATE_APPEND_FRAMEWORK_DESCRIBE_FILE not set. Defaulting to `%s` which will update the pkg/framework/describe.go after generating the template.\n", templateAppendFrameworkDescribeBool)
	}

	if utils.CheckIfEnvironmentExists("TEMPLATE_JOIN_SUITE_SPEC_FILE_NAMES") {
		klog.Infof("TEMPLATE_JOIN_SUITE_SPEC_FILE_NAMES is set to %s. Will join the suite package and spec file name to be used in the Describe of suites.\n", templateJoinSuiteSpecNamesBool)
	}

	templateDirPath = fmt.Sprintf("tests/%s", templateDirName)
	err = os.Mkdir(templateDirPath, 0775)
	if err != nil {
		klog.Errorf("failed to create package directory, %s, template with: %v", templateDirPath, err)
		return err
	}
	templateSpecFile = fmt.Sprintf("%s/%s.go", templateDirPath, templateSpecName)

	klog.Infof("Creating new test package directory and spec file %s.\n", templateSpecFile)
	if templateJoinSuiteSpecNamesBool == "true" {
		templateSpecName = fmt.Sprintf("%s%v", caser.String(templateDirName), caser.String(templateSpecName))
	} else {
		templateSpecName = caser.String(templateSpecName)
	}

	data := TemplateData{SuiteName: templateDirName,
		TestSpecName: templateSpecName}
	err = renderTemplate(templateSpecFile, templatePath, data, false)
	if err != nil {
		klog.Errorf("failed to render template with: %s", err)
		return err
	}

	err = goFmt(templateSpecFile)
	if err != nil {

		klog.Errorf("%s", err)
		return err
	}

	if templateAppendFrameworkDescribeBool == "true" {

		err = appendFrameworkDescribeFile(templateSpecName)

		if err != nil {
			return err
		}

	}

	return nil

}

func appendFrameworkDescribeFile(packageName string) error {

	var templatePath = "templates/framework_describe_func.tmpl"
	var describeFile = "pkg/framework/describe.go"
	var err error
	var caser = cases.Title(language.English)

	data := TemplateData{TestSpecName: caser.String(packageName)}
	err = renderTemplate(describeFile, templatePath, data, true)
	if err != nil {
		klog.Errorf("failed to append to pkg/framework/describe.go with : %s", err)
		return err
	}
	err = goFmt(describeFile)

	if err != nil {

		klog.Errorf("%s", err)
		return err
	}

	return nil

}
