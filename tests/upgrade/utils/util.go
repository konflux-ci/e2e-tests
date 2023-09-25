package utils

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	utilsFramework "github.com/redhat-appstudio/e2e-tests/pkg/utils"

	. "github.com/onsi/gomega"
)

func PrepareForUpgradeTests() (fw *framework.Framework, testNamespace string) {
	// Initialize the tests controllers
	fw, err := framework.NewFramework(UpgradeNamespace)
	Expect(err).NotTo(HaveOccurred())

	testNamespace = fw.UserNamespace
	Expect(testNamespace).NotTo(BeEmpty())
	// Check to see if the github token was provided
	Expect(utilsFramework.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
	return fw, testNamespace
}
