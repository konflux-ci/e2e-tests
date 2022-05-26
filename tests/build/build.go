package build

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	klog "k8s.io/klog/v2"

	routev1 "github.com/openshift/api/route/v1"
)

const (
	containerImageSource   = "quay.io/redhat-appstudio-qe/busybox-loop:latest"
	gitSourceRepoName      = "devfile-sample-hello-world"
	gitSourceURL           = "https://github.com/redhat-appstudio-qe/" + gitSourceRepoName
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

	var webhookRoute *routev1.Route
	var webhookURL string

	// Initialize the tests controllers
	f, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		applicationName = fmt.Sprintf("build-suite-test-application-%s", util.GenerateRandomString(4))
		appStudioE2EApplicationsNamespace = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, "appstudio-e2e-test")

		_, err := f.CommonController.CreateTestNamespace(appStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", appStudioE2EApplicationsNamespace, err)

		_, err = f.HasController.CreateHasApplication(applicationName, appStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(f.HasController.DeleteHasApplication, applicationName, appStudioE2EApplicationsNamespace)

	})

	When("component with container image source is created", func() {
		BeforeAll(func() {
			componentName = fmt.Sprintf("build-suite-test-component-image-source-%s", util.GenerateRandomString(4))
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
				pipelineRun, err = f.HasController.GetComponentPipelineRun(component.Name, applicationName, appStudioE2EApplicationsNamespace, false)
				Expect(pipelineRun.Name).To(BeEmpty())

				return strings.Contains(err.Error(), "no pipelinerun found")
			}, timeout, interval).Should(BeTrue(), "expected the PipelineRun not to be triggered")
		})
	})
	When("component with git source is created", Label("slow"), func() {

		BeforeAll(func() {
			componentName = fmt.Sprintf("build-suite-test-component-git-source-%s", util.GenerateRandomString(4))
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			timeout = time.Second * 60
			interval = time.Second * 1
			// Create a component with Git Source URL being defined
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, defaultGitSourceURL, "", outputContainerImage, "")
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
		It("triggers a PipelineRun", Label("build-templates-e2e"), func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
		})

		It("should reference the custom pipeline bundle in a PipelineRun", Label("build-templates-e2e"), func() {
			if customBundleRef == "" {
				Skip("skipping the specs - custom pipeline bundle is not defined")
			}
			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
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
			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(defaultBundleRef))
		})

		When("the PipelineRun has started", Label("build-templates-e2e"), func() {
			BeforeAll(func() {
				timeout = time.Second * 600
				interval = time.Second * 10
			})
			It("should eventually finish successfully", func() {
				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
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

		When("the PipelineRun is finished", func() {
			var webhookName string

			BeforeAll(func() {
				timeout = time.Minute * 5
				interval = time.Second * 10
				webhookName = "el" + componentName

			})

			It("eventually leads to a creation of a component webhook (event listener)", Label("webhook", "slow"), func() {
				Eventually(func() bool {
					_, err := f.HasController.GetEventListenerRoute(componentName, appStudioE2EApplicationsNamespace)
					if err != nil {
						klog.Infof("component webhook %s has not been created yet in %s namespace\n", webhookName, appStudioE2EApplicationsNamespace)
						return false
					}
					return true

				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the component webhook to be created")
			})
		})

		When("the container image is created and pushed to container registry", func() {
			It("contains non-empty sbom files", Label("build-templates-e2e", "sbom", "slow"), func() {
				component, err := f.HasController.GetHasComponent(componentName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				purl, cyclonedx, err := build.GetParsedSbomFilesContentFromImage(component.Spec.ContainerImage)
				Expect(err).NotTo(HaveOccurred())
				Expect(purl.ImageContents.Dependencies).ToNot(BeEmpty())
				Expect(cyclonedx.Components).ToNot(BeEmpty())
			})
		})
		When("the component event listener is created", Label("webhook", "slow"), func() {
			BeforeAll(func() {
				timeout = time.Minute * 1
				interval = time.Second * 1

				webhookRoute, err = f.HasController.GetEventListenerRoute(componentName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(webhookRoute.Spec.Host).ShouldNot(BeEmpty())

				if webhookRoute.Spec.TLS != nil {
					webhookURL = fmt.Sprintf("https://%s", webhookRoute.Spec.Host)
				} else {
					webhookURL = fmt.Sprintf("http://%s", webhookRoute.Spec.Host)
				}
			})

			It("should be eventually ready to receive events from Github", func() {
				Eventually(func() bool {
					c := http.Client{}
					req, err := http.NewRequestWithContext(context.Background(), "GET", webhookURL, bytes.NewReader([]byte("{}")))
					if err != nil {
						return false
					}
					resp, err := c.Do(req)
					if err != nil || resp.StatusCode > 299 {
						return false
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the event listener to be ready")
			})
		})

		When("the component webhook is configured on Github repository for 'push' events and a change is pushed", Label("webhook", "slow"), func() {
			var initialContainerImageURL, newContainerImageURL string
			var webhookID int64

			BeforeAll(func() {
				if utils.IsPrivateHostname(webhookRoute.Spec.Host) {
					Skip("Using private cluster (not reachable from Github), skipping...")
				}

				timeout = time.Minute * 2
				interval = time.Second * 5

				// Get Component CR to check the current value of Container Image
				component, err := f.HasController.GetHasComponent(componentName, appStudioE2EApplicationsNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				initialContainerImageURL = component.Spec.ContainerImage

				webhookID, err = f.CommonController.Github.CreateWebhook(gitSourceRepoName, webhookURL)
				Expect(err).ShouldNot(HaveOccurred())
				DeferCleanup(f.CommonController.Github.DeleteWebhook, gitSourceRepoName, webhookID)

				// Wait until EventListener's pod is ready to receive events
				err = f.CommonController.WaitForPodSelector(f.CommonController.IsPodRunning, appStudioE2EApplicationsNamespace, "eventlistener", componentName, 60, 100)
				Expect(err).NotTo(HaveOccurred())

				// Update a file in Component source Github repo to trigger a push event
				updatedFile, err := f.CommonController.Github.UpdateFile(gitSourceRepoName, "README.md", "test")
				Expect(err).NotTo(HaveOccurred())

				// A new output container image URL should contain the suffix with the latest commit SHA
				newContainerImageURL = fmt.Sprintf("%s-%s", initialContainerImageURL, updatedFile.GetSHA())
			})

			It("eventually leads to triggering another PipelineRun", func() {
				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, true)
					if err != nil {
						klog.Infoln("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			})
			It("PipelineRun should eventually finish", func() {
				timeout = time.Minute * 5
				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, true)
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
			It("ContainerImage field should be updated with the SHA of the commit that was pushed to the Component source repo", func() {
				Eventually(func() bool {
					component, err := f.HasController.GetHasComponent(componentName, appStudioE2EApplicationsNamespace)
					if err != nil {
						klog.Infoln("component was not updated yet")
						return false
					}
					return component.Spec.ContainerImage == newContainerImageURL
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the Component CR to get updated with a new container image URL")
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
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace)
			DeferCleanup(f.CommonController.DeleteConfigMap, constants.BuildPipelinesConfigMapName, appStudioE2EApplicationsNamespace)
		})
		It("should be referenced in another PipelineRun", func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(dummyPipelineBundleRef))
		})
	})

	When("a secret with dummy quay.io credentials is created in the testing namespace", Label("build-secret", "slow"), func() {
		BeforeAll(func() {

			_, err := f.CommonController.GetSecret(appStudioE2EApplicationsNamespace, constants.RegistryAuthSecretName)
			if err != nil {
				// If we have an error when getting RegistryAuthSecretName, it should be IsNotFound err
				Expect(errors.IsNotFound(err)).To(BeTrue())
			} else {
				Skip("a registry auth secret is already created in testing namespace - skipping....")
			}

			timeout = time.Minute * 1
			interval = time.Second * 5

			dummySecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: constants.RegistryAuthSecretName},
				Type:       v1.SecretTypeDockerConfigJson,
				Data:       map[string][]byte{".dockerconfigjson": []byte("{\"auths\":{\"quay.io\":{\"username\":\"test\",\"password\":\"test\",\"auth\":\"dGVzdDp0ZXN0\",\"email\":\"\"}}}")},
			}

			_, err = f.CommonController.CreateSecret(appStudioE2EApplicationsNamespace, dummySecret)
			Expect(err).ToNot(HaveOccurred())

			componentName = "build-suite-test-secret-overriding"
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace)
			DeferCleanup(f.CommonController.DeleteSecret, appStudioE2EApplicationsNamespace, constants.RegistryAuthSecretName)
		})

		It("should override the shared secret", func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.Workspaces).To(HaveLen(2))
			registryAuthWorkspace := &v1beta1.WorkspaceBinding{
				Name: "registry-auth",
				Secret: &v1.SecretVolumeSource{
					SecretName: "redhat-appstudio-registry-pull-secret",
				},
			}
			Expect(pipelineRun.Spec.Workspaces).To(ContainElement(*registryAuthWorkspace))
		})

		It("should not be possible to push to quay.io repo (PipelineRun should fail)", func() {
			timeout = time.Minute * 5
			interval = time.Second * 5
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, appStudioE2EApplicationsNamespace, false)
				Expect(err).ShouldNot(HaveOccurred())

				for _, condition := range pipelineRun.Status.Conditions {
					klog.Infof("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)
					return condition.Reason == "Failed"
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to fail")
		})
	})

	When("creating a component with a specific container image URL", Label("image repository protection"), func() {
		BeforeEach(func() {
			componentName = fmt.Sprintf("build-suite-test-component-image-url-%s", util.GenerateRandomString(4))
			timeout = time.Second * 10

			DeferCleanup(f.HasController.DeleteHasComponent, componentName, appStudioE2EApplicationsNamespace)
		})
		It("should fail for ContainerImage field set to a protected repository (without an image tag)", func() {
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images-protected", utils.GetQuayIOOrganization())
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func() (string, error) {
				return f.HasController.GetHasComponentConditionStatusMessage(componentName, appStudioE2EApplicationsNamespace)
			}, timeout).Should(ContainSubstring("create failed"), "timed out waiting for the component creation to fail")

		})
		It("should fail for ContainerImage field set to a protected repository followed by a random tag", func() {
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images-protected:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func() (string, error) {
				return f.HasController.GetHasComponentConditionStatusMessage(componentName, appStudioE2EApplicationsNamespace)
			}, timeout).Should(ContainSubstring("create failed"), "timed out waiting for the component creation to fail")
		})
		It("should succeed for ContainerImage field set to a protected repository followed by a namespace prefix + dash + string", func() {
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images-protected:%s-%s", utils.GetQuayIOOrganization(), appStudioE2EApplicationsNamespace, strings.Replace(uuid.New().String(), "-", "", -1))
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func() (string, error) {
				return f.HasController.GetHasComponentConditionStatusMessage(componentName, appStudioE2EApplicationsNamespace)
			}, timeout).Should(ContainSubstring("successfully created"), "timed out waiting for the component creation to succeed")
		})
		It("should succeed for ContainerImage field set to a custom (unprotected) repository without a tag being specified", func() {
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images", utils.GetQuayIOOrganization())
			component, err = f.HasController.CreateComponent(applicationName, componentName, appStudioE2EApplicationsNamespace, gitSourceURL, "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func() (string, error) {
				return f.HasController.GetHasComponentConditionStatusMessage(componentName, appStudioE2EApplicationsNamespace)
			}, timeout).Should(ContainSubstring("successfully created"), "timed out waiting for the component creation to succeed")
		})
	})

})
