package upgrade

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	utilsUpgrade "github.com/redhat-appstudio/e2e-tests/tests/upgrade/utils"
)

var _ = framework.UpgradeSuiteDescribe("Create users and check their state", Label("upgrade-cleanup"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var testNamespace string

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(utilsUpgrade.UpgradeNamespace)
		Expect(err).NotTo(HaveOccurred())

		testNamespace = fw.UserNamespace
		Expect(testNamespace).NotTo(BeEmpty())
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
	})

	It("Verify AppStudioProvisionedUser", func() {
		_, err := fw.SandboxController.DeleteUserSignup(utilsUpgrade.AppStudioProvisionedUser)
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates AppStudioBannedUser", func() {
		_, err := fw.SandboxController.DeleteUserSignup(utilsUpgrade.BannedUser)
		Expect(err).NotTo(HaveOccurred())
	})

})
