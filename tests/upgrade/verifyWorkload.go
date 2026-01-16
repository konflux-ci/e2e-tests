package upgrade

import (
	"github.com/konflux-ci/e2e-tests/tests/upgrade/utils"
	"github.com/konflux-ci/e2e-tests/tests/upgrade/verify"

	"github.com/konflux-ci/e2e-tests/pkg/framework"
	ginkgo "github.com/onsi/ginkgo/v2"
)

var _ = framework.UpgradeSuiteDescribe("Create users and check their state", ginkgo.Label("upgrade-verify"), func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework

	ginkgo.BeforeAll(func() {
		fw, _ = utils.PrepareForUpgradeTests()
	})

	ginkgo.It("Verify AppStudioProvisionedUser", func() {
		verify.VerifyAppStudioProvisionedUser(fw)
	})

	ginkgo.It("creates AppStudioDeactivatedUser", func() {
		verify.VerifyAppStudioDeactivatedUser(fw)
	})

	ginkgo.It("creates AppStudioBannedUser", func() {
		verify.VerifyAppStudioBannedUser(fw)
	})

})
