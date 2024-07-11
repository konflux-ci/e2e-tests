package common

import (
	"fmt"
	"time"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

const (
	ApplicationNameDefault       string = "appstudio"
	ReleaseStrategyPolicyDefault string = "mvp-policy"
	ReleaseStrategyPolicy        string = "policy"

	RedhatAppstudioUserSecret            string = "hacbs-release-tests-token"
	RedhatAppstudioQESecret              string = "redhat-appstudio-qe-bot-token"
	HacbsReleaseTestsTokenSecret         string = "redhat-appstudio-registry-pull-secret"
	PublicSecretNameAuth                 string = "cosign-public-key"
	ReleasePipelineServiceAccountDefault string = "release-service-account"

	SourceReleasePlanName          string = "source-releaseplan"
	SecondReleasePlanName          string = "the-second-releaseplan"
	TargetReleasePlanAdmissionName string = "demo"
	ReleasePvcName                 string = "release-pvc"
	ReleaseEnvironment             string = "production"

	ReleaseCreationTimeout              = 5 * time.Minute
	ReleasePipelineRunCreationTimeout   = 10 * time.Minute
	ReleasePipelineRunCompletionTimeout = 60 * time.Minute
	BuildPipelineRunCompletionTimeout   = 60 * time.Minute
	BuildPipelineRunCreationTimeout     = 10 * time.Minute
	ReleasePlanStatusUpdateTimeout      = 1 * time.Minute
	DefaultInterval                     = 100 * time.Millisecond

	// Pipelines constants
	ComponentName                   string = "dc-metro-map"
	DcMetroMapGitRepoName           string = "dc-metro-map"
	DcMetroMapGitRevision           string = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	AdditionalComponentName         string = "simple-python"
	AdditionalGitSourceComponentUrl string = "https://github.com/konflux-qe-stage/devfile-sample-python-basic"
	AdditionalGitRevision           string = "47fc22092005aabebce233a9b6eab994a8152bbd"
	ReleasedImagePushRepo           string = "quay.io/redhat-appstudio-qe/dcmetromap"
	AdditionalReleasedImagePushRepo string = "quay.io/redhat-appstudio-qe/simplepython"
	PyxisStageImagesApiEndpoint     string = "https://pyxis.preprod.api.redhat.com/v1/images/id/"
	GitLabRunFileUpdatesTestRepo    string = "https://gitlab.cee.redhat.com/hacbs-release-tests/app-interface"

	// EC constants
	EcPolicyLibPath     = "github.com/enterprise-contract/ec-policies//policy/lib"
	EcPolicyReleasePath = "github.com/enterprise-contract/ec-policies//policy/release"
	EcPolicyDataBundle  = "oci::quay.io/redhat-appstudio-tekton-catalog/data-acceptable-bundles:latest"
	EcPolicyDataPath    = "github.com/release-engineering/rhtap-ec-policy//data"

	// Service constants
	ApplicationName string = "application"

	githubUrlFormat = "https://github.com/%s/%s"

	stageOrg = "konflux-qe-stage"
)

var ManagednamespaceSecret = []corev1.ObjectReference{
	{Name: RedhatAppstudioUserSecret},
}

// Pipelines variables
var (
	githubOrg                          = utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
	DcMetroMapGitSourceURL             = fmt.Sprintf(githubUrlFormat, githubOrg, DcMetroMapGitRepoName)
	DcMetroMapGitSourceStageURL        = fmt.Sprintf(githubUrlFormat, stageOrg, DcMetroMapGitRepoName)
	RelSvcCatalogURL            string = utils.GetEnv("RELEASE_SERVICE_CATALOG_URL", "https://github.com/konflux-ci/release-service-catalog")
	RelSvcCatalogRevision       string = utils.GetEnv("RELEASE_SERVICE_CATALOG_REVISION", "staging")
)
