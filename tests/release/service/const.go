package service

import (
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

const (
	snapshotName                                = "snapshot"
	destinationReleasePlanAdmissionName         = "sre-production"
	releaseName                                 = "release"
	applicationName                             = "application"
	serviceAccount                              = constants.DefaultPipelineServiceAccount
	releaseStrategyPolicy                       = "policy"
	verifyEnterpriseContractTaskName            = "verify-enterprise-contract"
	applicationNameDefault               string = "appstudio"
	componentName                        string = "dc-metro-map"
	releaseEnvironment                   string = "production"
	releaseStrategyPolicyDefault         string = "mvp-policy"
	releasePipelineServiceAccountDefault string = "release-service-account"
	sourceReleasePlanName                string = "source-releaseplan"
	targetReleasePlanAdmissionName       string = "demo"
	releasePvcName                       string = "release-pvc"
	redhatAppstudioUserSecret            string = "hacbs-release-tests-token"
	hacbsReleaseTestsTokenSecret         string = "redhat-appstudio-registry-pull-secret"
	publicSecretNameAuth                 string = "cosign-public-key"
	gitSourceComponentUrl                string = "https://github.com/scoheb/dc-metro-map"
	releasedImagePushRepo                string = "quay.io/redhat-appstudio-qe/dcmetromap"
	cacheSyncTimeout                            = 1 * time.Minute
	releaseCreationTimeout                      = 5 * time.Minute
	releasePipelineRunCreationTimeout           = 10 * time.Minute
	releasePipelineRunCompletionTimeout         = 20 * time.Minute
	releasePlanOwnerReferencesTimeout           = 1 * time.Minute

	defaultInterval = 100 * time.Millisecond
	releaseDeploymentTimeout                    = 10 * time.Minute
	releaseFinishedTimeout                      = 5 * time.Minute

)

var managednamespaceSecret = []corev1.ObjectReference{
	{Name: redhatAppstudioUserSecret},
}
