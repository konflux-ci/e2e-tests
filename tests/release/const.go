package release

import (
	"time"
)

const (
	snapshotName                                  string = "snapshot"
	sourceReleasePlanName                         string = "source-releaseplan"
	targetReleasePlanAdmissionName                string = "target-releaseplanadmission"
	releaseStrategyName                           string = "m7-strategy"
	releaseName                                   string = "release"
	releasePipelineName                           string = "m6-release-pipeline"
	applicationName                               string = "application"
	releasePipelineBundle                         string = "quay.io/hacbs-release/m6-release-pipeline:main"
	releaseStrategyPolicy                         string = "m7-policy"
	releaseStrategyServiceAccount                 string = "m7-service-account"
	ReleaseStrategySpecParams_extraConfigGitUrl   string = "https://github.com/scoheb/strategy-configs.git"
	ReleaseStrategySpecParams_extraConfigPath     string = "m6.yaml"
	ReleaseStrategySpecParams_extraConfigRevision string = "main"

	avgPipelineCompletionTime = 10 * time.Minute
	defaultInterval           = 100 * time.Millisecond
)
