package common

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"time"
)

const (
	ApplicationNameDefault       string = "appstudio"
	ReleaseStrategyPolicyDefault string = "mvp-policy"
	ReleaseStrategyPolicy        string = "policy"

	RedhatAppstudioUserSecret            string = "hacbs-release-tests-token"
	HacbsReleaseTestsTokenSecret         string = "redhat-appstudio-registry-pull-secret"
	PublicSecretNameAuth                 string = "cosign-public-key"
	ReleasePipelineServiceAccountDefault string = "release-service-account"

	SourceReleasePlanName          string = "source-releaseplan"
	TargetReleasePlanAdmissionName string = "demo"
	ReleasePvcName                 string = "release-pvc"
	ReleaseEnvironment             string = "production"

	ReleaseCreationTimeout              = 5 * time.Minute
	ReleasePipelineRunCreationTimeout   = 10 * time.Minute
	ReleasePipelineRunCompletionTimeout = 20 * time.Minute
	DefaultInterval                     = 100 * time.Millisecond

	// Pipelines constants
	ComponentName                   string = "dc-metro-map"
	GitSourceComponentUrl           string = "https://github.com/scoheb/dc-metro-map"
	AdditionalComponentName         string = "simple-python"
	AdditionalGitSourceComponentUrl string = "https://github.com/devfile-samples/devfile-sample-python-basic"
	ReleasedImagePushRepo           string = "quay.io/redhat-appstudio-qe/dcmetromap"
	AdditionalReleasedImagePushRepo string = "quay.io/redhat-appstudio-qe/simplepython"
	PyxisStageImagesApiEndpoint     string = "https://pyxis.preprod.api.redhat.com/v1/images/id/"

	// Service constants
	ApplicationName string = "application"
)

var ManagednamespaceSecret = []corev1.ObjectReference{
	{Name: RedhatAppstudioUserSecret},
}

// Pipelines variables
var (
	RelSvcCatalogURL      string = utils.GetEnv("RELEASE_SERVICE_CATALOG_URL", "https://github.com/redhat-appstudio/release-service-catalog")
	RelSvcCatalogRevision string = utils.GetEnv("RELEASE_SERVICE_CATALOG_REVISION", "development")
)
