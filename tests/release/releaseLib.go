package common

import (
	"os"
	"time"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
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

func CreateComponent(devFw framework.Framework, devNamespace, appName, compName, gitURL, gitRevision, dockerFilePath string, buildPipelineBundle map[string]string) *appservice.Component {
	componentObj := appservice.ComponentSpec{
		ComponentName: compName,
		Application:   appName,
		Source: appservice.ComponentSource{
			ComponentSourceUnion: appservice.ComponentSourceUnion{
				GitSource: &appservice.GitSource{
					URL:           gitURL,
					Revision:      gitRevision,
					DockerfileURL: dockerFilePath,
				},
			},
		},
	}
	component, err := devFw.AsKubeAdmin.HasController.CreateComponent(componentObj, devNamespace, "", "", appName, false, buildPipelineBundle)
	Expect(err).NotTo(HaveOccurred())
	return component
}
