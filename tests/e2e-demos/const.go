package e2e

const (
	// Application Service controller is deployed the namespace: https://github.com/redhat-appstudio/infra-deployments/blob/main/argo-cd-apps/base/has.yaml#L14
	RedHatAppStudioApplicationNamespace string = "application-service"

	// See more info: https://github.com/redhat-appstudio/application-service#creating-a-github-secret-for-has
	ApplicationServiceGHTokenSecrName string = "has-github-token" // #nosec

	// Name for the GitOps Deployment resource
	GitOpsDeploymentName string = "gitops-deployment-e2e"

	// GitOps repository branch to use
	GitOpsRepositoryRevision string = "main"
)
