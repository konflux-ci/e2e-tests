package build

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/has"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	klog "k8s.io/klog/v2"
)

const (
	containerImageSource   = "quay.io/redhat-appstudio/e2e-tests:latest"
	gitSourceURL           = "https://github.com/devfile-samples/devfile-sample-python-basic"
	dummyPipelineBundleRef = "quay.io/redhat-appstudio-qe/dummy-pipeline-bundle:latest"
)

var _ = framework.BuildSuiteDescribe("Build Service E2E tests", func() {

	defer GinkgoRecover()

	var applicationName, componentName, appStudioE2EApplicationsNamespace, outputContainerImage string
	var pipelineRun v1beta1.PipelineRun
	var component *appservice.Component
	var timeout, interval time.Duration

	var customBundleConfigMap, defaultBundleConfigMap *v1.ConfigMap
	var defaultBundleRef, customBundleRef string

	// Initialize the tests controllers
	f, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		applicationName = fmt.Sprintf("build-suite-test-application-%s", util.GenerateRandomString(10))
		appStudioE2EApplicationsNamespace = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, "appstudio-e2e-test")

		_, err := f.HasController.CreateTestNamespace(appStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", appStudioE2EApplicationsNamespace, err)

		_, err = f.HasController.CreateHasApplication(applicationName, appStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(f.HasController.DeleteHasApplication, applicationName, appStudioE2EApplicationsNamespace)

	})

	When("component with container image source is created", func() {
		BeforeAll(func() {
			componentName = fmt.Sprintf("build-suite-test-component-image-source-%s", util.GenerateRandomString(10))
			outputContainerImage = ""
			timeout = time.Second * 10
			interval = time.Second * 1
			// Create a component with containerImageSource being defined
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, "", containerImageSource, outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace)
		})
		It("should not trigger a PipelineRun", func() {
			Consistently(func() bool {
				pipelineRun, err = f.HasController.GetComponentPipeline(component.Name, applicationName, appStudioE2EApplicationsNamespace)
				Expect(pipelineRun.Name).To(BeEmpty())

				return strings.Contains(err.Error(), "no pipelinerun found")
			}, timeout, interval).Should(BeTrue(), "expected the PipelineRun not to be triggered")
		})
	})
	When("component with git source is created", func() {
		BeforeAll(func() {
			componentName = fmt.Sprintf("build-suite-test-component-git-source-%s", util.GenerateRandomString(10))
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", has.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			timeout = time.Second * 60
			interval = time.Second * 1
			// Create a component with Git Source URL being defined
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace)

			customBundleConfigMap, err = f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, appStudioE2EApplicationsNamespace)
			if err != nil {
				if errors.IsNotFound(err) {
					klog.Infof("configmap with custom pipeline bundle not found in %s namespace\n", appStudioE2EApplicationsNamespace)
				} else {
					Fail(fmt.Sprintf("error occured when trying to get configmap %s in %s namespace: %v", constants.BuildPipelinesConfigMapName, appStudioE2EApplicationsNamespace, err))
				}
			} else {
				customBundleRef = customBundleConfigMap.Data["default_build_bundle"]
			}

			defaultBundleConfigMap, err = f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace)
			if err != nil {
				if errors.IsForbidden(err) {
					klog.Infof("don't have enough permissions to get a configmap with default pipeline in %s namespace\n", constants.BuildPipelinesConfigMapDefaultNamespace)
				} else {
					Fail(fmt.Sprintf("error occured when trying to get configmap %s in %s namespace: %v", constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace, err))
				}
			} else {
				defaultBundleRef = defaultBundleConfigMap.Data["default_build_bundle"]
			}

		})
		It("triggers a PipelineRun", func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipeline(componentName, applicationName, appStudioE2EApplicationsNamespace)
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
		})

		It("should reference the custom pipeline bundle in a PipelineRun", func() {
			if customBundleRef == "" {
				Skip("skipping the specs - custom pipeline bundle is not defined")
			}
			pipelineRun, err := f.HasController.GetComponentPipeline(componentName, applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(customBundleRef))
		})

		It("should reference the default pipeline bundle in a PipelineRun", func() {
			if customBundleRef != "" {
				Skip("skipping - custom pipeline bundle bundle (that overrides the default one) is defined")
			}
			if defaultBundleRef == "" {
				Skip("skipping - default pipeline bundle cannot be fetched")
			}
			pipelineRun, err := f.HasController.GetComponentPipeline(componentName, applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(defaultBundleRef))
		})

		When("the PipelineRun has started", func() {
			BeforeAll(func() {
				timeout = time.Second * 600
				interval = time.Second * 10
			})
			It("should eventually finish successfully", func() {
				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipeline(componentName, applicationName, appStudioE2EApplicationsNamespace)
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

		})

	})

	When("a configmap with 'dummy' custom pipeline bundle is created in the testing namespace", func() {
		BeforeAll(func() {
			if customBundleRef != "" {
				Skip("skipping the specs - a custom pipeline bundle (that overrides the default one) is defined")
			}
			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: constants.BuildPipelinesConfigMapName},
				Data:       map[string]string{"default_build_bundle": dummyPipelineBundleRef},
			}
			_, err = f.CommonController.CreateConfigMap(cm, appStudioE2EApplicationsNamespace)
			Expect(err).ToNot(HaveOccurred())

			componentName = "build-suite-test-component-pipeline-reference"
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace)
			DeferCleanup(f.CommonController.DeleteConfigMap, constants.BuildPipelinesConfigMapName, appStudioE2EApplicationsNamespace)
		})
		It("should be referenced in another PipelineRun", func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipeline(componentName, applicationName, appStudioE2EApplicationsNamespace)
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.HasController.GetComponentPipeline(componentName, applicationName, appStudioE2EApplicationsNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(dummyPipelineBundleRef))
		})
	})

})
