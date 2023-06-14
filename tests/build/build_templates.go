package build

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kubeapi "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/pipeline"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = framework.BuildSuiteDescribe("Build service E2E tests", Label("build", "HACBS"), func() {
	var f *framework.Framework
	var err error

	defer GinkgoRecover()
	Describe("HACBS pipelines", Ordered, Label("pipeline"), func() {

		var applicationName, componentName, testNamespace string
		var kubeadminClient *framework.ControllerHub
		pipelineCreatedRetryInterval := time.Second * 5
		pipelineCreatedTimeout := time.Minute * 5

		BeforeAll(func() {
			if os.Getenv("APP_SUFFIX") != "" {
				applicationName = fmt.Sprintf("test-app-%s", os.Getenv("APP_SUFFIX"))
			} else {
				applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))
			}
			testNamespace = os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV)
			if len(testNamespace) > 0 {
				asAdminClient, err := kubeapi.NewAdminKubernetesClient()
				Expect(err).ShouldNot(HaveOccurred())
				kubeadminClient, err = framework.InitControllerHub(asAdminClient)
				Expect(err).ShouldNot(HaveOccurred())
				_, err = kubeadminClient.CommonController.CreateTestNamespace(testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
			} else {
				f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
				Expect(err).NotTo(HaveOccurred())
				testNamespace = f.UserNamespace
				Expect(f.UserNamespace).NotTo(BeNil())
				kubeadminClient = f.AsKubeAdmin
			}

			_, err = kubeadminClient.HasController.GetHasApplication(applicationName, testNamespace)
			// In case the app with the same name exist in the selected namespace, delete it first
			if err == nil {
				Expect(kubeadminClient.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
				Eventually(func() bool {
					_, err := kubeadminClient.HasController.GetHasApplication(applicationName, testNamespace)
					return errors.IsNotFound(err)
				}, time.Minute*5, time.Second*1).Should(BeTrue(), "timed out when waiting for the app %s to be deleted in %s namespace", applicationName, testNamespace)
			}
			app, err := kubeadminClient.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(kubeadminClient.HasController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			for _, gitUrl := range componentUrls {
				gitUrl := gitUrl
				componentName = fmt.Sprintf("%s-%s", "test-comp", util.GenerateRandomString(4))
				// Create a component with Git Source URL being defined
				// using cdq since git ref is not known
				cdq, err := kubeadminClient.HasController.CreateComponentDetectionQuery(componentName, testNamespace, gitUrl, "", "", "", false)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

				for _, compDetected := range cdq.Status.ComponentDetected {
					c, err := kubeadminClient.HasController.CreateComponentFromStubSkipInitialChecks(compDetected, testNamespace, "", "", applicationName, false)
					Expect(err).ShouldNot(HaveOccurred())
					componentNames = append(componentNames, c.Name)
				}
			}
		})

		AfterAll(func() {
			// Do cleanup only in case the test succeeded
			if !CurrentSpecReport().Failed() {
				// Clean up only Application CR (Component and Pipelines are included) in case we are targeting specific namespace
				// Used e.g. in build-definitions e2e tests, where we are targeting build-templates-e2e namespace
				if os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV) != "" {
					DeferCleanup(kubeadminClient.HasController.DeleteHasApplication, applicationName, testNamespace, false)
				} else {
					Expect(kubeadminClient.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
					Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
					Expect(kubeadminClient.CommonController.DeleteProxyPlugin("tekton-results", "toolchain-host-operator")).NotTo(BeFalse())
				}
			}
		})

		for i, gitUrl := range componentUrls {
			i := i
			gitUrl := gitUrl
			It(fmt.Sprintf("triggers PipelineRun for component with source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				timeout := time.Minute * 25
				interval := time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the %s PipelineRun to start", componentNames[i])
			})
		}

		for i, gitUrl := range componentUrls {
			i := i
			gitUrl := gitUrl

			It(fmt.Sprintf("should eventually finish successfully for component with source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				component, err := kubeadminClient.HasController.GetHasComponent(componentNames[i], testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeadminClient.HasController.WaitForComponentPipelineToBeFinished(component, "", 2)).To(Succeed())
			})

			It("should ensure SBOM is shown", Label(buildTemplatesTestLabel), func() {
				pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pipelineRun).ToNot(BeNil(), "component pipelinerun not found")

				logs, err := kubeadminClient.TektonController.GetTaskRunLogs(pipelineRun.Name, "show-sbom", testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(logs).To(HaveLen(1))
				var sbomTaskLog string
				for _, log := range logs {
					sbomTaskLog = log
				}

				sbom := &build.SbomCyclonedx{}
				err = json.Unmarshal([]byte(sbomTaskLog), sbom)
				Expect(err).NotTo(HaveOccurred(), "failed to parse SBOM from show-sbom task output")
				Expect(sbom.BomFormat).ToNot(BeEmpty())
				Expect(sbom.SpecVersion).ToNot(BeEmpty())
				Expect(len(sbom.Components)).To(BeNumerically(">=", 1))
			})

			When("Pipeline Results are stored", Label("pipeline"), func() {
				var resultClient *pipeline.ResultClient
				var pipelineRun *v1beta1.PipelineRun

				BeforeAll(func() {
					// create the proxyplugin for tekton-results
					_, err = kubeadminClient.CommonController.CreateProxyPlugin("tekton-results", "toolchain-host-operator", "tekton-results", "tekton-results")
					Expect(err).NotTo(HaveOccurred())

					regProxyUrl := fmt.Sprintf("%s/plugins/tekton-results", f.ProxyUrl)
					resultClient = pipeline.NewClient(regProxyUrl, f.UserToken)

					err = wait.Poll(pipelineCreatedRetryInterval, pipelineCreatedTimeout, func() (done bool, err error) {
						pipelineRun, err = f.AsKubeDeveloper.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
						if err != nil {
							time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
							return false, fmt.Errorf("deletion of PipelineRun has been timedout: %v", err)
						}
						return true, nil
					})
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("should have Pipeline Records", func() {
					records, err := resultClient.GetRecords(testNamespace, string(pipelineRun.GetUID()))
					// temporary logs due to RHTAPBUGS-213
					GinkgoWriter.Printf("records for PipelineRun %s:\n%s\n", pipelineRun.Name, records)
					Expect(err).NotTo(HaveOccurred(), "got error getting records for PipelineRun %s: %v", pipelineRun.Name, err)
					Expect(len(records.Record)).NotTo(BeZero(), "No records found for PipelineRun %s", pipelineRun.Name)
				})

				It("should have Pipeline Logs", func() {
					// Verify if result is stored in Database
					// temporary logs due to RHTAPBUGS-213
					logs, err := resultClient.GetLogs(testNamespace, string(pipelineRun.GetUID()))
					GinkgoWriter.Printf("logs for PipelineRun %s:\n%s\n", pipelineRun.Name, logs)
					Expect(err).NotTo(HaveOccurred(), "got error getting logs for PipelineRun %s: %v", pipelineRun.Name, err)

					timeout := time.Minute * 2
					interval := time.Second * 10
					// temporary timeout  due to RHTAPBUGS-213
					Eventually(func() (bool, error) {
						// temporary logs due to RHTAPBUGS-213
						logs, err = resultClient.GetLogs(testNamespace, string(pipelineRun.GetUID()))
						GinkgoWriter.Printf("logs for PipelineRun %s:\n%s\n", pipelineRun.Name, logs)
						Expect(err).NotTo(HaveOccurred(), "got error getting logs for PipelineRun %s: %v", pipelineRun.Name, err)

						return len(logs.Record) != 0, err
					}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when getting logs for PipelineRun %s", pipelineRun.Name))

					// Verify if result is stored in S3
					// temporary logs due to RHTAPBUGS-213
					log, err := resultClient.GetLogByName(logs.Record[0].Name)
					GinkgoWriter.Printf("log for record %s:\n%s\n", logs.Record[0].Name, log)
					Expect(err).NotTo(HaveOccurred(), "got error getting log '%s' for PipelineRun %s: %v", logs.Record[0].Name, pipelineRun.Name, err)
					Expect(len(log)).NotTo(BeZero(), "no log content '%s' found for PipelineRun %s", logs.Record[0].Name, pipelineRun.Name)
				})
			})

			It("should validate tekton taskrun test results", Label(buildTemplatesTestLabel), func() {
				pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())
				Expect(build.ValidateBuildPipelineTestResults(pipelineRun, kubeadminClient.CommonController.KubeRest())).To(Succeed())
			})

			When("the container image is created and pushed to container registry", Label("sbom", "slow"), func() {
				var outputImage string
				var kubeController tekton.KubeController
				BeforeAll(func() {
					pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
					Expect(err).ShouldNot(HaveOccurred())

					for _, p := range pipelineRun.Spec.Params {
						if p.Name == "output-image" {
							outputImage = p.Value.StringVal
						}
					}
					Expect(outputImage).ToNot(BeEmpty(), "output image of a component could not be found")

					kubeController = tekton.KubeController{
						Commonctrl: *kubeadminClient.CommonController,
						Tektonctrl: *kubeadminClient.TektonController,
						Namespace:  testNamespace,
					}
				})
				It("verify-enterprice-contract check should pass", Label(buildTemplatesTestLabel), func() {
					Skip("Skip until RHTAP bug is solved: https://issues.redhat.com/browse/RHTAPBUGS-352")
					cm, err := kubeController.Commonctrl.GetConfigMap("ec-defaults", "enterprise-contract-service")
					Expect(err).ToNot(HaveOccurred())

					verifyECTaskBundle := cm.Data["verify_ec_task_bundle"]
					Expect(verifyECTaskBundle).ToNot(BeEmpty())

					publicSecretName := "cosign-public-key"
					publicKey, err := kubeController.GetTektonChainsPublicKey()
					Expect(err).ToNot(HaveOccurred())

					Expect(kubeController.CreateOrUpdateSigningSecret(
						publicKey, publicSecretName, testNamespace)).To(Succeed())

					defaultEcp, err := kubeController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
					Expect(err).NotTo(HaveOccurred())

					policySource := defaultEcp.Spec.Sources
					policy := ecp.EnterpriseContractPolicySpec{
						Sources: policySource,
						Configuration: &ecp.EnterpriseContractPolicyConfiguration{
							// The BuildahDemo pipeline used to generate the test data does not
							// include the required test tasks, so this policy should always fail.
							Collections: []string{"slsa2"},
							Exclude:     []string{"cve"},
						},
					}
					Expect(kubeController.CreateOrUpdatePolicyConfiguration(testNamespace, policy)).To(Succeed())

					generator := tekton.VerifyEnterpriseContract{
						ApplicationName:     applicationName,
						Bundle:              verifyECTaskBundle,
						ComponentName:       componentNames[i],
						Image:               outputImage,
						Name:                "verify-enterprise-contract",
						Namespace:           testNamespace,
						PolicyConfiguration: "ec-policy",
						PublicKey:           fmt.Sprintf("k8s://%s/%s", testNamespace, publicSecretName),
						SSLCertDir:          "/var/run/secrets/kubernetes.io/serviceaccount",
						Strict:              true,
						EffectiveTime:       "now",
					}
					ecPipelineRunTimeout := int(time.Duration(10 * time.Minute).Seconds())
					pr, err := kubeController.RunPipeline(generator, ecPipelineRunTimeout)
					Expect(err).NotTo(HaveOccurred())

					Expect(kubeController.WatchPipelineRun(pr.Name, ecPipelineRunTimeout)).To(Succeed())

					pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
					Expect(err).NotTo(HaveOccurred())

					tr, err := kubeController.GetTaskRunStatus(kubeadminClient.CommonController.KubeRest(), pr, "verify-enterprise-contract")
					Expect(err).NotTo(HaveOccurred())
					Expect(tekton.DidTaskSucceed(tr)).To(BeTrue())
					Expect(tr.Status.TaskRunResults).Should(Or(
						// TODO: delete the first option after https://issues.redhat.com/browse/RHTAP-810 is completed
						ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.OldTektonTaskTestOutputName, "{$.result}", `["SUCCESS"]`)),
						ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["SUCCESS"]`)),
					))
				})
				It("contains non-empty sbom files", Label(buildTemplatesTestLabel), func() {

					purl, cyclonedx, err := build.GetParsedSbomFilesContentFromImage(outputImage)
					Expect(err).NotTo(HaveOccurred())

					Expect(cyclonedx.BomFormat).To(Equal("CycloneDX"))
					Expect(cyclonedx.SpecVersion).ToNot(BeEmpty())
					Expect(cyclonedx.Version).ToNot(BeZero())
					Expect(cyclonedx.Components).ToNot(BeEmpty())

					numberOfLibraryComponents := 0
					for _, component := range cyclonedx.Components {
						Expect(component.Name).ToNot(BeEmpty())
						Expect(component.Type).ToNot(BeEmpty())
						Expect(component.Version).ToNot(BeEmpty())

						if component.Type == "library" {
							Expect(component.Purl).ToNot(BeEmpty())
							numberOfLibraryComponents++
						}
					}

					Expect(purl.ImageContents.Dependencies).ToNot(BeEmpty())
					Expect(len(purl.ImageContents.Dependencies)).To(Equal(numberOfLibraryComponents))

					for _, dependency := range purl.ImageContents.Dependencies {
						Expect(dependency.Purl).ToNot(BeEmpty())
					}
				})
			})
		}
	})
})
