package integration

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	containerImageSource = "quay.io/redhat-appstudio-qe/busybox-loop:latest"
	gitSourceRepoName    = "devfile-sample-python-basic"
	gitSourceURL         = "https://github.com/redhat-appstudio-qe/" + gitSourceRepoName
	bundleURL            = "quay.io/redhat-appstudio/example-test-bundle:build-pipeline-pass"
	inPipelineName       = "component-pipeline-pass"
)

var _ = framework.IntegrationServiceSuiteDescribe("Integration Service E2E tests", Label("integration-service", "HACBS"), func() {
	// TODO Investigate issue with 'invalid memory address or nil pointer dereference' in integration-service
	// Testsuite skipped
	if true {
		return
	}
	defer GinkgoRecover()

	var applicationName, componentName, appStudioE2EApplicationsNamespace, outputContainerImage string
	var timeout, interval time.Duration
	var applicationSnapshot *appstudioApi.Snapshot
	var applicationSnapshot_push *appstudioApi.Snapshot

	var defaultBundleConfigMap *v1.ConfigMap

	// Initialize the tests controllers
	f, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		applicationName = fmt.Sprintf("integ-app-%s", util.GenerateRandomString(4))
		appStudioE2EApplicationsNamespace = utils.GetGeneratedNamespace("integration-e2e")

		_, err := f.CommonController.CreateTestNamespace(appStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", appStudioE2EApplicationsNamespace, err)

		app, err := f.HasController.CreateHasApplication(applicationName, appStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
			Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
		)
		DeferCleanup(f.HasController.DeleteHasApplication, applicationName, appStudioE2EApplicationsNamespace, false)

	})

	When("component with git source is created", Label("slow"), func() {

		BeforeAll(func() {

			componentName = fmt.Sprintf("integration-suite-test-component-git-source-%s", util.GenerateRandomString(4))
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			timeout = time.Minute * 4
			interval = time.Second * 1
			// Create a component with Git Source URL being defined
			_, err := f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace, false)

			defaultBundleConfigMap, err = f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace)
			if err != nil {
				if errors.IsForbidden(err) {
					klog.Infof("don't have enough permissions to get a configmap with default pipeline in %s namespace\n", constants.BuildPipelinesConfigMapDefaultNamespace)
				} else {
					Fail(fmt.Sprintf("error occurred when trying to get configmap %s in %s namespace: %v", constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace, err))
				}
			}
			_ = defaultBundleConfigMap.Data["default_build_bundle"]
			_, err = f.IntegrationController.CreateIntegrationTestScenario(applicationName, appStudioE2EApplicationsNamespace, bundleURL, inPipelineName)
			Expect(err).ShouldNot(HaveOccurred())

		})
		It("triggers a PipelineRun", func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false, "")
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
		})

		When("the PipelineRun has started", Label("integration-service"), func() {
			BeforeAll(func() {
				timeout = time.Second * 600
				interval = time.Second * 10
			})
			It("should eventually finish successfully", func() {
				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false, "")
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						klog.Infof("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if condition.Reason == "Failed" {
							Fail(fmt.Sprintf("Pipelinerun %s has failed", pipelineRun.Name))
						}
					}
					return pipelineRun.IsDone()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})

			It("check if the integrationTestScenario is created", func() {
				testScenarios, err := f.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				for _, testScenario := range *testScenarios {
					klog.Infof("IntegrationTestScenario %s is found", testScenario.Name)
				}

			})

			It("check if the ApplicationSnapshot is created", func() {
				applicationSnapshot, err = f.IntegrationController.GetApplicationSnapshot(applicationName, appStudioE2EApplicationsNamespace, componentName)
				Expect(err).ShouldNot(HaveOccurred())
				klog.Infof("applicationSnapshot %s is found", applicationSnapshot.Name)
			})

			It("check if all of the integrationPipelineRuns passed", Label("slow"), func() {
				integrationTestScenarios, err := f.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				timeout = time.Second * 600
				interval = time.Second * 10
				for _, testScenario := range *integrationTestScenarios {
					Eventually(func() bool {
						Expect(f.IntegrationController.WaitForIntegrationPipelineToBeFinished(&testScenario, applicationSnapshot, applicationName, appStudioE2EApplicationsNamespace)).To(Succeed(), "Error when waiting for a integration pipeline to finish")
						return true
					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
				}
			})

			It("create an applicationSnapshot of push event", func() {
				applicationSnapshot_push, err = f.IntegrationController.CreateApplicationSnapshot(applicationName, appStudioE2EApplicationsNamespace, componentName)
				Expect(err).ShouldNot(HaveOccurred())
				klog.Infof("applicationSnapshot %s is found", applicationSnapshot_push.Name)
			})

			It("check if all of the integrationPipelineRuns created by push event passed", Label("slow"), func() {
				integrationTestScenarios, err := f.IntegrationController.GetIntegrationTestScenarios(applicationName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())

				for _, testScenario := range *integrationTestScenarios {
					timeout = time.Second * 60
					interval = time.Second * 2
					Eventually(func() bool {
						pipelineRun, err := f.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot_push.Name, appStudioE2EApplicationsNamespace)
						if err != nil {
							klog.Infof("cannot get the Integration PipelineRun: %v", err)
							return false
						}
						return pipelineRun.HasStarted()
					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
					timeout = time.Second * 600
					interval = time.Second * 10
					Eventually(func() bool {
						pipelineRun, err := f.IntegrationController.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot_push.Name, appStudioE2EApplicationsNamespace)
						Expect(err).ShouldNot(HaveOccurred())

						for _, condition := range pipelineRun.Status.Conditions {
							klog.Infof("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)
							if condition.Reason == "Failed" {
								Fail(fmt.Sprintf("Pipelinerun %s has failed", pipelineRun.Name))
							}
						}
						return pipelineRun.IsDone()
					}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
				}
			})

		})
	})
})
