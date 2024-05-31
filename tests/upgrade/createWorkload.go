package upgrade

import (
	"github.com/konflux-ci/e2e-tests/tests/upgrade/create"
	"github.com/konflux-ci/e2e-tests/tests/upgrade/utils"

	"github.com/konflux-ci/e2e-tests/pkg/framework"
	. "github.com/onsi/ginkgo/v2"
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
