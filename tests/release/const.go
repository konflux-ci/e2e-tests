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

	namespaceCreationTimeout            = 20 * time.Second // actual messured time between 11-20 seconds
	namespaceDeletionTimeout            = 20 * time.Second // actual messured time between 11-20 seconds
	snapshotCreationTimeout             = 2 * time.Second  // actual messured time between 1-2 seconds
	releaseStrategyCreationTimeout      = 2 * time.Second  // actual messured time between 0-1 seconds
	releasePlanCreationTimeout          = 2 * time.Second  // actual messured time between 0-1 seconds
	EnterpriseContractPolicyTimeout     = 2 * time.Second  // actual messured time between 0-1 seconds
	releasePlanAdmissionCreationTimeout = 2 * time.Second  // actual messured time between 0-1 seconds
	releaseCreationTimeout              = 2 * time.Second  // actual messured time between 0-1 seconds
	releasePipelineRunCreationTimeout   = 2 * time.Second  // actual messured time between 0-1 seconds
	releasePipelineRunCompletionTimeout = 10 * time.Second // actual messured time between 6-10 seconds
	avgControllerQueryTimeout           = 2 * time.Second  // average controller query timeout, actual messured time between 0-1 seconds

	defaultInterval = 100 * time.Millisecond
)
