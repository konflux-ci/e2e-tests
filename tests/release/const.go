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
	releasePipelineName            string = "m6-release-pipeline"
	applicationName                string = "appstudio"
	componentName                  string = "java-springboot"
	releasePipelineBundle          string = "quay.io/hacbs-release/m6-release-pipeline:main"
	releaseStrategyPolicy          string = "m7-policy"
	releaseStrategyServiceAccount  string = "service-account"
	sourceReleasePlanName          string = "source-releaseplan"
	targetReleasePlanAdmissionName string = "target-releaseplanadmission"
	releasePvcName                 string = "release-pvc"
	releaseEnvironment             string = "sre-production"
	redhatAppstudioUserSecret      string = "redhat-appstudio-user-workload"
	hacbsReleaseTestsTokenSecret   string = "hacbs-release-tests-token"
	publicSecretNameAuth           string = "cosign-public-key"
	gitSourceComponentUrl          string = "https://github.com/scoheb/devfile-sample-java-springboot-basic"
	sourceKeyName                  string = "release-e2e+release_e2e"
	destinationKeyName             string = "hacbs-release-tests+m5_robot_account"

	avgPipelineCompletionTime = 10 * time.Minute
	defaultInterval           = 100 * time.Millisecond
)
