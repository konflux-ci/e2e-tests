package upgrade

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	utilsUpgrade "github.com/redhat-appstudio/e2e-tests/tests/upgrade/utils"
)

var _ = framework.UpgradeSuiteDescribe("Create users and check their state", Label("upgrade-cleanup"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework

	BeforeAll(func() {
		fw, _ = utilsUpgrade.PrepareForUpgradeTests()
	})

	It("Delete AppStudioProvisionedUser", func() {
		_, err := fw.SandboxController.DeleteUserSignup(utilsUpgrade.AppStudioProvisionedUser)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Delete AppStudioDeactivatedUser", func() {
		_, err := fw.SandboxController.DeleteUserSignup(utilsUpgrade.DeactivatedUser)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Delete AppStudioBannedUser", func() {
		_, err := fw.SandboxController.DeleteUserSignup(utilsUpgrade.BannedUser)
		Expect(err).NotTo(HaveOccurred())
	})

})
