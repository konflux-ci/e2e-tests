package release

import (
	"time"
)

const (
	snapshotName                   string = "snapshot"
	sourceReleasePlanName          string = "source-releaseplan"
	targetReleasePlanAdmissionName string = "target-releaseplanadmission"
	releaseStrategyName            string = "m6-strategy"
	releaseName                    string = "release"
	releasePipelineName            string = "m6-release-pipeline"
	applicationName                string = "application"
	releasePipelineBundle          string = "quay.io/hacbs-release/m6-release-pipeline:main"
	releaseStrategyPolicy          string = "m6-policy"

	avgPipelineCompletionTime = 10 * time.Minute
	defaultInterval           = 100 * time.Millisecond
)

