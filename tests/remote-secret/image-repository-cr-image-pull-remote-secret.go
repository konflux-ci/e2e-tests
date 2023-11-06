package remotesecret

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	image "github.com/redhat-appstudio/image-controller/api/v1alpha1"
	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
)

/*
 * Component: remote secret
 * Description: SVPI-574 - Ensure existence of image pull remote secret and image pull secret when ImageRepository is created
 *              SVPI-652 - Ensure existence of image push remote secret and image push secret when ImageRepository is created
 * Note: This test covers the preferred approach (ImageRepository CR) that it is already in prod
 * More info: https://github.com/redhat-appstudio/image-controller#general-purpose-image-repository
 */

var _ = framework.RemoteSecretSuiteDescribe(Label("image-repository-cr-image-pull-remote-secret"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var timeout, interval time.Duration

	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	componentList := []*appservice.Component{}
	component := &appservice.Component{}
	imagePullRemoteSecret := &rs.RemoteSecret{}
	imagePushRemoteSecret := &rs.RemoteSecret{}
	pullTargets := []rs.TargetStatus{}
	pushTargets := []rs.TargetStatus{}
	imageRepository := &image.ImageRepository{}
	snapshot := &appservice.Snapshot{}
	env := &appservice.Environment{}

	applicationName := "image-repository-cr-dotnet-component"
	gitSourceUrl := "https://github.com/devfile-samples/devfile-sample-dotnet60-basic"
	environmentName := "image-pull-remote-secret"
	secret := ""

	AfterEach(framework.ReportFailure(&fw))

	Describe("SVPI-574 and SVPI-652 - Ensure existence of image pull remote secret, image push remote secret, image pull secret and image push secret when ImageRepository is created", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("rs-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
			}
		})

		It("creates an application", func() {
			createdApplication, err := fw.AsKubeDeveloper.HasController.CreateApplication(applicationName, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdApplication.Spec.DisplayName).To(Equal(applicationName))
			Expect(createdApplication.Namespace).To(Equal(namespace))
		})

		It("checks if application is healthy", func() {
			Eventually(func() string {
				appstudioApp, err := fw.AsKubeDeveloper.HasController.GetApplication(applicationName, namespace)
				Expect(err).NotTo(HaveOccurred())
				application = appstudioApp

				return application.Status.Devfile
			}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), fmt.Sprintf("timed out waiting for gitOps repository to be created for the %s application in %s namespace", applicationName, fw.UserNamespace))

			Eventually(func() bool {
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("timed out waiting for HAS controller to create gitops repository for the %s application in %s namespace", applicationName, fw.UserNamespace))
		})

		It("creates an environment", func() {
			env, err = fw.AsKubeDeveloper.GitOpsController.CreatePocEnvironment(environmentName, namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates component detection query", func() {
			cdq, err = fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(applicationName, namespace, gitSourceUrl, "", "", secret, false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates component", func() {
			for _, compDetected := range cdq.Status.ComponentDetected {
				c, err := fw.AsKubeDeveloper.HasController.CreateComponentWithoutGenerateAnnotation(compDetected.ComponentStub, namespace, secret, applicationName, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(c.Name).To(Equal(compDetected.ComponentStub.ComponentName))

				componentList = append(componentList, c)
			}

			Expect(componentList).To(HaveLen(1))
			component = componentList[0]

			Expect(component.Annotations["image.redhat.com/generate"]).To(Equal(""))
		})

		It("creates an image repository", func() {
			imageRepository, err = fw.AsKubeAdmin.ImageController.CreateImageRepositoryCR("image-repository", namespace, application.Name, component.Name)
			Expect(err).NotTo(HaveOccurred())
		})

		It("checks if image repository is ready", func() {
			Eventually(func() image.ImageRepositoryState {
				imageRepository, err = fw.AsKubeAdmin.ImageController.GetImageRepositoryCR(imageRepository.Name, imageRepository.Namespace)
				Expect(err).NotTo(HaveOccurred())

				return imageRepository.Status.State
			}, 2*time.Minute, 5*time.Second).Should(Equal(image.ImageRepositoryStateReady), fmt.Sprintf("ImageRepository '%s/%s' is not ready", imageRepository.Name, imageRepository.Namespace))
		})

		It("updates component with generated image from ImageRepository CR", func() {
			Eventually(func() string {
				imageRepository, err = fw.AsKubeAdmin.ImageController.GetImageRepositoryCR(imageRepository.Name, imageRepository.Namespace)
				Expect(err).NotTo(HaveOccurred())

				return imageRepository.Status.Image.URL
			}, 2*time.Minute, 5*time.Second).ShouldNot(BeEmpty(), fmt.Sprintf("ImageRepository '%s/%s' did not generate any image", imageRepository.Name, imageRepository.Namespace))

			component, err = fw.AsKubeDeveloper.HasController.GetComponent(component.GetName(), component.GetNamespace())
			Expect(err).NotTo(HaveOccurred())

			component.Spec.ContainerImage = imageRepository.Status.Image.URL
			err := fw.AsKubeDeveloper.HasController.UpdateComponent(component)
			Expect(err).NotTo(HaveOccurred())
		})

		It("waits for component pipeline to be finished", func() {
			component, err = fw.AsKubeAdmin.HasController.GetComponent(component.Name, namespace)
			Expect(err).ShouldNot(HaveOccurred(), "failed to get component: %v", err)

			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", 2, fw.AsKubeAdmin.TektonController)).To(Succeed())
		})

		It("finds the snapshot and checks if it is marked as successful", func() {
			timeout = time.Second * 600
			interval = time.Second * 10

			Eventually(func() error {
				snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", "", component.Name, namespace)
				if err != nil {
					GinkgoWriter.Println("snapshot has not been found yet")
					return err
				}
				if !fw.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot) {
					return fmt.Errorf("tests haven't succeeded for snapshot %s/%s. snapshot status: %+v", snapshot.GetNamespace(), snapshot.GetName(), snapshot.Status)
				}
				return nil
			}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out waiting for the snapshot for the component %s/%s to be marked as successful", component.GetNamespace(), component.GetName()))

		})

		It("checks if a SnapshotEnvironmentBinding is created successfully", func() {
			Eventually(func() error {
				_, err := fw.AsKubeAdmin.CommonController.GetSnapshotEnvironmentBinding(application.Name, namespace, env)
				if err != nil {
					GinkgoWriter.Println("SnapshotEnvironmentBinding has not been found yet")
					return err
				}
				return nil
			}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out waiting for the SnapshotEnvironmentBinding to be created (snapshot: %s, env: %s, namespace: %s)", snapshot.GetName(), env.GetName(), snapshot.GetNamespace()))
		})

		It("checks if image pull remote secret was created", func() {
			Eventually(func() error {
				imagePullRemoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.GetImageRepositoryRemoteSecret("image-repository-image-pull", applicationName, component.Spec.ComponentName, namespace)

				return err
			}, 5*time.Minute, 5*time.Second).Should(Succeed(), fmt.Sprintf("Image Pull Remote Secret in '%s' was not created", namespace))
		})

		It("checks if image push remote secret was created", func() {
			Eventually(func() error {
				imagePushRemoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.GetImageRepositoryRemoteSecret("image-repository-image-push", applicationName, component.Spec.ComponentName, namespace)

				return err
			}, 5*time.Minute, 5*time.Second).Should(Succeed(), fmt.Sprintf("Image Push Remote Secret in '%s' was not created", namespace))
		})

		It("checks if image pull remote secret was deployed", func() {
			Eventually(func() bool {
				imagePullRemoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.GetRemoteSecret(imagePullRemoteSecret.Name, imagePullRemoteSecret.Namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(imagePullRemoteSecret.Status.Conditions, "Deployed")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("Pull RemoteSecret %s/%s is not in deployed phase", namespace, imagePullRemoteSecret.GetName()))
		})

		It("checks if image push remote secret was deployed", func() {
			Eventually(func() bool {
				imagePushRemoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.GetRemoteSecret(imagePushRemoteSecret.Name, imagePushRemoteSecret.Namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(imagePushRemoteSecret.Status.Conditions, "Deployed")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("Push RemoteSecret %s/%s is not in deployed phase", namespace, imagePushRemoteSecret.GetName()))
		})

		It("checks if image pull secret is set and linked to the default service account", func() {
			pullTargets = imagePullRemoteSecret.Status.Targets
			Expect(pullTargets).To(HaveLen(1))

			IsTargetSecretLinkedToRightSA(namespace, imagePullRemoteSecret.Name, "default", pullTargets[0])
		})

		It("checks if image push secret is set and linked to the appstudio-pipeline service account", func() {
			pushTargets = imagePushRemoteSecret.Status.Targets
			Expect(pushTargets).To(HaveLen(1))

			IsTargetSecretLinkedToRightSA(namespace, imagePushRemoteSecret.Name, "appstudio-pipeline", pushTargets[0])
		})

		It("checks if image pull secret is correct", func() {
			IsRobotAccountTokenCorrect(pullTargets[0].SecretName, namespace, "pull", imageRepository, fw)
		})

		It("checks if image push secret is correct", func() {
			IsRobotAccountTokenCorrect(pushTargets[0].SecretName, namespace, "push", imageRepository, fw)
		})

	})
})
