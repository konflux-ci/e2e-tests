package integration

import (
	"fmt"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

const (
	EnvironmentName                = "development"
	gitURL                         = "https://github.com/konflux-ci/integration-examples.git"
	revision                       = "6a7c81802e785aa869f82301afe61f4e9775772b"
	pathInRepoPass                 = "pipelines/integration_resolver_pipeline_pass.yaml"
	pathIntegrationPipelineWithEnv = "pipelines/integration_resolver_pipeline_environment_pass.yaml"
	pathInRepoFail                 = "pipelines/integration_resolver_pipeline_fail.yaml"
	autoReleasePlan                = "auto-releaseplan"
	targetReleaseNamespace         = "default"

	componentRepoNameForGeneralIntegration    = "konflux-test-integration"
	componentRepoNameForGroupIntegration      = "konflux-test-integration-clone"
	componentRepoNameForIntegrationWithEnv    = "konflux-test-integration-with-env"
	componentRepoNameForStatusReporting       = "konflux-test-integration-status-report"
	multiComponentRepoNameForGroupSnapshot    = "group-snapshot-multi-component"
	multiComponentDefaultBranch               = "main"
	multiComponentGitRevision                 = "0d1835404efb8ab7bb1ab5b5b82cda1ebfda4b25"
	multiRepoComponentGitRevision             = "2e41cf5a68674503c86b6637de35eeedc2893794"
	gitlabComponentRepoName                   = "hacbs-test-project-integration"
	componentDefaultBranch                    = "main"
	componentRevision                         = "34da5a8f51fba6a8b7ec75a727d3c72ebb5e1274"
	referenceDoesntExist                      = "Reference does not exist"
	checkrunStatusCompleted                   = "completed"
	checkrunConclusionSuccess                 = "success"
	checkrunConclusionFailure                 = "failure"
	integrationPipelineRunCommitStatusSuccess = "success"
	integrationPipelineRunCommitStatusFail    = "failed"
	spaceRequestCronJobNamespace              = "spacerequest-cleaner"
	spaceRequestCronJobName                   = "spacerequest-cleaner"
	spaceRequestNamePrefix                    = "task-spacerequest-"

	snapshotAnnotation                       = "appstudio.openshift.io/snapshot"
	scenarioAnnotation                       = "test.appstudio.openshift.io/scenario"
	groupSnapshotAnnotation                  = "test.appstudio.openshift.io/pr-group"
	testGroupSnapshotAnnotation              = "test.appstudio.openshift.io/group-test-info"
	pipelinerunFinalizerByIntegrationService = "test.appstudio.openshift.io/pipelinerun"
	snapshotRerunLabel                       = "test.appstudio.openshift.io/run"
	snapshotCreationReport                   = "test.appstudio.openshift.io/snapshot-creation-report"

	chainsSignedAnnotation = "chains.tekton.dev/signed"
)

var (
	componentGitSourceURLForGeneralIntegration    = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), componentRepoNameForGeneralIntegration)
	componentGitSourceURLForGroupIntegration      = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), componentRepoNameForGroupIntegration)
	componentGitSourceURLForIntegrationWithEnv    = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), componentRepoNameForIntegrationWithEnv)
	componentGitSourceURLForStatusReporting       = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), componentRepoNameForStatusReporting)
	multiComponentGitSourceURLForGroupSnapshotA   = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), multiComponentRepoNameForGroupSnapshot)
	multiComponentGitSourceURLForGroupSnapshotB   = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), multiComponentRepoNameForGroupSnapshot)
	multiComponentContextDirs                     = []string{"go-component", "python-component"}
	gitlabOrg                                     = utils.GetEnv(constants.GITLAB_QE_ORG_ENV, constants.DefaultGitLabQEOrg)
	gitlabProjectIDForStatusReporting             = fmt.Sprintf("%s/%s", gitlabOrg, gitlabComponentRepoName)
	gitlabComponentGitSourceURLForStatusReporting = fmt.Sprintf("https://gitlab.com/%s/%s", gitlabOrg, gitlabComponentRepoName)
)
