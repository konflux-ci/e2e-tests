package byoc

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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/apis"
)

const (
	ManagedEnvironmentSecretName string = "byoc-managed-environment"
	ManagedEnvironmentType       string = "managed-gitops.redhat.com/managed-environment"
	ManagedEnvironmentName       string = "development"
	QuarkusDevfileSource         string = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"
)

var (
	applicationName = utils.GetGeneratedNamespace("byoc-app")
	componentName   = utils.GetGeneratedNamespace("byoc-comp")
)

// Initialize simple scenarios to test RHTAP byoc feature flow: Feature jira link: https://issues.redhat.com/browse/RHTAP-129
// QE Test Jira link: https://issues.redhat.com/browse/RHTAP-542
var byocTestScenario = []Scenario{
	{
		Name: "Deploy RHTAP sample application into a Kubernetes cluster provided by user",
		ApplicationService: ApplicationService{
			GithubRepository: QuarkusDevfileSource,
			ApplicationName:  applicationName,
		},
		Byoc: Byoc{
			ClusterType:     appservice.ConfigurationClusterType_Kubernetes,
			TargetNamespace: utils.GetGeneratedNamespace("byoc-k8s-target"),
		},
	},
	{
		Name: "Deploy RHTAP sample application into a Openshift cluster provided by user",
		ApplicationService: ApplicationService{
			GithubRepository: QuarkusDevfileSource,
			ApplicationName:  applicationName,
		},
		Byoc: Byoc{
			ClusterType:     appservice.ConfigurationClusterType_OpenShift,
			TargetNamespace: utils.GetGeneratedNamespace("byoc-ocp-target"),
		},
	},
}

var _ = framework.E2ESuiteDescribe(Label("byoc"), Ordered, func() {
	rootPath, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	var vc vcluster.Vcluster
	var ephemeralClusterClient *kubernetes.Clientset
	var fw *framework.Framework
	var byocKubeconfig, byocAPIServerURL, kubeIngressDomain string

	// Initialize the application struct
	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	compDetected := appservice.ComponentDetectionDescription{}
	componentObj := &appservice.Component{}

	for _, suite := range byocTestScenario {
		suite := suite

		Describe(suite.Name, func() {
			BeforeAll(func() {
				fw, err = framework.NewFramework(utils.GetGeneratedNamespace(fmt.Sprintf("byoc-%s", strings.ToLower(string(suite.Byoc.ClusterType)))))
				Expect(err).NotTo(HaveOccurred())

				// Use target test cluster as Ingress for the cluster provided for user. Ingress is mandatory for kubernetes cluster like vcluster in this case.
				openshiftIngress, err := fw.AsKubeAdmin.CommonController.GetOpenshiftIngress()
				Expect(err).NotTo(HaveOccurred())

				kubeIngressDomain = openshiftIngress.Spec.Domain
				Expect(kubeIngressDomain).NotTo(BeEmpty(), "domain is not pressent in the cluster. Make sure your openshift cluster have the domain defined in ingress cluster object")

				if suite.Byoc.ClusterType == appservice.ConfigurationClusterType_Kubernetes {
					vc = vcluster.NewVclusterController(fmt.Sprintf("%s/tmp", rootPath), fw.AsKubeAdmin.CommonController.CustomClient)

					byocKubeconfig, err = vc.InitializeVCluster(fw.UserNamespace, fw.UserNamespace)
					Expect(err).NotTo(HaveOccurred())
					Expect(byocKubeconfig).NotTo(BeEmpty(), "failed to initialize vcluster. Kubeconfig not provided")

				} else if suite.Byoc.ClusterType == appservice.ConfigurationClusterType_OpenShift {
					byocKubeconfig = utils.GetEnv("BYOC_KUBECONFIG", "")
					Expect(byocKubeconfig).NotTo(BeEmpty(), "Please provide BYOC_KUBECONFIG env pointing to a valid openshift kubeconfig file")
				}
			})

			// Remove all resources created by the tests in case the suite was successfull
			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					Expect(fw.AsKubeDeveloper.HasController.DeleteAllComponentsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.HasController.DeleteAllSnapshotEnvBindingsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.ReleaseController.DeleteAllSnapshotsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(fw.UserNamespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllGitOpsDeploymentInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
				}
			})

			It("initializes byoc cluster connection and creates targetNamespace", func() {

				config, err := clientcmd.BuildConfigFromFlags("", byocKubeconfig)
				Expect(err).NotTo(HaveOccurred())
				byocAPIServerURL = config.Host
				Expect(byocAPIServerURL).NotTo(BeEmpty())

				ephemeralClusterClient, err = client.NewKubeFromKubeConfigFile(byocKubeconfig)
				Expect(err).NotTo(HaveOccurred())

				// Cluster is managed by a user so we need to create the target cluster where we will deploy the RHTAP components
				ns, err := ephemeralClusterClient.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: suite.Byoc.TargetNamespace}}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ns.Name).To(Equal(suite.Byoc.TargetNamespace))
			})

			It("creates managed-gitops.redhat.com/managed-environment secret type", func() {
				kubeConfData, err := os.ReadFile(byocKubeconfig)
				data := make(map[string][]byte)
				data["kubeconfig"] = kubeConfData
				Expect(err).NotTo(HaveOccurred())

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ManagedEnvironmentSecretName,
						Namespace: fw.UserNamespace,
					},
					Type: corev1.SecretType(ManagedEnvironmentType),
					Data: data,
				}

				_, err = fw.AsKubeAdmin.CommonController.CreateSecret(fw.UserNamespace, secret)
				Expect(err).NotTo(HaveOccurred())
			})

			It("creates environment", func() {
				// Note: It is not possible yet to configure integration service snapshots to deploy to a specific environment.
				// As an workaround for now: Deletes the development environment and recreate it with kubernetes cluster information
				Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 1*time.Minute)).NotTo(HaveOccurred())

				environment, err := fw.AsKubeAdmin.GitOpsController.CreateEphemeralEnvironment(ManagedEnvironmentName, fw.UserNamespace, suite.Byoc.TargetNamespace, byocAPIServerURL, ManagedEnvironmentSecretName, suite.Byoc.ClusterType, kubeIngressDomain)
				Expect(environment.Name).To(Equal(ManagedEnvironmentName))
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
				_, err := fw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(componentName, fw.UserNamespace, suite.ApplicationService.GithubRepository, "", "", "", false)
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
			It(fmt.Sprintf("checks if component %s was deployed in the target cluster and namespace", componentObj.Name), func() {
				if suite.Byoc.ClusterType == appservice.ConfigurationClusterType_Kubernetes {
					Skip("skip until https://issues.redhat.com/browse/DEVHAS-329 is completed")
				}

				var deployment *appsv1.Deployment
				Eventually(func() bool {
					deployment, err = ephemeralClusterClient.AppsV1().Deployments(suite.Byoc.TargetNamespace).Get(context.TODO(), componentObj.Name, metav1.GetOptions{})
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
	}
})
