package create

import (
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	utils "github.com/konflux-ci/e2e-tests/tests/upgrade/utils"
	. "github.com/onsi/gomega"
)

func CreateAppStudioProvisionedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.RegisterSandboxUser(utils.AppStudioProvisionedUser)
	Expect(err).NotTo(HaveOccurred())
}

func CreateAppStudioDeactivatedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.RegisterDeactivatedSandboxUser(utils.DeactivatedUser)
	Expect(err).NotTo(HaveOccurred())
}

func CreateAppStudioBannedUser(fw *framework.Framework) {
	_, err := fw.SandboxController.RegisterBannedSandboxUser(utils.BannedUser)
	Expect(err).NotTo(HaveOccurred())
}
