package release

import (
	"time"

	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

	releasePipelineNameDefault           string = "release"
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
	releaseEnvironment                   string = "production"
	redhatAppstudioUserSecret            string = "hacbs-release-tests-token"
	hacbsReleaseTestsTokenSecret         string = "redhat-appstudio-registry-pull-secret"
	publicSecretNameAuth                 string = "cosign-public-key"
	gitSourceComponentUrl                string = "https://github.com/scoheb/dc-metro-map"
	sourceKeyName                        string = "release-e2e+release_e2e"
	destinationKeyName                   string = "redhat-appstudio-qe+redhat_appstudio_quality"
	containerImageUrl                    string = "quay.io/redhat-appstudio-qe/dcmetromap:latest"
	releasePipelineBundleDefault         string = "quay.io/hacbs-release/pipeline-release:main"
	roleName                             string = "role-release-service-account"

	namespaceCreationTimeout              = 5 * time.Minute
	namespaceDeletionTimeout              = 5 * time.Minute
	snapshotCreationTimeout               = 5 * time.Minute
	releaseStrategyCreationTimeout        = 5 * time.Minute
	releasePlanCreationTimeout            = 5 * time.Minute
	EnterpriseContractPolicyTimeout       = 5 * time.Minute
	releasePlanAdmissionCreationTimeout   = 5 * time.Minute
	releaseCreationTimeout                = 5 * time.Minute
	releasePipelineRunCreationTimeout     = 25 * time.Minute
	releasePipelineRunCompletionTimeout   = 40 * time.Minute
	avgControllerQueryTimeout             = 5 * time.Minute
	pipelineServiceAccountCreationTimeout = 7 * time.Minute

	defaultInterval = 100 * time.Millisecond
)

var paramsReleaseStrategy = []appstudiov1alpha1.Params{
	{Name: "extraConfigGitUrl", Value: "https://github.com/scoheb/strategy-configs.git"},
	{Name: "extraConfigPath", Value: "m6.yaml"},
	{Name: "extraConfigRevision", Value: "main"},
}

var managednamespaceSecret = []corev1.ObjectReference{
	{Name: redhatAppstudioUserSecret},
}

var roleRules = map[string][]string{
	"apiGroupsList": {""},
	"roleResources": {"secrets"},
	"roleVerbs":     {"get", "list", "watch"},
}
