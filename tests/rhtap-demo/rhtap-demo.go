package rhtap_demo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/jvm-build-service/openshift-with-appstudio-test/e2e"
	jvmclientSet "github.com/redhat-appstudio/jvm-build-service/pkg/client/clientset/versioned"
	pipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/google/go-github/v44/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	r "github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	routev1 "github.com/openshift/api/route/v1"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	e2eConfig "github.com/redhat-appstudio/e2e-tests/tests/rhtap-demo/config"
)

const (
	// Environment name used for rhtap-demo tests
	EnvironmentName string = "development"

	// Secret Name created by spi to interact with github
	SPIGithubSecretName string = "e2e-github-secret"

	// Environment name used for e2e-tests demos
	SPIQuaySecretName string = "e2e-quay-secret"

	// Timeouts
	appDeployTimeout            = time.Minute * 20
	appRouteAvailableTimeout    = time.Minute * 5
	customResourceUpdateTimeout = time.Minute * 10
	jvmRebuildTimeout           = time.Minute * 40
	mergePRTimeout              = time.Minute * 1
	pipelineRunStartedTimeout   = time.Minute * 5
	pullRequestCreationTimeout  = time.Minute * 5
	releasePipelineTimeout      = time.Minute * 15
	snapshotTimeout             = time.Minute * 4
	releaseTimeout              = time.Minute * 4
	testPipelineTimeout         = time.Minute * 15
	branchCreateTimeout         = time.Minute * 1

	// Intervals
	defaultPollingInterval    = time.Second * 2
	jvmRebuildPollingInterval = time.Second * 10
	snapshotPollingInterval   = time.Second * 1
	releasePollingInterval    = time.Second * 1
)

var supportedRuntimes = []string{"Dockerfile", "Node.js", "Go", "Quarkus", "Python", "JavaScript", "springboot", "dotnet", "maven"}

