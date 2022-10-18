package release

import (
	"time"
)

const (
	snapshotName             = "snapshot"
	sourceReleasePlanName    = "source-release-plan"
	destinationReleasePAName = "sre-production"
	releaseStrategyName      = "strategy"
	releaseName              = "release"
	releasePipelineName      = "release-pipeline"
	applicationName          = "application"
	releasePipelineBundle    = "quay.io/hacbs-release/demo:m5-alpine"
	serviceAccount           = "pipeline"
	releaseStrategyPolicy    = "policy"

	avgPipelineCompletionTime = 2 * time.Minute
	defaultInterval           = 100 * time.Millisecond
)
