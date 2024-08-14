package integration

import (
	"fmt"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

const (
	EnvironmentName                = "development"
	gitURL                         = "https://github.com/konflux-ci/integration-examples.git"
	revision                       = "ab868616ab02be79b6abdf85dcd2a3aef321ff14"
	pathInRepoPass                 = "pipelines/integration_resolver_pipeline_pass.yaml"
	pathIntegrationPipelineWithEnv = "pipelines/integration_resolver_pipeline_environment_pass.yaml"
	pathInRepoFail                 = "pipelines/integration_resolver_pipeline_fail.yaml"
	autoReleasePlan                = "auto-releaseplan"
	targetReleaseNamespace         = "default"

	componentRepoNameForGeneralIntegration    = "konflux-test-integration"
	componentRepoNameForIntegrationWithEnv    = "konflux-test-integration-with-env"
	componentRepoNameForStatusReporting       = "konflux-test-integration-status-report"
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
	pipelinerunFinalizerByIntegrationService = "test.appstudio.openshift.io/pipelinerun"
	snapshotRerunLabel                       = "test.appstudio.openshift.io/run"

	chainsSignedAnnotation = "chains.tekton.dev/signed"
)

var (
	componentGitSourceURLForGeneralIntegration    = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), componentRepoNameForGeneralIntegration)
	componentGitSourceURLForIntegrationWithEnv    = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), componentRepoNameForIntegrationWithEnv)
	componentGitSourceURLForStatusReporting       = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), componentRepoNameForStatusReporting)
	gitlabOrg                                     = utils.GetEnv(constants.GITLAB_QE_ORG_ENV, constants.DefaultGitLabQEOrg)
	gitlabProjectIDForStatusReporting             = fmt.Sprintf("%s/%s", gitlabOrg, componentRepoNameForGeneralIntegration)
	gitlabComponentGitSourceURLForStatusReporting = fmt.Sprintf("https://gitlab.com/%s/%s", gitlabOrg, componentRepoNameForGeneralIntegration)
)
