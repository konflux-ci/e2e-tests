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

	applicationSnapshotCreationTimeout  = 10 * time.Second // actual messured time between 5-6 seconds
	releaseStrategyCreationTimeout      = 2 * time.Second  // actual messured time between 0-1 seconds
	releasePlanCreationTimeout          = 2 * time.Second  // actual messured time between 0-1 seconds
	releasePlanAdmissionCreationTimeout = 2 * time.Second  // actual messured time between 0-1 seconds
	releaseCreationTimeout              = 2 * time.Second  // actual messured time between 0-1 seconds
	releasePipelineRunCreationTimeout   = 30 * time.Second // actual messured time between 16-17 seconds
	releasePipelineRunCompletionTimeot  = 10 * time.Minute // actual the test failed , this is an approximation
	releaseCreationTimeout              = 1 * time.Minute  // actual the test failed , this is an approximation

	avgPipelineCompletionTime = 2 * time.Minute
	defaultInterval           = 100 * time.Millisecond
)
