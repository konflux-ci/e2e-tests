package release

import (
	"time"
)

const (
	snapshotName                        = "snapshot"
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

	releasePipelineNameDefault           string = "bundle-release-pipelinerun"
	applicationNameDefault               string = "appstudio"
	componentName                        string = "dc-metro-map"
	buildPipelineBundleDefault           string = "quay.io/redhat-appstudio/hacbs-templates-bundle:latest"
	buildPipelineBundleDefaultName       string = "build-pipelines-defaults"
	releaseStrategyPolicyDefault         string = "mvp-policy"
	releaseStrategyServiceAccountDefault string = "release-service-account"
	sourceReleasePlanName                string = "source-releaseplan"
	targetReleasePlanAdmissionName       string = "demo"
	releasePvcName                       string = "release-pvc"
	releaseStrategyDefaultName           string = "mvp-strategy"
	releaseEnvironment                   string = "production" //"sre-production"
	redhatAppstudioUserSecret            string = "hacbs-release-tests-token"
	hacbsReleaseTestsTokenSecret         string = "redhat-appstudio-registry-pull-secret"
	publicSecretNameAuth                 string = "cosign-public-key"
	gitSourceComponentUrl                string = "https://github.com/sbose78/dc-metro-map"
	sourceKeyName                        string = "release-e2e+release_e2e"
	destinationKeyName                   string = "hacbs-release-tests+m5_robot_account"
	containerImageUrl                    string = "quay.io/hacbs-release-tests/dcmetromap:latest"
	releasePipelineBundleDefault         string = "quay.io/hacbs-release/pipeline-release:main"
	roleName                             string = "role-release-service-account"
	roleRefKind                          string = "Role"
	displayEnvironment                   string = "demo production"

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
