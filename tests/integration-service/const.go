package integration

import (
	"fmt"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

const (
	componentRepoURL = "https://github.com/redhat-appstudio-qe/hacbs-test-project"
	EnvNameForNBE    = "user-picked-environment"
	revisionForNBE   = "main"
	pathInRepoForNBE = "pipelines/integration_test_app.yaml"

	BundleURL              = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass"
	BundleURLFail          = "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-fail"
	InPipelineName         = "integration-pipeline-pass"
	InPipelineNameFail     = "integration-pipeline-fail"
	EnvironmentName        = "development"
	gitURL                 = "https://github.com/redhat-appstudio/integration-examples.git"
	revision               = "843f455fe87a6d7f68c238f95a8f3eb304e65ac5"
	pathInRepo             = "pipelines/integration_resolver_pipeline_pass.yaml"
	autoReleasePlan        = "auto-releaseplan"
	targetReleaseNamespace = "default"

	componentRepoNameForStatusReporting = "hacbs-test-project-integration"
	componentDefaultBranch              = "main"
	componentRevision                   = "34da5a8f51fba6a8b7ec75a727d3c72ebb5e1274"
	pathInRepoForReportingPass          = "pipelines/integration_resolver_pipeline_pass.yaml"
	pathInRepoForReportingFail          = "pipelines/integration_resolver_pipeline_fail.yaml"
	referenceDoesntExist                = "Reference does not exist"
	checkrunStatusCompleted             = "completed"
	checkrunConclusionSuccess           = "success"
	checkrunConclusionFailure           = "failure"

	environmentLabel                         = "appstudio.openshift.io/environment"
	snapshotAnnotation                       = "appstudio.openshift.io/snapshot"
	scenarioAnnotation                       = "test.appstudio.openshift.io/scenario"
	pipelinerunFinalizerByIntegrationService = "test.appstudio.openshift.io/pipelinerun"
	snapshotRerunLabel                       = "test.appstudio.openshift.io/run"

	chainsSignedAnnotation = "chains.tekton.dev/signed"
)

var (
	componentGitSourceURLForStatusReporting = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), componentRepoNameForStatusReporting)
)
