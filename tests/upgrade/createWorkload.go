package upgrade

import (
	"github.com/redhat-appstudio/e2e-tests/tests/upgrade/create"
	"github.com/redhat-appstudio/e2e-tests/tests/upgrade/utils"

	. "github.com/onsi/ginkgo/v2"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.UpgradeSuiteDescribe("Create users and check their state", Label("upgrade-create"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework

	BeforeAll(func() {
		fw, _ = utils.PrepareForUpgradeTests()
	})

	It("creates AppStudioProvisionedUser", func() {
		create.CreateAppStudioProvisionedUser(fw)
	})

	It("creates AppStudioDeactivatedUser", func() {
		create.CreateAppStudioDeactivatedUser(fw)
	})

	It("creates AppStudioBannedUser", func() {
		create.CreateAppStudioBannedUser(fw)
	})

})
