package constants

// Global constants
const (
	// A github token is required to run the tests. The token need to have permissions to the given github organization. By default the e2e use redhat-appstudio-qe github organization.
	GITHUB_TOKEN_ENV string = "GITHUB_TOKEN" // #nosec

	// The github organization is used to create the gitops repositories in Red Hat Appstudio.
	GITHUB_E2E_ORGANIZATION_ENV string = "GITHUB_E2E_ORGANIZATION" // #nosec

	// The github organization is used to create the gitops repositories in Red Hat Appstudio.
	QUAY_E2E_ORGANIZATION_ENV string = "QUAY_E2E_ORGANIZATION" // #nosec

	//The Tekton namespace
	TEKTON_CHAINS_NS string = "tekton-chains" // #nosec

	//Cluster Registration namespace 
	CLUSTER_REG_NS string = "cluster-reg-config" // #nosec
)
