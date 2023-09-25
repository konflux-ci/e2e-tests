package verify

import (
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	sandbox "github.com/redhat-appstudio/e2e-tests/pkg/sandbox"
	utils "github.com/redhat-appstudio/e2e-tests/tests/upgrade/utils"
)

func VerifyAppStudioProvisionedSpace(fw *framework.Framework) {
}

func VerifyAppStudioProvisionedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.CheckUserCreated(utils.AppStudioProvisionedUser)
	Expect(err).NotTo(HaveOccurred())
}

func VerifyAppStudioDeactivatedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.CheckUserCreatedWithSignUp(utils.DeactivatedUser, sandbox.GetUserSignupSpecsDeactivated(utils.DeactivatedUser))
	Expect(err).NotTo(HaveOccurred())
}

func VerifyAppStudioBannedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.CheckUserCreatedWithSignUp(utils.BannedUser, sandbox.GetUserSignupSpecsBanned(utils.BannedUser))
	Expect(err).NotTo(HaveOccurred())
}
