package constants

// Global constants
const (
	// A github token is required to run the tests. The token need to have permissions to the given github organization. By default the e2e use redhat-appstudio-qe github organization.
	GITHUB_TOKEN_ENV string = "GITHUB_TOKEN" // #nosec

	// The github organization is used to create the gitops repositories in Red Hat Appstudio.
	GITHUB_E2E_ORGANIZATION_ENV string = "GITHUB_E2E_ORGANIZATION" // #nosec

	// The github organization is used to create the gitops repositories in Red Hat Appstudio.
	QUAY_E2E_ORGANIZATION_ENV string = "QUAY_E2E_ORGANIZATION" // #nosec

	// The private devfile sample git repository to use in certain HAS e2e tests
	PRIVATE_DEVFILE_SAMPLE string = "PRIVATE_DEVFILE_SAMPLE" // #nosec

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

	BuildPipelinesConfigMapName             = "build-pipelines-defaults"
	BuildPipelinesConfigMapDefaultNamespace = "build-templates"
)
