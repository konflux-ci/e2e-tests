package build

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/has"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

const (
	containerImageSource = "quay.io/redhat-appstudio/e2e-tests:latest"
	gitSourceURL         = "https://github.com/devfile-samples/devfile-sample-python-basic"
)

var _ = framework.BuildSuiteDescribe("Build Service E2E tests", func() {

	defer GinkgoRecover()

	var applicationName, componentName, appStudioE2EApplicationsNamespace, outputContainerImage string
	var component *appservice.Component
	var timeout, interval time.Duration

	// Initialize the tests controllers
	f, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		applicationName = "build-suite-test-application"
		appStudioE2EApplicationsNamespace = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, "appstudio-e2e-test")

		_, err := f.HasController.CreateTestNamespace(appStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", appStudioE2EApplicationsNamespace, err)

		_, err = f.HasController.CreateHasApplication(applicationName, appStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Cleanup of Application CR
		DeferCleanup(f.HasController.DeleteHasApplication, applicationName, appStudioE2EApplicationsNamespace)
	})

	When("component with container image source is created", func() {
		BeforeAll(func() {
			componentName = "build-suite-test-component-image-source"
			outputContainerImage = ""
			timeout = time.Second * 10
			interval = time.Second * 1
			// Create a component with containerImageSource being defined
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, "", containerImageSource, outputContainerImage)
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace)
		})
		It("should not trigger a PipelineRun", func() {
			Consistently(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipeline(component.Name, applicationName, appStudioE2EApplicationsNamespace)
				Expect(pipelineRun.Name).To(BeEmpty())

				return strings.Contains(err.Error(), "no pipelinerun found")
			}, timeout, interval).Should(BeTrue(), "expected the PipelineRun not to be triggered")
		})
	})
	When("component with git source is created", func() {
		BeforeAll(func() {
			componentName = "build-suite-test-component-git-source"
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", has.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			timeout = time.Second * 60
			interval = time.Second * 1
			// Create a component with Git Source URL being defined
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage)
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace)
		})
		It("should trigger a PipelineRun", func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipeline(componentName, applicationName, appStudioE2EApplicationsNamespace)
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
		})
		Context("the PipelineRun is running", func() {
			BeforeAll(func() {
				timeout = time.Second * 600
				interval = time.Second * 10
			})
			It("should finish successfully", func() {
				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipeline(componentName, applicationName, appStudioE2EApplicationsNamespace)
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if condition.Reason == "Failed" {
							Fail(fmt.Sprintf("Pipelinerun %s has failed", pipelineRun.Name))
						}
					}
					return pipelineRun.IsDone()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})

		})

	})
})
