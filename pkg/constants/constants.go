package constants

// Global constants
const (
	// A github token is required to run the tests. The token need to have permissions to the given github organization. By default the e2e use redhat-appstudio-qe github organization.
	GITHUB_TOKEN_ENV string = "GITHUB_TOKEN" // #nosec

	// The github organization is used to create the gitops repositories in Red Hat Appstudio.
	GITHUB_E2E_ORGANIZATION_ENV string = "MY_GITHUB_ORG" // #nosec

	// The quay organization is used to push container images using Red Hat Appstudio pipelines.
	QUAY_E2E_ORGANIZATION_ENV string = "QUAY_E2E_ORGANIZATION" // #nosec

	// The quay.io username to perform container builds and puush
	QUAY_OAUTH_USER_ENV string = "QUAY_OAUTH_USER" // #nosec

	// The quay.io token to perform container builds and push. The token must be correlated with the QUAY_OAUTH_USER environment
	QUAY_OAUTH_TOKEN_ENV string = "QUAY_OAUTH_TOKEN" // #nosec

	// The private devfile sample git repository to use in certain HAS e2e tests
	PRIVATE_DEVFILE_SAMPLE string = "PRIVATE_DEVFILE_SAMPLE" // #nosec

	// The Tekton Chains namespace
	TEKTON_CHAINS_NS string = "tekton-chains" // #nosec

	// Namespace for running the end-to-end Tekton Chains tests
	TEKTON_CHAINS_E2E_NS string = "tekton-chains-e2e"

	// Namespace for running the end-to-end Pipeline-Service tests
	PIPELINE_SERVICE_E2E_NS string = "pipeline-service-e2e"

	//base64 Encoded docker config json value to create registry pull secret
	DOCKER_CONFIG_JSON string = "DOCKER_CONFIG_JSON"

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

	HostOperatorNamespace   string = "toolchain-host-operator"
	MemberOperatorNamespace string = "toolchain-member-operator"

	HostOperatorWorkload   string = "host-operator-controller-manager"
	MemberOperatorWorkload string = "member-operator-controller-manager"

	OLMOperatorNamespace string = "openshift-operator-lifecycle-manager"
	OLMOperatorWorkload  string = "olm-operator"

	OSAPIServerNamespace string = "openshift-apiserver"
	OSAPIServerWorkload  string = "apiserver"

	RegistryAuthSecretName = "redhat-appstudio-registry-pull-secret"

	SharedPullSecretName      = "redhat-appstudio-user-workload"
	SharedPullSecretNamespace = "build-templates"

	JVMBuildImageSecretName = "jvm-build-image-secrets"
	JBSConfigName           = "jvm-build-config"
)

var (
	ComponentDefaultLabel         = map[string]string{"e2e-test": "true"}
	ComponentDefaultAnnotation    = map[string]string{"appstudio.openshift.io/component-initial-build": "processed"}
	ComponentPaCRequestAnnotation = map[string]string{"appstudio.openshift.io/pac-provision": "request"}
)
