package release

import (
	"time"
)

const (
	snapshotName                        = "snapshot"
	destinationReleasePlanAdmissionName = "sre-production"
	releaseStrategyName                 = "strategy"
	releaseName                         = "release"
	releasePipelineName                 = "release-pipeline"
	applicationName                     = "application"
	releasePipelineBundle               = "quay.io/hacbs-release/demo:m5-alpine"
	serviceAccount                      = "pipeline"
	releaseStrategyPolicy               = "policy"
	environment                         = "test-environment"
	releaseStrategyServiceAccount       = "pipeline"

	sourceReleaseLinkName                = "source-release-link"
	targetReleaseLinkName                = "target-release-link"
	releasePipelineNameDefault           = "m6-release-pipeline"
	applicationNameDefault               = "appstudio"
	componentName                        = "java-springboot"
	releasePipelineBundleDefault         = "quay.io/hacbs-release/m6-release-pipeline:main"
	releaseStrategyPolicyDefault         = "m7-policy"
	releaseStrategyServiceAccountDefault = "service-account"
	sourceReleasePlanName                = "source-releaseplan"
	targetReleasePlanAdmissionName       = "target-releaseplanadmission"
	releasePvcName                       = "release-pvc"
	releaseEnvironment                   = "sre-production"
	redhatAppstudioUserSecret            = "redhat-appstudio-user-workload"
	hacbsReleaseTestsTokenSecret         = "hacbs-release-tests-token"
	publicSecretNameAuth                 = "cosign-public-key"
	gitSourceComponentUrl                = "https://github.com/scoheb/devfile-sample-java-springboot-basic"
	sourceKeyName                        = "release-e2e+release_e2e"
	destinationKeyName                   = "hacbs-release-tests+m5_robot_account"

	namespaceCreationTimeout              = 1 * time.Minute
	namespaceDeletionTimeout              = 1 * time.Minute
	snapshotCreationTimeout               = 1 * time.Minute
	releaseStrategyCreationTimeout        = 1 * time.Minute
	releasePlanCreationTimeout            = 1 * time.Minute
	EnterpriseContractPolicyTimeout       = 1 * time.Minute
	releasePlanAdmissionCreationTimeout   = 1 * time.Minute
	releaseCreationTimeout                = 1 * time.Minute
	releasePipelineRunCreationTimeout     = 5 * time.Minute
	releasePipelineRunCompletionTimeout   = 10 * time.Minute
	avgControllerQueryTimeout             = 1 * time.Minute
	pipelineServiceAccountCreationTimeout = 3 * time.Minute

	avgPipelineCompletionTime = 2 * time.Minute
	defaultInterval           = 100 * time.Millisecond
)
