package common

import (
	"os"
	"time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

func CreateComponentByCDQ(devFw framework.Framework, devNamespace, managedNamespace, appName, compName string, sourceGitURL string) *appservice.Component {
	// using cdq since git ref is not known
	var componentDetected appservice.ComponentDetectionDescription
	cdq, err := devFw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(compName, devNamespace, sourceGitURL, "", "", "", false)
	Expect(err).NotTo(HaveOccurred())
	Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "Expected length of the detected Components was not 1")

	for _, compDetected := range cdq.Status.ComponentDetected {
		componentDetected = compDetected
	}

	component, err := devFw.AsKubeAdmin.HasController.CreateComponent(componentDetected.ComponentStub, devNamespace, "", "", appName, false, map[string]string{})
	Expect(err).NotTo(HaveOccurred())
	GinkgoWriter.Println("component : ", component.Name)
	Expect(err).ShouldNot(HaveOccurred())
	return component
}

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
