package remotesecret

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
)

/*
 * Component: remote secret
 * Description: SVPI-574 - Ensure existence of image pull remote secret and image pull secret when component is created
 * Note: This test covers the current approach (component annotation)
 * More info: https://github.com/redhat-appstudio/image-controller#legacy-deprecated-component-image-repository
 */

var _ = framework.RemoteSecretSuiteDescribe(Label("component-annotation-image-pull-remote-secret"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var timeout, interval time.Duration

	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	componentList := []*appservice.Component{}
	imagePullRemoteSecret := &rs.RemoteSecret{}
	component := &appservice.Component{}
	targets := []rs.TargetStatus{}
	snapshot := &appservice.Snapshot{}
	env := &appservice.Environment{}

	applicationName := "component-annotation-dotnet-component"
	gitSourceUrl := "https://github.com/devfile-samples/devfile-sample-dotnet60-basic"
	environmentName := "image-pull-remote-secret"
	secret := ""

	AfterEach(framework.ReportFailure(&fw))

	Describe("SVPI-601 - Ensure existence of image pull remote secret and image pull secret when component is created", Ordered, func() {
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
				c, err := fw.AsKubeDeveloper.HasController.CreateComponent(compDetected.ComponentStub, namespace, "", secret, applicationName, true, map[string]string{})
				Expect(err).NotTo(HaveOccurred())
				Expect(c.Name).To(Equal(compDetected.ComponentStub.ComponentName))

				componentList = append(componentList, c)
			}

			Expect(componentList).To(HaveLen(1))
			component = componentList[0]

			Expect(component.Annotations["image.redhat.com/generate"]).To(Equal("{\"visibility\": \"public\"}"))
		})

		It("waits for components pipeline to be finished", func() {
			for _, component := range componentList {
				component, err = fw.AsKubeAdmin.HasController.GetComponent(component.GetName(), namespace)
				Expect(err).ShouldNot(HaveOccurred(), "failed to get component: %v", err)

				Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", 2, fw.AsKubeAdmin.TektonController)).To(Succeed())
			}
		})

		It("finds the snapshot and checks if it is marked as successful", func() {
			timeout = time.Second * 600
			interval = time.Second * 10
			for _, component := range componentList {
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
			}
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

		It("checks if image pull remote secret was deployed", func() {
			Eventually(func() bool {
				imagePullRemoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.GetImageRepositoryRemoteSecret(component.Spec.ComponentName+"-pull", applicationName, component.Spec.ComponentName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(imagePullRemoteSecret.Status.Conditions, "Deployed")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("Pull RemoteSecret %s/%s is not in deployed phase", namespace, imagePullRemoteSecret.GetName()))
		})

		It("checks if image pull secret is set and linked to the default service account", func() {
			targets = imagePullRemoteSecret.Status.Targets
			Expect(targets).To(HaveLen(1))

			target := targets[0]
			Expect(target.Namespace).To(Equal(namespace))
			Expect(target.SecretName).To(Equal(imagePullRemoteSecret.Name))
			Expect(target.ServiceAccountNames).To(HaveLen(1))
			Expect(target.ServiceAccountNames[0]).To(Equal("default"))
		})

		It("checks if image pull secret is correct", func() {
			secret, err := fw.AsKubeAdmin.CommonController.GetSecret(namespace, targets[0].SecretName)
			Expect(err).NotTo(HaveOccurred())

			// get robot account name and token from image pull secret
			robotAccountName, robotAccountToken := build.GetRobotAccountInfoFromSecret(secret)

			// get expected robot account token
			expectedRobotAccountToken, err := build.GetRobotAccountToken(robotAccountName)
			Expect(err).ShouldNot(HaveOccurred())

			// ensure secret points to the expected robot account token
			Expect(robotAccountToken).To(Equal(expectedRobotAccountToken))
		})
	})
})
