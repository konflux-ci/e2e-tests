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

	namespaceCreationTimeout              = 20 * time.Second
	namespaceDeletionTimeout              = 20 * time.Second
	snapshotCreationTimeout               = 5 * time.Second
	releaseStrategyCreationTimeout        = 5 * time.Second
	releasePlanCreationTimeout            = 5 * time.Second
	EnterpriseContractPolicyTimeout       = 5 * time.Second
	releasePlanAdmissionCreationTimeout   = 5 * time.Second
	releaseCreationTimeout                = 5 * time.Second
	releasePipelineRunCreationTimeout     = 20 * time.Second
	releasePipelineRunCompletionTimeout   = 900 * time.Second
	avgControllerQueryTimeout             = 5 * time.Second
	pipelineServiceAccountCreationTimeout = 5 * time.Minute

	defaultInterval = 100 * time.Millisecond

	sourceReleaseLinkName                string = "source-release-link"
	targetReleaseLinkName                string = "target-release-link"
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
	roleBindingName                      string = "role-relase-service-account-binding"
	subjectKind                          string = "ServiceAccount"
	roleRefApiGroup                      string = "rbac.authorization.k8s.io"
	roleRefName                          string = "role-m6-service-account"
	roleRefKind                          string = "Role"
	displayEnvironment                   string = "demo production"
	avgPipelineCompletionTime                   = 2 * time.Minute
	// defaultInterval           = 100 * time.Millisecond
)
