package common

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func NewFramework(workspace string) *framework.Framework {
	var fw *framework.Framework
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
	time.AfterFunc(240*time.Minute, func() {
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
			Source: appstudioApi.ComponentSource{
				appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						Revision: gitSourceRevision,
						URL:      gitSourceURL,
					},
				},
			},
		},
	}

	if componentName2 != "" && containerImage2 != "" {
		newSnapshotComponent := appstudioApi.SnapshotComponent{
			Name:           componentName2,
			ContainerImage: containerImage2,
			Source: appstudioApi.ComponentSource{
				appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						Revision: gitSourceRevision2,
						URL:      gitSourceURL2,
					},
				},
			},
		}
		snapshotComponents = append(snapshotComponents, newSnapshotComponent)
	}

	snapshotName := "snapshot-sample-" + util.GenerateRandomString(4)

	return fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithComponents(snapshotName, componentName, applicationName, namespace, snapshotComponents)
}

func CheckReleaseStatus(releaseCR *releaseApi.Release) error {
	GinkgoWriter.Println("releaseCR: %s", releaseCR.Name)
	conditions := releaseCR.Status.Conditions
	GinkgoWriter.Println("len of conditions: %d", len(conditions))
	if len(conditions) > 0 {
		for _, c := range conditions {
			GinkgoWriter.Println("type of c: %s", c.Type)
			if c.Type == "Released" {
				GinkgoWriter.Println("status of c: %s", c.Status)
				if c.Status == "True" {
					GinkgoWriter.Println("Release CR is released")
					return nil
				} else if c.Status == "False" && c.Reason == "Progressing" {
					return fmt.Errorf("release %s/%s is in progressing", releaseCR.GetNamespace(), releaseCR.GetName())
				} else {
					GinkgoWriter.Println("Release CR failed/skipped")
					Expect(string(c.Status)).To(Equal("True"), fmt.Sprintf("Release %s failed/skipped", releaseCR.Name))
					return nil
				}
			}
		}
	}
	return nil
}

// CreateOpaqueSecret creates a k8s Secret in a workspace if it doesn't exist.
// It populates the Secret data fields based on the mapping of fields to
// environment variables containing the base64 encoded field data.
func CreateOpaqueSecret(
	fw *framework.Framework,
	namespace, secretName string,
	fieldEnvMap map[string]string,
) {
	secretData := make(map[string][]byte)

	for field, envVar := range fieldEnvMap {
		envValue := os.Getenv(envVar)
		Expect(envValue).ToNot(BeEmpty())

		decodedValue, err := base64.StdEncoding.DecodeString(envValue)
		Expect(err).ToNot(HaveOccurred())

		secretData[field] = decodedValue
	}

	secret, err := fw.AsKubeAdmin.CommonController.GetSecret(namespace, secretName)
	if secret == nil || errors.IsNotFound(err) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: secretData,
		}

		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(namespace, secret)
		Expect(err).ToNot(HaveOccurred())
	}
}
