package build

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck
	. "github.com/onsi/gomega"    //nolint:staticcheck

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
)

var _ = framework.BuildSuiteDescribe("Build service E2E tests", Label("build-service"), func() {

	var f *framework.Framework
	AfterEach(framework.ReportFailure(&f))
	var err error
	defer GinkgoRecover()

	Describe("Creating component with container image source", Label("github"), Ordered, func() {
		var applicationName, componentName, testNamespace string
		var timeout time.Duration
		var buildPipelineAnnotation map[string]string

		BeforeAll(func() {
			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			componentName = fmt.Sprintf("build-suite-test-component-image-source-%s", util.GenerateRandomString(6))
			outputContainerImage := ""
			timeout = time.Second * 10

			// get the build pipeline bundle annotation before creating the component
			buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

			// Create a component with containerImageSource being defined
			component := appservice.ComponentSpec{
				ComponentName:  componentName,
				ContainerImage: containerImageSource,
			}
			_, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(component, testNamespace, outputContainerImage, "", applicationName, true, buildPipelineAnnotation)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*2)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*2)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
			}
		})

		It("should not trigger a PipelineRun", func() {
			Consistently(func() bool {
				_, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				Expect(err).To(HaveOccurred())
				return strings.Contains(err.Error(), "no pipelinerun found")
			}, timeout, constants.PipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("expected no PipelineRun to be triggered for the component %s in %s namespace", componentName, testNamespace))
		})
	})
})
