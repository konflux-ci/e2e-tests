package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	"gopkg.in/yaml.v2"
)

var _ = framework.E2ESuiteDescribe("magic", func() {
	defer GinkgoRecover()

	// Initialize the tests controllers /home/flacatusu/WORKSPACE/appstudio-qe/e2e-tests/tests/e2e-demos/config
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())
	configTest, err := LoadTestGeneratorConfig()
	Expect(err).NotTo(HaveOccurred())

	// Initialize the application struct
	application := &appservice.Application{}

	for _, appTest := range configTest.Tests {
		appTest := appTest

		It("Create Application", func() {
			createdApplication, err := framework.HasController.CreateHasApplication(appTest.ApplicationName, AppStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdApplication.Spec.DisplayName).To(Equal(appTest.ApplicationName))
			Expect(createdApplication.Namespace).To(Equal(AppStudioE2EApplicationsNamespace))
		})

		It("Application Health", func() {
			Eventually(func() string {
				appstudioApp, err := framework.HasController.GetHasApplication(appTest.ApplicationName, AppStudioE2EApplicationsNamespace)
				Expect(err).NotTo(HaveOccurred())
				application = appstudioApp

				return application.Status.Devfile
			}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return framework.HasController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
		})

		It("Component Deployment", func() {
			for _, component := range appTest.Components {
				var containerIMG = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

				component, err := framework.HasController.CreateComponent(application.Name, component.Name, AppStudioE2EApplicationsNamespace, QuarkusDevfileSource, "", containerIMG, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(component.Name).To(Equal(component.Name))
				err = framework.HasController.WaitForComponentPipelineToBeFinished(component.Name, application.Name, AppStudioE2EApplicationsNamespace)
				Expect(err).NotTo(HaveOccurred(), "Failed component pipeline %v", err)
			}
		})
	}
})

func LoadTestGeneratorConfig() (config.WorkflowSpec, error) {
	c := config.WorkflowSpec{}
	// Open config file
	file, err := os.Open(filepath.Clean("/home/flacatusu/WORKSPACE/appstudio-qe/e2e-tests/tests/e2e-demos/config/default.yaml"))
	if err != nil {
		return c, err
	}

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	return c, nil
}
