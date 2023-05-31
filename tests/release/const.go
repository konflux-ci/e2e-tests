package release

import (
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
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
	serviceAccount                      = constants.DefaultPipelineServiceAccount
	releaseStrategyPolicy               = "policy"
	environment                         = "test-environment"
	releaseStrategyServiceAccount       = constants.DefaultPipelineServiceAccount

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
	roleName                             string = "role-release-service-account"

	additionalComponentName         string = "simple-python"
	additionalGitSourceComponentUrl string = "https://github.com/devfile-samples/devfile-sample-python-basic"
	addtionalOutputContainerImage   string = constants.DefaultReleasedImagePushRepo
	pyxisStageURL                   string = "https://pyxis.preprod.api.redhat.com/v1/images/id/"

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

var paramsReleaseStrategyPyxis = []appstudiov1alpha1.Params{
	{Name: "extraConfigGitUrl", Value: "https://github.com/hacbs-release/strategy-configs"},
	{Name: "extraConfigPath", Value: "mvp.yaml"},
	{Name: "extraConfigGitRevision", Value: "main"},
	{Name: "pyxisServerType", Value: "stage"},
	{Name: "pyxisSecret", Value: "pyxis"},
	{Name: "tag", Value: "latest"},
}

var paramsReleaseStrategyMvp = []appstudiov1alpha1.Params{
	{Name: "extraConfigGitUrl", Value: "https://github.com/hacbs-release/strategy-configs"},
	{Name: "extraConfigPath", Value: "mvp.yaml"},
	{Name: "extraConfigGitRevision", Value: "main"},
}

var paramsReleaseStrategyM6 = []appstudiov1alpha1.Params{
	{Name: "extraConfigGitUrl", Value: "https://github.com/hacbs-release/strategy-configs"},
	{Name: "extraConfigPath", Value: "mvp.yaml"},
	{Name: "extraConfigGitRevision", Value: "main"},
}

var managednamespaceSecret = []corev1.ObjectReference{
	{Name: redhatAppstudioUserSecret},
}
