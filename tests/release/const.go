package release

import (
	"time"
)

const (
	snapshotName                   string = "snapshot"
	sourceReleaseLinkName          string = "source-release-link"
	targetReleaseLinkName          string = "target-release-link"
	releaseStrategyName            string = "strategy"
	releaseName                    string = "release"
	releasePipelineName            string = "release-pipeline"
	applicationName                string = "application"
	releasePipelineBundle          string = "quay.io/hacbs-release/demo:m5-alpine"
	releaseStrategyPolicy          string = "policy"
	releaseStrategyServiceAccount  string = "pipeline"
	sourceReleasePlanName          string = "source-releaseplan"
	targetReleasePlanAdmissionName string = "target-releaseplanadmission"

	avgPipelineCompletionTime = 10 * time.Minute
	defaultInterval           = 100 * time.Millisecond
)
