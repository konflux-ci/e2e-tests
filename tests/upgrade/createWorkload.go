package upgrade

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/upgrade/create"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	. "github.com/redhat-appstudio/e2e-tests/tests/upgrade/utils"
)

var _ = framework.UpgradeSuiteDescribe("Create users and check their state", Label("upgrade-create"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var testNamespace string

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(UpgradeNamespace)
		Expect(err).NotTo(HaveOccurred())

		testNamespace = fw.UserNamespace
		Expect(testNamespace).NotTo(BeEmpty())
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
	})

	It("creates AppStudioProvisionedUser", func() {
		create.CreateAppStudioProvisionedUser(fw)
	})

	It("creates AppStudioBannedUser", func() {
		create.CreateAppStudioBannedUser(fw)
	})

})
