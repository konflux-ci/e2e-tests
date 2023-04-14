package spi

import "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

type AccessCheckTest struct {
	TestName        string
	Accessibility   v1beta1.SPIAccessCheckAccessibility
	RepoURL         string
	RepoType        v1beta1.SPIRepoType
	ServiceProvider v1beta1.ServiceProviderType
}

type ServiceAccountTest struct {
	TestName                string
	IsImagePullSecret       bool
	IsManagedServiceAccount bool
}

var (
	AccessCheckTests = []AccessCheckTest{
		{
			TestName:        "public GitHub repository",
			Accessibility:   v1beta1.SPIAccessCheckAccessibilityPublic,
			RepoURL:         "https://github.com/devfile-samples/devfile-sample-code-with-quarkus",
			RepoType:        v1beta1.SPIRepoTypeGit,
			ServiceProvider: v1beta1.ServiceProviderTypeGitHub,
		},
		{
			TestName:        "private GitHub repository",
			Accessibility:   v1beta1.SPIAccessCheckAccessibilityPrivate,
			RepoURL:         "https://github.com/redhat-appstudio-qe/private-quarkus-devfile-sample",
			RepoType:        v1beta1.SPIRepoTypeGit,
			ServiceProvider: v1beta1.ServiceProviderTypeGitHub,
		},
	}

	ServiceAccountTests = []ServiceAccountTest{
		{
			TestName:                "link a secret to an existing service account",
			IsImagePullSecret:       false,
			IsManagedServiceAccount: false,
		},
		{
			TestName:                "link a secret to an existing service account as image pull secret",
			IsImagePullSecret:       true,
			IsManagedServiceAccount: false,
		},
		{
			TestName:                "link a secret to a managed service account",
			IsImagePullSecret:       false,
			IsManagedServiceAccount: true,
		},
	}
)
