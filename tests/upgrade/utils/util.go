package utils

import (
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	utilsFramework "github.com/konflux-ci/e2e-tests/pkg/utils"

	gomega "github.com/onsi/gomega"
)

func PrepareForUpgradeTests() (fw *framework.Framework, testNamespace string) {
	// Initialize the tests controllers
	fw, err := framework.NewFramework(UpgradeNamespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	testNamespace = fw.UserNamespace
	gomega.Expect(testNamespace).NotTo(gomega.BeEmpty())
	// Check to see if the github token was provided
	gomega.Expect(utilsFramework.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(gomega.BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
	return fw, testNamespace
}
