package byoc

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/magefile/mage/sh"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	client "github.com/redhat-appstudio/e2e-tests/pkg/clients/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/vcluster"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/gitops"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	ManagedEnvironmentSecretName string = "byoc-managed-environment"
	ManagedEnvironmentType       string = "managed-gitops.redhat.com/managed-environment"
	ManagedEnvironmentName       string = "development"
	QuarkusDevfileSource         string = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"
	QuarkusComponentEndpoint     string = "/hello-resteasy"
)

var (
	applicationName = utils.GetGeneratedNamespace("byoc-app")
	cdqName         = utils.GetGeneratedNamespace("byoc-comp")
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

var _ = framework.ByocSuiteDescribe(Label("byoc"), Ordered, func() {
	rootPath, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	var vc vcluster.Vcluster
	var ephemeralClusterClient *kubernetes.Clientset
	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
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
				fw, err = framework.NewFramework(utils.GetGeneratedNamespace("byoc"))
				Expect(err).NotTo(HaveOccurred())

				// Use target test cluster as Ingress for the cluster provided for user. Ingress is mandatory for kubernetes cluster like vcluster in this case.
				openshiftIngress, err := fw.AsKubeAdmin.CommonController.GetOpenshiftIngress()
				Expect(err).NotTo(HaveOccurred())

				kubeIngressDomain = openshiftIngress.Spec.Domain
				Expect(kubeIngressDomain).NotTo(BeEmpty(), "domain is not present in the cluster. Make sure your openshift cluster has the domain defined in ingress cluster object")

				if suite.Byoc.ClusterType == appservice.ConfigurationClusterType_Kubernetes {
					Expect(sh.Run("which", "vcluster")).To(Succeed(), "please install vcluster locally in order to run kubernetes suite")

					vc = vcluster.NewVclusterController(fmt.Sprintf("%s/tmp", rootPath), fw.AsKubeAdmin.CommonController.CustomClient)

					byocKubeconfig, err = vc.InitializeVCluster(fw.UserNamespace, fw.UserNamespace, kubeIngressDomain)
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
					// RHTAPBUGS-978: temporary timeout to 10min
					if err := fw.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(fw.UserNamespace, 10*time.Minute); err != nil {
						if err := fw.AsKubeAdmin.StoreAllArtifactsForNamespace(fw.UserNamespace); err != nil {
							Fail(fmt.Sprintf("error archiving artifacts:\n%s", err))
						}
						Fail(fmt.Sprintf("error deleting all componentns in namespace:\n%s", err))
					}
					Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.CommonController.DeleteAllSnapshotEnvBindingsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.IntegrationController.DeleteAllSnapshotsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(fw.UserNamespace)).To(Succeed())
					Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllGitOpsDeploymentsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
					Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
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
				ns, err := ephemeralClusterClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: suite.Byoc.TargetNamespace}}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ns.Name).To(Equal(suite.Byoc.TargetNamespace))
			})

			It("creates managed-gitops.redhat.com/managed-environment secret type", func() {
				kubeConfData, err := os.ReadFile(byocKubeconfig)
				Expect(err).NotTo(HaveOccurred())
				data := make(map[string][]byte)
				data["kubeconfig"] = kubeConfData

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
				Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 1*time.Minute)).To(Succeed())

				environment, err := fw.AsKubeAdmin.GitOpsController.CreateEphemeralEnvironment(ManagedEnvironmentName, fw.UserNamespace, suite.Byoc.TargetNamespace, byocAPIServerURL, ManagedEnvironmentSecretName, suite.Byoc.ClusterType, kubeIngressDomain)
				Expect(err).NotTo(HaveOccurred())
				Expect(environment.Name).To(Equal(ManagedEnvironmentName))
			})

			It("creates RHTAP application and check its health", func() {
				createdApplication, err := fw.AsKubeDeveloper.HasController.CreateApplication(applicationName, fw.UserNamespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdApplication.Spec.DisplayName).To(Equal(applicationName))
				Expect(createdApplication.Namespace).To(Equal(fw.UserNamespace))

				Eventually(func() string {
					application, err = fw.AsKubeAdmin.HasController.GetApplication(applicationName, fw.UserNamespace)
					Expect(err).NotTo(HaveOccurred())

					return application.Status.Devfile
				}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), fmt.Sprintf("timed out waiting for gitOps repository to be created for the %s application in %s namespace", applicationName, fw.UserNamespace))

				Eventually(func() bool {
					// application info should be stored even after deleting the application in application variable
					gitOpsRepository := gitops.ObtainGitOpsRepositoryName(application.Status.Devfile)

					return fw.AsKubeAdmin.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
				}, 1*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("timed out waiting for HAS controller to create gitops repository for the %s application in %s namespace", applicationName, fw.UserNamespace))
			})

			It("creates Red Hat AppStudio ComponentDetectionQuery for Component repository", func() {
				_, err := fw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(cdqName, fw.UserNamespace, suite.ApplicationService.GithubRepository, "", "", "", false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates CDQ object information", func() {
				// Validate that the CDQ completes successfully
				Eventually(func() (appservice.ComponentDetectionMap, error) {
					// application info should be stored even after deleting the application in application variable
					cdq, err = fw.AsKubeAdmin.HasController.GetComponentDetectionQuery(cdqName, fw.UserNamespace)
					if err != nil {
						return nil, err
					}
					return cdq.Status.ComponentDetected, nil
					// Validate that the completed CDQ only has one detected component
				}, 1*time.Minute, 1*time.Second).Should(HaveLen(1), fmt.Sprintf("ComponentDetectionQuery %s/%s does not have the expected amount of components", fw.UserNamespace, cdqName))

				// Get the stub CDQ and validate its content
				for _, compDetected = range cdq.Status.ComponentDetected {
					Expect(compDetected.DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
					Expect(compDetected.Language).To(Equal("Java"), "Detected language was not java")
					Expect(compDetected.ProjectType).To(Equal("Quarkus"), "Detected framework was not quarkus")
				}
			})

			It("creates Red Hat AppStudio Quarkus component", func() {
				compDetected.ComponentStub.ComponentName = util.GenerateRandomString(6)
				componentObj, err = fw.AsKubeAdmin.HasController.CreateComponent(compDetected.ComponentStub, fw.UserNamespace, "", "", applicationName, true, map[string]string{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("waits component pipeline to finish", func() {
				Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(componentObj, "",
					fw.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true})).To(Succeed())
			})

			// Deploy the component using gitops and check for the health
			It(fmt.Sprintf("checks if component %s was deployed in the target cluster and namespace", componentObj.Name), func() {
				var deployment *appsv1.Deployment
				var expectedReplicas int32 = 1
				Eventually(func() error {
					deployment, err = ephemeralClusterClient.AppsV1().Deployments(suite.Byoc.TargetNamespace).Get(context.Background(), componentObj.Name, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("could not get deployment %s/%s: %+v", suite.Byoc.TargetNamespace, componentObj.GetName(), err)
					}
					if deployment.Status.AvailableReplicas != expectedReplicas {
						return fmt.Errorf("expected %d replicas for %s/%s deployment, got %d", expectedReplicas, deployment.GetNamespace(), deployment.GetName(), deployment.Status.AvailableReplicas)
					}
					return nil
				}, 25*time.Minute, 10*time.Second).Should(Succeed(), fmt.Sprintf("timed out waiting for deployment of a component %s/%s to become ready in ephemeral cluster", suite.Byoc.TargetNamespace, componentObj.GetName()))
				Expect(err).NotTo(HaveOccurred())
			})

			if suite.Byoc.ClusterType == appservice.ConfigurationClusterType_Kubernetes {
				It("checks if ingress exists and is accessible in the kubernetes ephemeral cluster", func() {
					var ingress *v1.Ingress
					Eventually(func() error {
						ingress, err = ephemeralClusterClient.NetworkingV1().Ingresses(suite.Byoc.TargetNamespace).Get(context.Background(), componentObj.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						return nil
					}, 10*time.Minute, 10*time.Second).Should(Succeed(), fmt.Sprintf("ingress %s/%s didn't appear in target cluster in 10 minutes: %+v", componentObj.GetName(), suite.Byoc.TargetNamespace, ingress))

					if len(ingress.Spec.Rules) == 0 {
						Fail(fmt.Sprintf("kubernetes ingress %s/%s did not have any rule set during component creation", ingress.GetNamespace(), ingress.GetName()))
					}

					// Add complex endpoint checks when: https://issues.redhat.com/browse/DEVHAS-367 is ready
					Eventually(func() bool {
						// Add endpoint of component when: https://issues.redhat.com/browse/DEVHAS-367 is ready
						return utils.HostIsAccessible(fmt.Sprintf("http://%s", ingress.Spec.Rules[0].Host))
					}, 10*time.Minute, 10*time.Second).Should(BeTrue(), fmt.Sprintf("timed out waiting for ingress %s/%s host (%s) to be accessible: %+v", ingress.GetNamespace(), ingress.GetName(), ingress.Spec.Rules[0].Host, ingress))
				})
			}
		})
	}
})
