package common

import (
	"fmt"
	"os"
	"time"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	. "github.com/onsi/gomega"
)

func NewFramework(workspace string) *framework.Framework {
	var fw  *framework.Framework
	var err error
	stageOptions := utils.Options{
		ToolchainApiUrl: os.Getenv(constants.TOOLCHAIN_API_URL_ENV),
		KeycloakUrl:     os.Getenv(constants.KEYLOAK_URL_ENV),
		OfflineToken:    os.Getenv(constants.OFFLINE_TOKEN_ENV),
	}
	fw, err = framework.NewFrameworkWithTimeout(
		workspace,
		time.Minute*60,
		stageOptions,
	)
	Expect(err).NotTo(HaveOccurred())

	// Create a ticker that ticks every 3 minutes
	ticker := time.NewTicker(3 * time.Minute)
	// Schedule the stop of the ticker after 30 minutes
	time.AfterFunc(60*time.Minute, func() {
		ticker.Stop()
		fmt.Println("Stopped executing every 3 minutes.")
	})
	// Run a goroutine to handle the ticker ticks
	go func() {
		for range ticker.C {
			fw, err = framework.NewFrameworkWithTimeout(
				workspace,
				time.Minute*60,
				stageOptions,
			)
			Expect(err).NotTo(HaveOccurred())
		}
	}()
	return fw
}

func CreateComponent(devFw framework.Framework, devNamespace, appName, compName, gitURL, gitRevision, contextDir, dockerFilePath string, buildPipelineBundle map[string]string) *appservice.Component {
	componentObj := appservice.ComponentSpec{
		ComponentName: compName,
		Application:   appName,
		Source: appservice.ComponentSource{
			ComponentSourceUnion: appservice.ComponentSourceUnion{
				GitSource: &appservice.GitSource{
					URL:           gitURL,
					Revision:      gitRevision,
					Context:       contextDir,
					DockerfileURL: dockerFilePath,
				},
			},
		},
	}
	component, err := devFw.AsKubeAdmin.HasController.CreateComponent(componentObj, devNamespace, "", "", appName, true, buildPipelineBundle)
	Expect(err).NotTo(HaveOccurred())
	return component
}

// CreateSnapshotWithImageSource creates a snapshot having two images and sources.
func CreateSnapshotWithImageSource(fw framework.Framework, componentName, applicationName, namespace, containerImage, gitSourceURL, gitSourceRevision, componentName2, containerImage2, gitSourceURL2, gitSourceRevision2 string) (*appstudioApi.Snapshot, error) {
	snapshotComponents := []appstudioApi.SnapshotComponent{
		{
			Name:           componentName,
			ContainerImage: containerImage,
			Source:         appstudioApi.ComponentSource{
				appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						Revision: gitSourceRevision,
						URL: gitSourceURL,
					},
				},
			},
		},
	}

	if componentName2 != "" && containerImage2 != "" {
		newSnapshotComponent := appstudioApi.SnapshotComponent{
			Name:           componentName2,
			ContainerImage: containerImage2,
			Source:         appstudioApi.ComponentSource{
				appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						Revision: gitSourceRevision2,
						URL: gitSourceURL2,
					},
				},
			},
		}
		snapshotComponents = append(snapshotComponents, newSnapshotComponent)
	}

	snapshotName := "snapshot-sample-" + util.GenerateRandomString(4)

	return fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithComponents(snapshotName, componentName, applicationName, namespace, snapshotComponents)
}
