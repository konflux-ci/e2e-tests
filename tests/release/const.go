package release

import (
	"time"
)

const (
	snapshotName                        = "snapshot"
	sourceReleasePlanName               = "source-release-plan"
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

	defaultInterval = 100 * time.Millisecond
)
