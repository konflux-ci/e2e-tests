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

	namespaceCreationTimeout              = 20 * time.Second
	namespaceDeletionTimeout              = 20 * time.Second
	snapshotCreationTimeout               = 2 * time.Second
	releaseStrategyCreationTimeout        = 1 * time.Second
	releasePlanCreationTimeout            = 1 * time.Second
	EnterpriseContractPolicyTimeout       = 1 * time.Second
	releasePlanAdmissionCreationTimeout   = 1 * time.Second
	releaseCreationTimeout                = 1 * time.Second
	releasePipelineRunCreationTimeout     = 1 * time.Second
	releasePipelineRunCompletionTimeout   = 120 * time.Second
	avgControllerQueryTimeout             = 1 * time.Second
	pipelineServiceAccountCreationTimeout = 1 * time.Minute

	defaultInterval = 100 * time.Millisecond
)
