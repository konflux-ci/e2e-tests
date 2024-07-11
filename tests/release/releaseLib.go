package common

import (
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
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

func CreateComponent(devFw framework.Framework, devNamespace, appName, compName, gitURL, gitRevision, contextDir, dockerFilePath string, buildPipelineBundle map[string]string) (component *appservice.Component, baseBranchName, pacBranchName string) {
	pacBranchName = constants.PaCPullRequestBranchPrefix + compName
	baseBranchName = fmt.Sprintf("base-%s", util.GenerateRandomString(6))

	err := devFw.AsKubeAdmin.CommonController.Github.CreateRef(utils.GetRepoName(gitURL), "main", gitRevision, baseBranchName)
	Expect(err).ShouldNot(HaveOccurred())

	componentObj := appservice.ComponentSpec{
		ComponentName: compName,
		Application:   appName,
		Source: appservice.ComponentSource{
			ComponentSourceUnion: appservice.ComponentSourceUnion{
				GitSource: &appservice.GitSource{
					URL:           gitURL,
					Revision:      baseBranchName,
					Context:       contextDir,
					DockerfileURL: dockerFilePath,
				},
			},
		},
	}

	component, err = devFw.AsKubeAdmin.HasController.CreateComponent(componentObj, devNamespace, "", "", appName, true, utils.MergeMaps(constants.ComponentPaCRequestAnnotation, buildPipelineBundle))
	Expect(err).NotTo(HaveOccurred())
	return
}
