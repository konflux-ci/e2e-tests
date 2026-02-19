package build

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/build-service/controllers"
	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck // ST1001 - Ginkgo DSL convention
	. "github.com/onsi/gomega"    //nolint:staticcheck // ST1001 - Gomega DSL convention

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

	Describe("test build annotations", Label("github", "annotations"), Ordered, func() {
		var testNamespace, componentName, applicationName string
		var componentObj appservice.ComponentSpec
		var component *appservice.Component
		var buildPipelineAnnotation map[string]string
		invalidAnnotation := "foo"

		BeforeAll(func() {
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).ShouldNot(HaveOccurred())
			testNamespace = f.UserNamespace

			applicationName = fmt.Sprintf("build-suite-test-application-%s", util.GenerateRandomString(4))
			_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			componentName = fmt.Sprintf("%s-%s", "test-annotations", util.GenerateRandomString(6))

			// get the build pipeline bundle annotation
			buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

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

		When("component is created with invalid build request annotations", func() {

			invalidBuildAnnotation := map[string]string{
				controllers.BuildRequestAnnotationName: invalidAnnotation,
			}

			BeforeAll(func() {
				componentObj = appservice.ComponentSpec{
					ComponentName: componentName,
					Application:   applicationName,
					Source: appservice.ComponentSource{
						ComponentSourceUnion: appservice.ComponentSourceUnion{
							GitSource: &appservice.GitSource{
								URL:           annotationsTestGitHubURL,
								Revision:      annotationsTestRevision,
								DockerfileURL: constants.DockerFilePath,
							},
						},
					},
				}

				component, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(invalidBuildAnnotation, buildPipelineAnnotation))
				Expect(component).ToNot(BeNil())
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("handles invalid request annotation", func() {

				expectedInvalidAnnotationMessage := fmt.Sprintf("unexpected build request: %s", invalidAnnotation)

				// Waiting for 1 minute to see if any pipelinerun is triggered
				Consistently(func() bool {
					_, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
					Expect(err).To(HaveOccurred())
					return strings.Contains(err.Error(), "no pipelinerun found")
				}, time.Minute*1, constants.PipelineRunPollingInterval).Should(BeTrue(), "timeout while checking if any pipelinerun is triggered")

				buildStatus := &controllers.BuildStatus{}
				Eventually(func() error {
					component, err = f.AsKubeAdmin.HasController.GetComponent(componentName, testNamespace)
					if err != nil {
						return err
					} else if component == nil {
						return fmt.Errorf("got component as nil after getting component %s in namespace %s", componentName, testNamespace)
					}
					buildStatusAnnotationValue := component.Annotations[controllers.BuildStatusAnnotationName]
					GinkgoWriter.Printf(buildStatusAnnotationValueLoggingFormat, buildStatusAnnotationValue)
					statusBytes := []byte(buildStatusAnnotationValue)
					err = json.Unmarshal(statusBytes, buildStatus)
					if err != nil {
						return err
					}
					if !strings.Contains(buildStatus.Message, expectedInvalidAnnotationMessage) {
						return fmt.Errorf("build status message is not as expected, got: %q, expected: %q", buildStatus.Message, expectedInvalidAnnotationMessage)
					}
					return nil
				}, time.Minute*2, 2*time.Second).Should(Succeed(), "failed while checking build status message for component %q is correct after setting invalid annotations", componentName)
			})
		})
	})

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

