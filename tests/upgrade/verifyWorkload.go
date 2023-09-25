package upgrade

import (
	"github.com/redhat-appstudio/e2e-tests/tests/upgrade/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/upgrade/verify"

	. "github.com/onsi/ginkgo/v2"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.UpgradeSuiteDescribe("Create users and check their state", Label("upgrade-verify"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework

	BeforeAll(func() {
		fw, _ = utils.PrepareForUpgradeTests()
	})

	It("Verify AppStudioProvisionedUser", func() {
		verify.VerifyAppStudioProvisionedUser(fw)
	})

	It("creates AppStudioDeactivatedUser", func() {
		verify.VerifyAppStudioDeactivatedUser(fw)
	})

	It("creates AppStudioBannedUser", func() {
		verify.VerifyAppStudioBannedUser(fw)
	})

})
