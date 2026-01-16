package verify

import (
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	sandbox "github.com/konflux-ci/e2e-tests/pkg/sandbox"
	utils "github.com/konflux-ci/e2e-tests/tests/upgrade/utils"
	gomega "github.com/onsi/gomega"
)

func VerifyAppStudioProvisionedSpace(fw *framework.Framework) {
}

func VerifyAppStudioProvisionedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.CheckUserCreated(utils.AppStudioProvisionedUser)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func VerifyAppStudioDeactivatedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.CheckUserCreatedWithSignUp(utils.DeactivatedUser, sandbox.GetUserSignupSpecsDeactivated(utils.DeactivatedUser))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func VerifyAppStudioBannedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.CheckUserCreatedWithSignUp(utils.BannedUser, sandbox.GetUserSignupSpecsBanned(utils.BannedUser))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
