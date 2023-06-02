package byoc

import (
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
)

// Describe Byoc test scenarios
type Scenario struct {
	// Scenario test name
	Name string

	// Describe an application to create into user BYOC cluster
	ApplicationService ApplicationService

	// Specs obout BYOC provided by user
	Byoc Byoc
}

// RHTAP Application status
type ApplicationService struct {
	// Valid github public repository
	GithubRepository string

	// Application name sample created in BYOC cluster
	ApplicationName string
}

// Byoc is an RHTAP environment provided by users
type Byoc struct {
	// Define the cluster provided by user where to deploy RHTAP apps
	ClusterType appservice.ConfigurationClusterType

	// Target Namespace where to deploy RHTAP applications in BYOC provided
	TargetNamespace string
}
