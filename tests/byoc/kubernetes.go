package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	client "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/vcluster"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/apis"
)

const (
	QuarkusDevfileSource                   string = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"
	ManagedEnvironmentType                 string = "managed-gitops.redhat.com/managed-environment"
	KubernetesManagedEnvironmentSecretName string = "kubeconfig-vcluster"
	KubernetesEnvironmentName              string = "development"
)

var (
	applicationName = utils.GetGeneratedNamespace("byoc-app")
	componentName   = utils.GetGeneratedNamespace("byoc-comp")
)

var _ = framework.E2ESuiteDescribe(Label("byoc", "kubernetes"), Ordered, func() {
	defer GinkgoRecover()

	var vc vcluster.Vcluster
	var ephemeralClusterClient *kubernetes.Clientset
	var fw *framework.Framework
	var stagingKubeconfig string
	var kubernetesApiServerUrl string

	rootPath, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	// Initialize the application struct
	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	compDetected := appservice.ComponentDetectionDescription{}
	componentObj := &appservice.Component{}

	Describe("Deploy RHTAP application in Kubernetes clusters", func() {
		BeforeAll(func() {
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("byoc-k8s"))
			Expect(err).NotTo(HaveOccurred())
			vc = vcluster.NewVclusterController(fmt.Sprintf("%s/tmp", rootPath), fw.AsKubeAdmin.CommonController.CustomClient)
		})

		It("starts Kubernetes ephemeral cluster", func() {
			stagingKubeconfig, err = vc.InitializeVCluster(fw.UserNamespace, fw.UserNamespace)
			Expect(err).NotTo(HaveOccurred())
			config, err := clientcmd.LoadFromFile(stagingKubeconfig)
			Expect(err).NotTo(HaveOccurred())
			kubernetesApiServerUrl = config.Clusters[config.CurrentContext].Server

			ephemeralClusterClient, err = client.NewKubeFromKubeConfigFile(stagingKubeconfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("checks if kubernetes ephemeral cluster API is available", func() {
			Eventually(func() bool {
				return utils.HostIsAccessible(kubernetesApiServerUrl)
			}, 10*time.Minute, 10*time.Second).Should(BeTrue(), "timed out when waiting for staging cluster API")
		})

		It("creates managed-gitops.redhat.com/managed-environment secret type", func() {
			kubeConfData, err := os.ReadFile(stagingKubeconfig)
			data := make(map[string][]byte)
			data["kubeconfig"] = kubeConfData
			Expect(err).NotTo(HaveOccurred())

			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      KubernetesManagedEnvironmentSecretName,
					Namespace: fw.UserNamespace,
				},
				Type: v1.SecretType(ManagedEnvironmentType),
				Data: data,
			}

			_, err = fw.AsKubeAdmin.CommonController.CreateSecret(fw.UserNamespace, secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates environment", func() {
			// Note: It is not possible yet to configure integration service snapshots to deploy to a specific environment.
			// As an workaround for now: Deletes the development environment and recreate it with kubernetes cluster information
			Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 1*time.Minute)).NotTo(HaveOccurred())

			environment, err := fw.AsKubeAdmin.GitOpsController.CreateEphemeralEnvironment(KubernetesEnvironmentName, fw.UserNamespace, fw.UserNamespace, kubernetesApiServerUrl, KubernetesManagedEnvironmentSecretName)
			Expect(environment.Name).To(Equal(KubernetesEnvironmentName))
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates namespace in the ephemeral target cluster", func() {
			ns, err := ephemeralClusterClient.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: fw.UserNamespace}}, metav1.CreateOptions{})
			Expect(ns.Name).To(Equal(fw.UserNamespace))
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates RHTAP application and check healths", func() {
			createdApplication, err := fw.AsKubeDeveloper.HasController.CreateHasApplication(applicationName, fw.UserNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdApplication.Spec.DisplayName).To(Equal(applicationName))
			Expect(createdApplication.Namespace).To(Equal(fw.UserNamespace))

			Eventually(func() string {
				application, err = fw.AsKubeAdmin.HasController.GetHasApplication(applicationName, fw.UserNamespace)
				Expect(err).NotTo(HaveOccurred())

				return application.Status.Devfile
			}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return fw.AsKubeAdmin.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
		})

		It("creates Red Hat AppStudio ComponentDetectionQuery for Component repository", func() {
			_, err := fw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(componentName, fw.UserNamespace, QuarkusDevfileSource, "", "", "", false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("validates CDQ object information", func() {
			// Validate that the CDQ completes successfully
			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				cdq, err = fw.AsKubeAdmin.HasController.GetComponentDetectionQuery(componentName, fw.UserNamespace)
				return err == nil && len(cdq.Status.ComponentDetected) > 0
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "ComponentDetectionQuery did not complete successfully")

			// Validate that the completed CDQ only has one detected component
			Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

			// Get the stub CDQ and validate its content
			for _, compDetected = range cdq.Status.ComponentDetected {
				Expect(compDetected.DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
				Expect(compDetected.Language).To(Equal("Java"), "Detected language was not java")
				Expect(compDetected.ProjectType).To(Equal("Quarkus"), "Detected framework was not quarkus")
			}
		})

		It("creates Red Hat AppStudio Quarkus component", func() {
			outputContainerImg := fmt.Sprintf("quay.io/%s/test-images:%s-%s", utils.GetQuayIOOrganization(), fw.UserName, strings.Replace(uuid.New().String(), "-", "", -1))
			componentObj, err = fw.AsKubeAdmin.HasController.CreateComponentFromStub(compDetected, fw.UserNamespace, outputContainerImg, "", applicationName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("waits component pipeline to finish", FlakeAttempts(3), func() {
			if CurrentSpecReport().NumAttempts > 1 {
				pipelineRun, err := fw.AsKubeAdmin.HasController.GetComponentPipelineRun(componentObj.Name, application.Name, fw.UserNamespace, "")
				Expect(err).ShouldNot(HaveOccurred(), "failed to get pipelinerun: %v", err)

				if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsFalse() {
					err = fw.AsKubeAdmin.TektonController.DeletePipelineRun(pipelineRun.Name, fw.UserNamespace)
					Expect(err).ShouldNot(HaveOccurred(), "failed to delete pipelinerun when retriger: %v", err)

					delete(componentObj.Annotations, constants.ComponentInitialBuildAnnotationKey)
					err = fw.AsKubeAdmin.HasController.KubeRest().Update(context.Background(), componentObj)
					Expect(err).ShouldNot(HaveOccurred(), "failed to update component to trigger another pipeline build: %v", err)
				}
			}

			if err := fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(fw.AsKubeAdmin.CommonController, componentObj.Name, application.Name, fw.UserNamespace, ""); err != nil {
				Expect(err).ShouldNot(HaveOccurred(), "pipeline didnt finish successfully: %v", err)
			}
		})

		// Deploy the component using gitops and check for the health
		It(fmt.Sprintf("deploys component %s using gitops", componentObj.Name), func() {
			Skip("skip until https://issues.redhat.com/browse/DEVHAS-329 is completed")
			var deployment *appsv1.Deployment
			Eventually(func() bool {
				deployment, err = ephemeralClusterClient.AppsV1().Deployments(fw.UserNamespace).Get(context.TODO(), componentObj.Name, metav1.GetOptions{})
				if err != nil && !errors.IsNotFound(err) {
					return false
				}
				if deployment.Status.AvailableReplicas == 1 {
					GinkgoWriter.Printf("Deployment %s is ready\n", deployment.Name)
					return true
				}

				return false
			}, 25*time.Minute, 10*time.Second).Should(BeTrue(), fmt.Sprintf("Component deployment didn't become ready: %+v", deployment))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
