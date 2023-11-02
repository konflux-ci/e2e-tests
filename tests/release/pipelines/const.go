package pipelines

import (
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

const (
	serviceAccount                              = constants.DefaultPipelineServiceAccount
	applicationNameDefault               string = "appstudio"
	componentName                        string = "dc-metro-map"
	releaseStrategyPolicyDefault         string = "mvp-policy"
	releaseStrategyServiceAccountDefault string = "release-service-account"
	sourceReleasePlanName                string = "source-releaseplan"
	targetReleasePlanAdmissionName       string = "demo"
	releasePvcName                       string = "release-pvc"
	releaseEnvironment                   string = "production"
	redhatAppstudioUserSecret            string = "hacbs-release-tests-token"
	hacbsReleaseTestsTokenSecret         string = "redhat-appstudio-registry-pull-secret"
	publicSecretNameAuth                 string = "cosign-public-key"
	gitSourceComponentUrl                string = "https://github.com/scoheb/dc-metro-map"
	releasedImagePushRepo                string = "quay.io/redhat-appstudio-qe/dcmetromap"
	additionalReleasedImagePushRepo      string = "quay.io/redhat-appstudio-qe/simplepython"

	additionalComponentName             string = "simple-python"
	additionalGitSourceComponentUrl     string = "https://github.com/devfile-samples/devfile-sample-python-basic"
	pyxisStageImagesApiEndpoint         string = "https://pyxis.preprod.api.redhat.com/v1/images/id/"
	snapshotCreationTimeout                    = 5 * time.Minute
	releaseCreationTimeout                     = 5 * time.Minute
	releasePipelineRunCreationTimeout          = 10 * time.Minute
	releasePipelineRunCompletionTimeout        = 20 * time.Minute
	avgControllerQueryTimeout                  = 5 * time.Minute
	releaseDeploymentTimeout                   = 10 * time.Minute
	releaseFinishedTimeout                     = 5 * time.Minute

	defaultInterval = 100 * time.Millisecond
)

var managednamespaceSecret = []corev1.ObjectReference{
	{Name: redhatAppstudioUserSecret},
}