var _ = framework.RhtapDemoSuiteDescribe(Label("rhtap-demo"), func() {
	defer GinkgoRecover()

	var timeout, interval time.Duration
	var namespace string
	var err error

	// Initialize the application struct
	application := &appservice.Application{}
	snapshot := &appservice.Snapshot{}
	env := &appservice.Environment{}
	fw := &framework.Framework{}
	AfterEach(framework.ReportFailure(&fw))

	for _, appTest := range e2eConfig.TestScenarios {
		appTest := appTest
		if !appTest.Skip {

			Describe(appTest.Name, Ordered, func() {
				BeforeAll(func() {
					// Initialize the tests controllers
					fw, err = framework.NewFramework(utils.GetGeneratedNamespace("rhtap-demo"))
					Expect(err).NotTo(HaveOccurred())
					namespace = fw.UserNamespace
					Expect(namespace).NotTo(BeEmpty())

					// collect SPI ResourceQuota metrics (temporary)
					err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("rhtap-demo", namespace, "appstudio-crds-spi")
					Expect(err).NotTo(HaveOccurred())

					suiteConfig, _ := GinkgoConfiguration()
					GinkgoWriter.Printf("Parallel processes: %d\n", suiteConfig.ParallelTotal)
					GinkgoWriter.Printf("Running on namespace: %s\n", namespace)
					GinkgoWriter.Printf("User: %s\n", fw.UserName)
				})

				// Remove all resources created by the tests
				AfterAll(func() {
					// collect SPI ResourceQuota metrics (temporary)
					err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("rhtap-demo", namespace, "appstudio-crds-spi")
					Expect(err).NotTo(HaveOccurred())

					if !CurrentSpecReport().Failed() {
						Expect(fw.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.CommonController.DeleteAllSnapshotEnvBindingsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.IntegrationController.DeleteAllSnapshotsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(namespace)).To(Succeed())
						Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllGitOpsDeploymentsInASpecificNamespace(namespace, 30*time.Second)).To(Succeed())
						Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
					}
				})

				// Create an application in a specific namespace
				It("creates an application", func() {
					GinkgoWriter.Printf("Parallel process %d\n", GinkgoParallelProcess())
					createdApplication, err := fw.AsKubeDeveloper.HasController.CreateApplication(appTest.ApplicationName, namespace)
					Expect(err).NotTo(HaveOccurred())
					Expect(createdApplication.Spec.DisplayName).To(Equal(appTest.ApplicationName))
					Expect(createdApplication.Namespace).To(Equal(namespace))
				})

				// Check the application health and check if a devfile was generated in the status
				It("checks if application is healthy", func() {
					Eventually(func() string {
						appstudioApp, err := fw.AsKubeDeveloper.HasController.GetApplication(appTest.ApplicationName, namespace)
						Expect(err).NotTo(HaveOccurred())
						application = appstudioApp

						return application.Status.Devfile
					}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), fmt.Sprintf("timed out waiting for gitOps repository to be created for the %s application in %s namespace", appTest.ApplicationName, fw.UserNamespace))

					Eventually(func() bool {
						gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

						return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
					}, 1*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("timed out waiting for HAS controller to create gitops repository for the %s application in %s namespace", appTest.ApplicationName, fw.UserNamespace))
				})

				// Create an environment in a specific namespace
				It("creates an environment", func() {
					env, err = fw.AsKubeDeveloper.GitOpsController.CreatePocEnvironment(EnvironmentName, namespace)
					Expect(err).NotTo(HaveOccurred())
				})

				for _, componentSpec := range appTest.Components {
					componentSpec := componentSpec
					var componentNewBaseBranch string
					componentRepositoryName := utils.ExtractGitRepositoryNameFromURL(componentSpec.GitSourceUrl)
					cdq := &appservice.ComponentDetectionQuery{}
					componentList := []*appservice.Component{}
					var secret string

					if componentSpec.Private {
						secret = SPIGithubSecretName
						It(fmt.Sprintf("injects manually SPI token for component %s", componentSpec.Name), func() {
							// Inject spi tokens to work with private components
							if componentSpec.ContainerSource != "" {
								// More info about manual token upload for quay.io here: https://github.com/redhat-appstudio/service-provider-integration-operator/pull/115
								oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "") + `", "username":"` + utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "") + `"}`

								_ = fw.AsKubeAdmin.SPIController.InjectManualSPIToken(namespace, componentSpec.ContainerSource, oauthCredentials, corev1.SecretTypeDockerConfigJson, SPIQuaySecretName)
							}
							githubCredentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`
							_ = fw.AsKubeDeveloper.SPIController.InjectManualSPIToken(namespace, componentSpec.GitSourceUrl, githubCredentials, corev1.SecretTypeBasicAuth, SPIGithubSecretName)
						})
					}

					It(fmt.Sprintf("creates componentdetectionquery for component %s", componentSpec.Name), func() {
						gitRevision := componentSpec.GitSourceRevision
						// In case the advanced build (PaC) is enabled for this component,
						// we need to create a new branch that we will target
						// and that will contain the PaC configuration, so we can avoid polluting the default (main) branch
						if componentSpec.AdvancedBuildSpec != nil {
							componentNewBaseBranch = fmt.Sprintf("base-%s", util.GenerateRandomString(4))
							gitRevision = componentNewBaseBranch
							Expect(fw.AsKubeAdmin.CommonController.Github.CreateRef(componentRepositoryName, componentSpec.GitSourceDefaultBranchName, componentSpec.GitSourceRevision, componentNewBaseBranch)).To(Succeed())
						}
						cdq, err = fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(componentSpec.Name, namespace, componentSpec.GitSourceUrl, gitRevision, componentSpec.GitSourceContext, secret, false)
						Expect(err).NotTo(HaveOccurred())
					})

					It("check if components have supported languages by AppStudio", func() {
						if appTest.Name == e2eConfig.MultiComponentWithUnsupportedRuntime {
							// Validate that the completed CDQ only has detected 1 component and not also the unsupported component
							Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "cdq also detect unsupported component")
						}
						for _, component := range cdq.Status.ComponentDetected {
							Expect(supportedRuntimes).To(ContainElement(component.ProjectType), "unsupported runtime used for multi component tests")
						}
					})

					// Components for now can be imported from gitUrl, container image or a devfile
					if componentSpec.GitSourceUrl != "" {
						It(fmt.Sprintf("creates component %s (private: %t) from git source %s", componentSpec.Name, componentSpec.Private, componentSpec.GitSourceUrl), func() {
							for _, compDetected := range cdq.Status.ComponentDetected {
								c, err := fw.AsKubeDeveloper.HasController.CreateComponent(compDetected.ComponentStub, namespace, "", secret, appTest.ApplicationName, true, map[string]string{})
								Expect(err).NotTo(HaveOccurred())
								Expect(c.Name).To(Equal(compDetected.ComponentStub.ComponentName))
								Expect(supportedRuntimes).To(ContainElement(compDetected.ProjectType), "unsupported runtime used for multi component tests")

								componentList = append(componentList, c)
							}
						})
					} else {
						defer GinkgoRecover()
						Fail("Please Provide a valid test sample")
					}

					// Start to watch the pipeline until is finished
					It(fmt.Sprintf("waits for %s component (private: %t) pipeline to be finished", componentSpec.Name, componentSpec.Private), func() {
						if componentSpec.ContainerSource != "" {
							Skip(fmt.Sprintf("component %s was imported from quay.io/docker.io source. Skipping pipelinerun check.", componentSpec.Name))
						}
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

					// Deploy the component using gitops and check for the health
					if !componentSpec.SkipDeploymentCheck {
						var expectedReplicas int32 = 1
						It(fmt.Sprintf("deploys component %s successfully using gitops", componentSpec.Name), func() {
							var deployment *appsv1.Deployment
							for _, component := range componentList {
								Eventually(func() error {
									deployment, err = fw.AsKubeDeveloper.CommonController.GetDeployment(component.Name, namespace)
									if err != nil {
										return err
									}
									if deployment.Status.AvailableReplicas != expectedReplicas {
										return fmt.Errorf("the deployment %s/%s does not have the expected amount of replicas (expected: %d, got: %d)", deployment.GetNamespace(), deployment.GetName(), expectedReplicas, deployment.Status.AvailableReplicas)
									}
									return nil
								}, 25*time.Minute, 10*time.Second).Should(Succeed(), fmt.Sprintf("timed out waiting for deployment of a component %s/%s to become ready", component.GetNamespace(), component.GetName()))
							}
						})

						It(fmt.Sprintf("checks if component %s route(s) exist and health endpoint (if defined) is reachable", componentSpec.Name), func() {
							for _, component := range componentList {
								Eventually(func() error {
									gitOpsRoute, err := fw.AsKubeDeveloper.CommonController.GetOpenshiftRouteByComponentName(component.Name, namespace)
									Expect(err).NotTo(HaveOccurred())
									if componentSpec.HealthEndpoint != "" {
										err = fw.AsKubeDeveloper.CommonController.RouteEndpointIsAccessible(gitOpsRoute, componentSpec.HealthEndpoint)
										if err != nil {
											GinkgoWriter.Printf("Failed to request component endpoint: %+v\n retrying...\n", err)
											return err
										}
									}
									return nil
								}, 5*time.Minute, 10*time.Second).Should(Succeed())
							}
						})
					}

					if componentSpec.K8sSpec != nil && componentSpec.K8sSpec.Replicas > 1 {
						It(fmt.Sprintf("scales component %s replicas", componentSpec.Name), Pending, func() {
							for _, component := range componentList {
								c, err := fw.AsKubeDeveloper.HasController.GetComponent(component.Name, namespace)
								Expect(err).NotTo(HaveOccurred())
								_, err = fw.AsKubeDeveloper.HasController.ScaleComponentReplicas(c, pointer.Int(int(componentSpec.K8sSpec.Replicas)))
								Expect(err).NotTo(HaveOccurred())
								var deployment *appsv1.Deployment

								Eventually(func() error {
									deployment, err = fw.AsKubeDeveloper.CommonController.GetDeployment(c.Name, namespace)
									Expect(err).NotTo(HaveOccurred())
									if deployment.Status.AvailableReplicas != componentSpec.K8sSpec.Replicas {
										return fmt.Errorf("the deployment %s/%s does not have the expected amount of replicas (expected: %d, got: %d)", deployment.GetNamespace(), deployment.GetName(), componentSpec.K8sSpec.Replicas, deployment.Status.AvailableReplicas)
									}
									return nil
								}, 5*time.Minute, 10*time.Second).Should(Succeed(), "Component deployment %s/%s didn't get scaled to desired replicas", deployment.GetNamespace(), deployment.GetName())
								Expect(err).NotTo(HaveOccurred())
							}
						})
					}
					if componentSpec.AdvancedBuildSpec != nil {
						Describe(fmt.Sprintf("RHTAP Advanced build test for %s", componentSpec.Name), Ordered, func() {
							var managedNamespace string

							var component *appservice.Component
							var release *releaseApi.Release
							var snapshot *appservice.Snapshot
							var pipelineRun, testPipelinerun *tektonapi.PipelineRun
							var integrationTestScenario *integrationv1beta1.IntegrationTestScenario

							// PaC related variables
							var prNumber int
							var mergeResultSha, pacBranchName, pacControllerHost, pacPurgeBranchName string
							var mergeResult *github.PullRequestMergeResult
							var pacControllerRoute *routev1.Route

							BeforeAll(func() {
								managedNamespace = fw.UserNamespace + "-managed"
								// Used for identifying related webhook on GitHub - in order to delete it
								pacControllerRoute, err = fw.AsKubeAdmin.CommonController.GetOpenshiftRoute("pipelines-as-code-controller", "openshift-pipelines")
								Expect(err).ShouldNot(HaveOccurred())
								pacControllerHost = pacControllerRoute.Spec.Host
								component = componentList[0]

								sharedSecret, err := fw.AsKubeAdmin.CommonController.GetSecret(constants.QuayRepositorySecretNamespace, constants.QuayRepositorySecretName)
								Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error when getting shared secret - make sure the secret %s in %s userNamespace is created", constants.QuayRepositorySecretName, constants.QuayRepositorySecretNamespace))
								createReleaseConfig(*fw, managedNamespace, component.GetName(), appTest.ApplicationName, sharedSecret.Data[".dockerconfigjson"])

								its := componentSpec.AdvancedBuildSpec.TestScenario
								integrationTestScenario, err = fw.AsKubeAdmin.IntegrationController.CreateIntegrationTestScenario_beta1(appTest.ApplicationName, fw.UserNamespace, its.GitURL, its.GitRevision, its.TestPath)
								Expect(err).ShouldNot(HaveOccurred())

								pacBranchName = fmt.Sprintf("appstudio-%s", component.GetName())
								pacPurgeBranchName = fmt.Sprintf("appstudio-purge-%s", component.GetName())

								// JBS related config
								_, err = fw.AsKubeAdmin.JvmbuildserviceController.CreateJBSConfig(constants.JBSConfigName, fw.UserNamespace)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(fw.AsKubeAdmin.JvmbuildserviceController.WaitForCache(fw.AsKubeAdmin.CommonController, fw.UserNamespace)).Should(Succeed())
							})
							AfterAll(func() {
								if !CurrentSpecReport().Failed() {
									Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(Succeed())
									Expect(fw.AsKubeAdmin.JvmbuildserviceController.DeleteJBSConfig(constants.JBSConfigName, fw.UserNamespace)).To(Succeed())
								}

								// Delete created webhook from GitHub
								hooks, err := fw.AsKubeAdmin.CommonController.Github.ListRepoWebhooks(componentRepositoryName)
								Expect(err).NotTo(HaveOccurred())

								for _, h := range hooks {
									hookUrl := h.Config["url"].(string)
									if strings.Contains(hookUrl, pacControllerHost) {
										Expect(fw.AsKubeAdmin.CommonController.Github.DeleteWebhook(componentRepositoryName, h.GetID())).To(Succeed())
										break
									}
								}
								// Delete new branch created by PaC and a testing branch used as a component's base branch
								Expect(fw.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepositoryName, pacBranchName)).To(Succeed())
								Expect(fw.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepositoryName, componentNewBaseBranch)).To(Succeed())
								Expect(fw.AsKubeAdmin.CommonController.Github.DeleteRef(constants.StrategyConfigsRepo, component.GetName())).To(Succeed())
							})
							When("Component is switched to Advanced Build mode", func() {

								BeforeAll(func() {
									component, err = fw.AsKubeAdmin.HasController.GetComponent(component.GetName(), fw.UserNamespace)
									Expect(err).ShouldNot(HaveOccurred(), "failed to get component: %v", err)

									component.Annotations["skip-initial-checks"] = "false"
									for k, v := range constants.ComponentPaCRequestAnnotation {
										component.Annotations[k] = v
									}
									Expect(fw.AsKubeAdmin.CommonController.KubeRest().Update(context.TODO(), component)).To(Succeed())
									Expect(err).ShouldNot(HaveOccurred(), "failed to update component: %v", err)
								})

								It("triggers creation of a PR in the sample repo", func() {

									var prSHA string
									Eventually(func() error {
										prs, err := fw.AsKubeAdmin.CommonController.Github.ListPullRequests(componentRepositoryName)
										Expect(err).ShouldNot(HaveOccurred())
										for _, pr := range prs {
											if pr.Head.GetRef() == pacBranchName {
												prNumber = pr.GetNumber()
												prSHA = pr.GetHead().GetSHA()
												return nil
											}
										}
										return fmt.Errorf("could not get the expected PaC branch name %s", pacBranchName)
									}, pullRequestCreationTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for init PaC PR (branch %q) to be created against the %q repo", pacBranchName, componentRepositoryName))

									// We actually don't need the "on-pull-request" PipelineRun to complete, so we can delete it
									Eventually(func() error {
										pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appTest.ApplicationName, fw.UserNamespace, prSHA)
										if err == nil {
											Expect(fw.AsKubeAdmin.TektonController.DeletePipelineRun(pipelineRun.Name, pipelineRun.Namespace)).To(Succeed())
											return nil
										}
										return err
									}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for init PaC PipelineRun to be present in the user namespace %q for component %q with a label pointing to %q", fw.UserNamespace, component.GetName(), appTest.ApplicationName))

								})

								It("should eventually lead to triggering another PipelineRun after merging the PaC init branch ", func() {
									Eventually(func() error {
										mergeResult, err = fw.AsKubeAdmin.CommonController.Github.MergePullRequest(componentRepositoryName, prNumber)
										return err
									}, mergePRTimeout).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v\n", err))

									mergeResultSha = mergeResult.GetSHA()

									Eventually(func() error {
										pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appTest.ApplicationName, fw.UserNamespace, mergeResultSha)
										if err != nil {
											GinkgoWriter.Printf("PipelineRun has not been created yet for component %s/%s\n", fw.UserNamespace, component.GetName())
											return err
										}
										if !pipelineRun.HasStarted() {
											return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
										}
										return nil
									}, pipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for a PipelineRun in namespace %q with label component label %q and application label %q and sha label %q to start", fw.UserNamespace, component.GetName(), appTest.ApplicationName, mergeResultSha))
								})
							})

							When("SLSA level 3 customizable PipelineRun is created", func() {
								It("should eventually complete successfully", func() {
									Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, mergeResultSha, 2, fw.AsKubeAdmin.TektonController)).To(Succeed())
								})

								It("does not contain an annotation with a Snapshot Name", func() {
									Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(""))
								})
							})

							When("SLSA level 3 customizable PipelineRun completes successfully", func() {
								It("should be possible to download the SBOM file", func() {
									var outputImage string
									for _, p := range pipelineRun.Spec.Params {
										if p.Name == "output-image" {
											outputImage = p.Value.StringVal
										}
									}
									Expect(outputImage).ToNot(BeEmpty(), "output image of a component could not be found")

									_, _, err = build.GetParsedSbomFilesContentFromImage(outputImage)
									Expect(err).NotTo(HaveOccurred())
								})

								It("should validate Tekton TaskRun test results successfully", func() {
									pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appTest.ApplicationName, fw.UserNamespace, mergeResultSha)
									Expect(err).ShouldNot(HaveOccurred())
									Expect(build.ValidateBuildPipelineTestResults(pipelineRun, fw.AsKubeAdmin.CommonController.KubeRest())).To(Succeed())
								})

								It("should validate pipelineRun is signed", func() {
									pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appTest.ApplicationName, fw.UserNamespace, mergeResultSha)
									Expect(err).ShouldNot(HaveOccurred())
									Expect(pipelineRun.Annotations["chains.tekton.dev/signed"]).To(Equal("true"))
								})

								It("should find the related Snapshot CR", func() {
									Eventually(func() error {
										snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", fw.UserNamespace)
										return err
									}, snapshotTimeout, snapshotPollingInterval).Should(Succeed(), "timed out when trying to check if the Snapshot exists for PipelineRun %s/%s", fw.UserNamespace, pipelineRun.GetName())
								})

								It("should validate the pipelineRun is annotated with the name of the Snapshot", func() {
									pipelineRun, err = fw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appTest.ApplicationName, fw.UserNamespace, mergeResultSha)
									Expect(err).ShouldNot(HaveOccurred())
									Expect(pipelineRun.Annotations["appstudio.openshift.io/snapshot"]).To(Equal(snapshot.GetName()))
								})

								It("should find the related Integration Test PipelineRun", func() {
									Eventually(func() error {
										testPipelinerun, err = fw.AsKubeAdmin.IntegrationController.GetIntegrationPipelineRun(integrationTestScenario.Name, snapshot.Name, fw.UserNamespace)
										if err != nil {
											GinkgoWriter.Printf("failed to get Integration test PipelineRun for a snapshot '%s' in '%s' namespace: %+v\n", snapshot.Name, fw.UserNamespace, err)
											return err
										}
										if !testPipelinerun.HasStarted() {
											return fmt.Errorf("pipelinerun %s/%s hasn't started yet", testPipelinerun.GetNamespace(), testPipelinerun.GetName())
										}
										return nil
									}, pipelineRunStartedTimeout, defaultPollingInterval).Should(Succeed())
									Expect(testPipelinerun.Labels["appstudio.openshift.io/snapshot"]).To(ContainSubstring(snapshot.Name))
									Expect(testPipelinerun.Labels["test.appstudio.openshift.io/scenario"]).To(ContainSubstring(integrationTestScenario.Name))
								})
							})

							When("Integration Test PipelineRun is created", func() {
								It("should eventually complete successfully", func() {
									Expect(fw.AsKubeAdmin.IntegrationController.WaitForIntegrationPipelineToBeFinished(integrationTestScenario, snapshot, fw.UserNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a integration pipeline for snapshot %s/%s to finish", fw.UserNamespace, snapshot.GetName()))
								})
							})

							When("Integration Test PipelineRun completes successfully", func() {

								It("should lead to Snapshot CR being marked as passed", func() {
									snapshot, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", fw.UserNamespace)
									Expect(err).ShouldNot(HaveOccurred())
									Expect(fw.AsKubeAdmin.CommonController.HaveTestsSucceeded(snapshot)).To(BeTrue(), fmt.Sprintf("tests have not succeeded for snapshot %s/%s", snapshot.GetNamespace(), snapshot.GetName()))
								})

								It("should trigger creation of Release CR", func() {
									Eventually(func() error {
										release, err = fw.AsKubeAdmin.ReleaseController.GetRelease("", snapshot.Name, fw.UserNamespace)
										return err
									}, releaseTimeout, releasePollingInterval).Should(Succeed(), fmt.Sprintf("timed out when trying to check if the release exists for snapshot %s/%s", fw.UserNamespace, snapshot.GetName()))
								})
							})

							When("Release CR is created", func() {
								It("triggers creation of Release PipelineRun", func() {
									Eventually(func() error {
										pipelineRun, err = fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
										if err != nil {
											GinkgoWriter.Printf("pipelineRun for component '%s' in namespace '%s' not created yet: %+v\n", component.GetName(), managedNamespace, err)
											return err
										}
										if !pipelineRun.HasStarted() {
											return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
										}
										return nil
									}, pipelineRunStartedTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("failed to get pipelinerun named %q in namespace %q with label to release %q in namespace %q to start", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
								})
							})

							When("Release PipelineRun is triggered", func() {
								It("should eventually succeed", func() {
									Eventually(func() error {
										pr, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, release.Name, release.Namespace)
										Expect(err).ShouldNot(HaveOccurred())
										Expect(utils.HasPipelineRunFailed(pr)).NotTo(BeTrue(), fmt.Sprintf("did not expect PipelineRun %s/%s to fail", pr.GetNamespace(), pr.GetName()))
										if !pr.IsDone() {
											return fmt.Errorf("release pipelinerun %s/%s has not finished yet", pr.GetNamespace(), pr.GetName())
										}
										Expect(utils.HasPipelineRunSucceeded(pr)).To(BeTrue(), fmt.Sprintf("PipelineRun %s/%s did not succeed", pr.GetNamespace(), pr.GetName()))
										return nil
									}, releasePipelineTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("failed to see pipelinerun %q in namespace %q with a label pointing to release %q in namespace %q to complete successfully", pipelineRun.Name, managedNamespace, release.Name, release.Namespace))
								})
							})
							When("Release PipelineRun is completed", func() {
								It("should lead to Release CR being marked as succeeded", func() {
									Eventually(func() error {
										release, err = fw.AsKubeAdmin.ReleaseController.GetRelease(release.Name, "", fw.UserNamespace)
										Expect(err).ShouldNot(HaveOccurred())
										if !release.IsReleased() {
											return fmt.Errorf("release CR %s/%s is not marked as finished yet", release.GetNamespace(), release.GetName())
										}
										return nil
									}, customResourceUpdateTimeout, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("failed to see release %q in namespace %q get marked as released", release.Name, fw.UserNamespace))
								})
							})

							When("JVM Build Service is used for rebuilding dependencies", func() {
								It("should eventually rebuild of all artifacts and dependencies successfully", func() {
									jvmClient := jvmclientSet.New(fw.AsKubeAdmin.JvmbuildserviceController.JvmbuildserviceClient().JvmbuildserviceV1alpha1().RESTClient())
									tektonClient := pipelineclientset.New(fw.AsKubeAdmin.TektonController.PipelineClient().TektonV1beta1().RESTClient())
									kubeClient := kubernetes.New(fw.AsKubeAdmin.CommonController.KubeInterface().CoreV1().RESTClient())
									//status report ends up in artifacts/redhat-appstudio-e2e/redhat-appstudio-e2e/artifacts/rp_preproc/attachments/xunit
									defer e2e.GenerateStatusReport(fw.UserNamespace, jvmClient, kubeClient, tektonClient)
									Eventually(func() error {
										abList, err := fw.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(fw.UserNamespace)
										Expect(err).ShouldNot(HaveOccurred())
										for _, ab := range abList.Items {
											if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
												return fmt.Errorf("artifactbuild %s not complete", ab.Spec.GAV)
											}
										}
										dbList, err := fw.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(fw.UserNamespace)
										Expect(err).ShouldNot(HaveOccurred())
										for _, db := range dbList.Items {
											if db.Status.State != v1alpha1.DependencyBuildStateComplete {
												return fmt.Errorf("dependencybuild %s not complete", db.Spec.ScmInfo.SCMURL)
											}
										}
										return nil
									}, jvmRebuildTimeout, jvmRebuildPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for all artifactbuilds and dependencybuilds to complete in namespace %q", fw.UserNamespace))
								})
							})

							When("User switches to simple build", func() {
								BeforeAll(func() {
									comp, err := fw.AsKubeAdmin.HasController.GetComponent(component.GetName(), fw.UserNamespace)
									Expect(err).ShouldNot(HaveOccurred())
									comp.Annotations["build.appstudio.openshift.io/request"] = "unconfigure-pac"
									Expect(fw.AsKubeAdmin.CommonController.KubeRest().Update(context.TODO(), comp)).To(Succeed())
								})
								AfterAll(func() {
									// Delete the new branch created by sending purge PR while moving to simple build
									err = fw.AsKubeAdmin.CommonController.Github.DeleteRef(componentRepositoryName, pacPurgeBranchName)
									if err != nil {
										Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
									}
								})
								It("creates a pull request for removing PAC configuration", func() {
									Eventually(func() error {
										prs, err := fw.AsKubeAdmin.CommonController.Github.ListPullRequests(componentRepositoryName)
										Expect(err).ShouldNot(HaveOccurred())
										for _, pr := range prs {
											if pr.Head.GetRef() == pacPurgeBranchName {
												return nil
											}
										}
										return fmt.Errorf("could not get the expected PaC purge PR branch %s", pacPurgeBranchName)
									}, time.Minute*1, defaultPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for PaC purge PR to be created against the %q repo", componentRepositoryName))
								})
							})
						})
					}
				}
			})
		}
	}
})

func createReleaseConfig(fw framework.Framework, managedNamespace, componentName, appName string, secretData []byte) {
	var err error
	_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
	Expect(err).ShouldNot(HaveOccurred())

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "release-pull-secret", Namespace: managedNamespace},
		Data: map[string][]byte{".dockerconfigjson": secretData},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
	Expect(err).ShouldNot(HaveOccurred())

	managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount("release-service-account", managedNamespace, []corev1.ObjectReference{{Name: secret.Name}}, nil)
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(fw.UserNamespace, managedServiceAccount)
	Expect(err).NotTo(HaveOccurred())
	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
	Expect(err).NotTo(HaveOccurred())

	publicKey, err := fw.AsKubeAdmin.TektonController.GetTektonChainsPublicKey()
	Expect(err).ToNot(HaveOccurred())

	Expect(fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(publicKey, "cosign-public-key", managedNamespace)).To(Succeed())

	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan("source-releaseplan", fw.UserNamespace, appName, managedNamespace, "")
	Expect(err).NotTo(HaveOccurred())

	components := []r.Component{{Name: componentName, Repository: constants.DefaultReleasedImagePushRepo}}
	sc := fw.AsKubeAdmin.ReleaseController.GenerateReleaseStrategyConfig(components)
	scYaml, err := yaml.Marshal(sc)
	Expect(err).ShouldNot(HaveOccurred())

	scPath := componentName + ".yaml"
	Expect(fw.AsKubeAdmin.CommonController.Github.CreateRef(constants.StrategyConfigsRepo, constants.StrategyConfigsDefaultBranch, "", componentName)).To(Succeed())
	_, err = fw.AsKubeAdmin.CommonController.Github.CreateFile(constants.StrategyConfigsRepo, scPath, string(scYaml), componentName)
	Expect(err).ShouldNot(HaveOccurred())

	defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
	Expect(err).NotTo(HaveOccurred())
	ecPolicyName := componentName + "-policy"
	defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
		Description: "Red Hat's enterprise requirements",
		PublicKey:   string(publicKey),
		Sources:     defaultEcPolicy.Spec.Sources,
		Configuration: &ecp.EnterpriseContractPolicyConfiguration{
			Collections: []string{"minimal"},
			Exclude:     []string{"cve"},
		},
	}
	_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(ecPolicyName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())
	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy(componentName+"-strategy", managedNamespace, "release", constants.ReleasePipelineImageRef, ecPolicyName, "release-service-account", []releaseApi.Params{
		{Name: "extraConfigGitUrl", Value: fmt.Sprintf("https://github.com/%s/strategy-configs.git", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))},
		{Name: "extraConfigPath", Value: scPath},
		{Name: "extraConfigGitRevision", Value: componentName},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission("demo", fw.UserNamespace, appName, managedNamespace, "", "", componentName+"-strategy")
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode("release-pvc", managedNamespace, corev1.ReadWriteOnce)
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
		"apiGroupsList": {""},
		"roleResources": {"secrets"},
		"roleVerbs":     {"get", "list", "watch"},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", "release-service-account", managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
	Expect(err).NotTo(HaveOccurred())
}
