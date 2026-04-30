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

	"k8s.io/klog"

	gh "github.com/google/go-github/v66/github"
	"github.com/konflux-ci/e2e-tests/magefiles/installation"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine"
	"github.com/konflux-ci/e2e-tests/magefiles/rulesengine/engine"
	"github.com/konflux-ci/e2e-tests/magefiles/upgrade"
	forgejoClient "github.com/konflux-ci/e2e-tests/pkg/clients/forgejo"
	"github.com/konflux-ci/e2e-tests/pkg/clients/github"
	"github.com/konflux-ci/e2e-tests/pkg/clients/gitlab"
	"github.com/konflux-ci/e2e-tests/pkg/clients/slack"
	"github.com/konflux-ci/e2e-tests/pkg/clients/sprayproxy"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/testspecs"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/image-controller/pkg/quay"
	"github.com/magefile/mage/sh"
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

	if pr.RepoName == "e2e-tests" {
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
	case "infra-deployments":
		return engine.MageEngine.RunRules(rctx, "tests", "infra-deployments")
	default:
		labelFilter := utils.GetEnv("E2E_TEST_SUITE_LABEL", "!upgrade-create && !upgrade-verify && !upgrade-cleanup")
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
	// Konflux Nightly E2E job
	if strings.Contains(jobName, "-periodic") {
		requiresSprayProxyRegistering = true
		return nil
	}

	if openshiftJobSpec.Refs.Repo != "e2e-tests" {

		if strings.HasSuffix(jobName, "-service-e2e") {
			var envVarPrefix, imageTagSuffix, testSuiteLabel string
			sp := strings.Split(os.Getenv("COMPONENT_IMAGE"), "@")

			switch {
			case strings.Contains(jobName, "application-service"):
				requiresSprayProxyRegistering = true
				envVarPrefix = "HAS"
				imageTagSuffix = "has-image"
				testSuiteLabel = "konflux"
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
			requiresSprayProxyRegistering = true
			os.Setenv("INFRA_DEPLOYMENTS_ORG", pr.RemoteName)
			os.Setenv("INFRA_DEPLOYMENTS_BRANCH", pr.BranchName)
			os.Setenv("E2E_TEST_SUITE_LABEL", "konflux,ec")
		} else { // openshift/release rehearse job for e2e-tests/infra-deployments repos
			requiresSprayProxyRegistering = true
		}
	}

	return nil
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
			if time.Since(wh.GetCreatedAt().Time) > dayDuration {
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

// Remove all the repos which matches FORGEJO_REPO_REGEX or older than 1 day from Forgejo/Codeberg
func CleanupForgejoRepos() error {
	dryRun, err := strconv.ParseBool(utils.GetEnv("DRY_RUN", "true"))
	if err != nil {
		return err
	}
	token := utils.GetEnv(constants.CODEBERG_BOT_TOKEN_ENV, "")
	if token == "" {
		return fmt.Errorf("empty CODEBERG_BOT_TOKEN env variable")
	}
	apiURL := utils.GetEnv(constants.CODEBERG_API_URL_ENV, constants.DefaultCodebergAPIURL)
	org := utils.GetEnv(constants.CODEBERG_QE_ORG_ENV, constants.DefaultCodebergQEOrg)
	fc, err := forgejoClient.NewForgejoClient(token, apiURL, org)
	if err != nil {
		return err
	}
	repos, err := fc.GetAllRepositories()
	if err != nil {
		return err
	}
	// Filter repos by regex
	reposToBeDeletedRegexp := utils.GetEnv("FORGEJO_REPO_REGEX", "^devfile-sample-hello-world-\\S{6}$|^build-nudge-parent-\\S{6}$|^build-nudge-child-\\S{6}$|^konflux-test-integration-\\S{6}$")
	r, err := regexp.Compile(reposToBeDeletedRegexp)
	if err != nil {
		return fmt.Errorf("unable to compile regex: %s", err)
	}

	reposToBeDeleted := []string{}
	for _, repo := range repos {
		dayDuration, _ := time.ParseDuration("24h")
		if time.Since(repo.Created) > dayDuration {
			if r.MatchString(repo.Name) {
				reposToBeDeleted = append(reposToBeDeleted, repo.Name)
			}
		}
	}
	if dryRun {
		klog.Info("Dry run enabled. Listing repositories that would be deleted:")
	}

	for _, repoName := range reposToBeDeleted {
		if dryRun {
			klog.Infof("\t%s", repoName)
		} else {
			err := fc.DeleteRepositoryIfExists(org + "/" + repoName)
			if err != nil {
				klog.Warningf("error deleting repository: %s\n", err)
			}
		}
	}
	if dryRun {
		klog.Info("If you really want to delete these repositories, run `DRY_RUN=false ./mage CleanupForgejoRepos`")
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

	if err := UpgradeTestsWorkflow(); err != nil {
		return fmt.Errorf("error when running upgrade tests: %v", err)
	}

	if testFailure {
		return fmt.Errorf("error when running upgrade tests - see the log above for more details")
	}

	return nil
}

// TestDisasterRecovery runs the DR backup/restore e2e suite in CI.
func (ci CI) TestDisasterRecovery() error {
	if err := ci.init(); err != nil {
		return fmt.Errorf("error when running ci init: %v", err)
	}

	if err := PreflightChecks(); err != nil {
		return fmt.Errorf("error when running preflight checks: %v", err)
	}

	if err := setRequiredEnvVars(); err != nil {
		return fmt.Errorf("error when setting up required env vars: %v", err)
	}

	return DisasterRecoveryWorkflow()
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

func BootstrapClusterForDR(version string) (*installation.InstallAppStudio, error) {
	os.Setenv("INFRA_DEPLOYMENTS_ORG", "redhat-appstudio") // #nosec G104
	os.Setenv("INFRA_DEPLOYMENTS_BRANCH", version)         // #nosec G104
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

// DisasterRecoveryWorkflow orchestrates the DR backup/restore e2e test flow.
// It bootstraps a Konflux cluster at the version specified by KONFLUX_BASE_VERSION
// (defaults to "main" when unset), runs the disaster-recovery labelled tests which
// cover both backwards-compatibility (with mid-test upgrade) and same-version
// scenarios on a single ROSA cluster in sequence.
func DisasterRecoveryWorkflow() error {
	version := os.Getenv("KONFLUX_BASE_VERSION")
	if version == "" {
		version = "main"
	}

	ic, err := BootstrapClusterForDR(version)
	if err != nil {
		klog.Errorf("failed to bootstrap cluster for disaster recovery tests: %s", err)
		return err
	}

	err = CheckClusterAfterUpgrade(ic)
	if err != nil {
		klog.Errorf("cluster not ready after bootstrap: %s", err)
		return err
	}

	return runTestsWithTimeout("disaster-recovery", "disaster-recovery-report.xml", "1080m")
}

func runTests(labelsToRun string, junitReportFile string) error {
	return runTestsWithTimeout(labelsToRun, junitReportFile, "90m")
}

// runTestsWithTimeout is like runTests but accepts a custom ginkgo timeout.
// Use this for test suites that need longer than the default 90 minutes.
func runTestsWithTimeout(labelsToRun, junitReportFile, timeout string) error {
	extraFilter := strings.TrimSpace(os.Getenv("E2E_EXTRA_LABEL_FILTER"))
	if extraFilter != "" {
		if strings.Contains(extraFilter, "||") {
			labelsToRun = fmt.Sprintf("(%s) && (%s)", labelsToRun, extraFilter)
		} else {
			labelsToRun = fmt.Sprintf("(%s) && %s", labelsToRun, extraFilter)
		}
		klog.Infof("Extra label filter applied: running tests with label filter '%s'", labelsToRun)
	}

	ginkgoArgs := []string{"-p", "-v", "--output-interceptor-mode=none", "--no-color", "--fail-on-empty",
		"--timeout=" + timeout, "--json-report=e2e-report.json", fmt.Sprintf("--output-dir=%s", artifactDir),
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
