package constants

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

// Global constants
const (
	// A github token is required to run the tests. The token need to have permissions to the given github organization. By default the e2e use redhat-appstudio-qe github organization.
	GITHUB_TOKEN_ENV string = "GITHUB_TOKEN" // #nosec

	// The github organization is used to create the gitops repositories in Red Hat Appstudio.
	GITHUB_E2E_ORGANIZATION_ENV string = "GITHUB_E2E_ORGANIZATION" // #nosec

	// The github organization is used to create the gitops repositories in Red Hat Appstudio.
	QUAY_E2E_ORGANIZATION_ENV string = "QUAY_E2E_ORGANIZATION" // #nosec

	//base64 Encoded docker config json value to create registry pull secret
	DOCKER_CONFIG_JSON string = "DOCKER_CONFIG_JSON"

	//The Tekton namespace
	TEKTON_CHAINS_NS string = "tekton-chains" // #nosec

	//Cluster Registration namespace
	CLUSTER_REG_NS string = "cluster-reg-config" // #nosec

	// E2E test namespace where the app and component CRs will be created
	E2E_APPLICATIONS_NAMESPACE_ENV string = "E2E_APPLICATIONS_NAMESPACE"

	// Skip checking "ApplicationServiceGHTokenSecrName" secret
	SKIP_HAS_SECRET_CHECK_ENV string = "SKIP_HAS_SECRET_CHECK"

	// Test namespace's required labels
	ArgoCDLabelKey   string = "argocd.argoproj.io/managed-by"
	ArgoCDLabelValue string = "gitops-service-argocd"

	HostOperatorNamespace  string = "toolchain-host-operator"
	MemberOperatorNamespace  string = "toolchain-member-operator"

	OLMOperatorNamespace string = "openshift-operator-lifecycle-manager"
	OLMOperatorWorkload string = "olm-operator"

	OSAPIServerNamespace string = "openshift-apiserver"
	OSAPIServerWorkload string = "apiserver"

)

func GetQuayIOOrganization() string {
	return utils.GetEnv(QUAY_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
}

func GetDockerConfigJson() string {
	return utils.GetEnv(DOCKER_CONFIG_JSON, "")
}
