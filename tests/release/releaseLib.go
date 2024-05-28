package common

import (
	"os"
	"time"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	. "github.com/onsi/gomega"
)

func NewFramework(workspace string) *framework.Framework {
	stageOptions := utils.Options{
		ToolchainApiUrl: os.Getenv(constants.TOOLCHAIN_API_URL_ENV),
		KeycloakUrl:     os.Getenv(constants.KEYLOAK_URL_ENV),
		OfflineToken:    os.Getenv(constants.OFFLINE_TOKEN_ENV),
	}
	fw, err := framework.NewFrameworkWithTimeout(
		workspace,
		time.Minute*60,
		stageOptions,
	)
	Expect(err).NotTo(HaveOccurred())
	return fw
}
