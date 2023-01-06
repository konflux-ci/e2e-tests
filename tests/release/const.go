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

	namespaceCreationTimeout              = 60 * time.Second
	namespaceDeletionTimeout              = 60 * time.Second
	snapshotCreationTimeout               = 60 * time.Second
	releaseStrategyCreationTimeout        = 60 * time.Second
	releasePlanCreationTimeout            = 60 * time.Second
	EnterpriseContractPolicyTimeout       = 60 * time.Second
	releasePlanAdmissionCreationTimeout   = 60 * time.Second
	releaseCreationTimeout                = 60 * time.Second
	releasePipelineRunCreationTimeout     = 5 * time.Minute
	releasePipelineRunCompletionTimeout   = 10 * time.Minute
	avgControllerQueryTimeout             = 10 * time.Second
	pipelineServiceAccountCreationTimeout = 3 * time.Minute

	defaultInterval = 100 * time.Millisecond
)
