package build

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kubeapi "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/pipeline"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var _ = framework.BuildSuiteDescribe("Build service E2E tests", Label("build", "HACBS"), func() {
	var f *framework.Framework
	var err error

	defer GinkgoRecover()
	Describe("HACBS pipelines", Ordered, func() {

		var applicationName, componentName, testNamespace, outputContainerImage string
		var kubeadminClient *framework.ControllerHub

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
			Expect(utils.WaitUntil(kubeadminClient.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			for _, gitUrl := range componentUrls {
				gitUrl := gitUrl
				componentName = fmt.Sprintf("%s-%s", "test-component", util.GenerateRandomString(4))
				componentNames = append(componentNames, componentName)
				outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
				// Create a component with Git Source URL being defined
				_, err := kubeadminClient.HasController.CreateComponent(applicationName, componentName, testNamespace, gitUrl, "", "", outputContainerImage, "", false)
				Expect(err).ShouldNot(HaveOccurred())
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
				timeout := time.Second * 1800
				interval := time.Second * 10
				Eventually(func() bool {
					pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if !pipelineRun.IsDone() {
							return false
						}

						if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
							failMessage := tekton.GetFailedPipelineRunLogs(kubeadminClient.CommonController, pipelineRun)
							Fail(failMessage)
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})

			It("should validate Tekton Results", func() {
				// Create an Service account and a token associating it with the service account
				resultSA := "tekton-results-tests"
				_, err := kubeadminClient.CommonController.CreateServiceAccount(resultSA, testNamespace, nil)
				Expect(err).NotTo(HaveOccurred())
				_, err = kubeadminClient.CommonController.CreateRoleBinding("tekton-results-tests", testNamespace, "ServiceAccount", resultSA, "ClusterRole", "tekton-results-readonly", "rbac.authorization.k8s.io")
				Expect(err).NotTo(HaveOccurred())

				resultSecret := &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "tekton-results-tests",
						Annotations: map[string]string{"kubernetes.io/service-account.name": resultSA},
					},
					Type: v1.SecretTypeServiceAccountToken,
				}

				_, err = kubeadminClient.CommonController.CreateSecret(testNamespace, resultSecret)
				Expect(err).ToNot(HaveOccurred())
				err = kubeadminClient.CommonController.LinkSecretToServiceAccount(testNamespace, resultSecret.Name, resultSA, false)
				Expect(err).ToNot(HaveOccurred())

				resultSecret, err = kubeadminClient.CommonController.GetSecret(testNamespace, resultSecret.Name)
				Expect(err).ToNot(HaveOccurred())
				token := resultSecret.Data["token"]

				// Retrive Result REST API url
				resultRoute, err := kubeadminClient.CommonController.GetOpenshiftRoute("tekton-results", "tekton-results")
				Expect(err).NotTo(HaveOccurred())
				resultUrl := fmt.Sprintf("https://%s", resultRoute.Spec.Host)
				resultClient := pipeline.NewClient(resultUrl, string(token))

				pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())

				// Verify Records
				records, err := resultClient.GetRecords(testNamespace, string(pipelineRun.GetUID()))
				Expect(err).NotTo(HaveOccurred())
				Expect(len(records.Record)).NotTo(BeZero())
				// Verify Logs
				logs, err := resultClient.GetLogs(testNamespace, string(pipelineRun.GetUID()))
				Expect(err).NotTo(HaveOccurred())
				Expect(len(logs.Record)).NotTo(BeZero())
			})

			It("should validate HACBS taskrun results", func() {
				// List Of Taskruns Expected to Get Taskrun Results
				gatherResult := []string{"clair-scan", "inspect-image", "label-check", "sbom-json-check"}
				// TODO: once we migrate "build" e2e tests to kcp, remove this condition
				// and add the 'sbom-json-check' taskrun to gatherResults slice
				s, _ := GinkgoConfiguration()
				if strings.Contains(s.LabelFilter, buildTemplatesKcpTestLabel) {
					gatherResult = append(gatherResult, "sbom-json-check")
				}
				pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[0], applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())

				for i := range gatherResult {
					if gatherResult[i] == "inspect-image" {
						// Fetching BASE_IMAGE shouldn't fail
						result, err := build.FetchImageTaskRunResult(pipelineRun, gatherResult[i], "BASE_IMAGE")
						Expect(err).ShouldNot(HaveOccurred())
						ret := build.ValidateImageTaskRunResults(gatherResult[i], result)
						Expect(ret).Should(BeTrue())
					} else if gatherResult[i] == "clair-scan" {
						// Fetching HACBS_TEST_OUTPUT shouldn't fail
						result, err := build.FetchTaskRunResult(pipelineRun, gatherResult[i], "HACBS_TEST_OUTPUT")
						Expect(err).ShouldNot(HaveOccurred())
						ret := build.ValidateTaskRunResults(gatherResult[i], result)
						// Vulnerabilities should get periodically eliminated with image rebuild, so the result of that task might be different
						// This should not block e2e tests with errors.
						GinkgoWriter.Printf("retcode for validate taskrun result is %s\n", ret)
					} else {
						// Fetching HACBS_TEST_OUTPUT shouldn't fail
						result, err := build.FetchTaskRunResult(pipelineRun, gatherResult[i], "HACBS_TEST_OUTPUT")
						Expect(err).ShouldNot(HaveOccurred())
						ret := build.ValidateTaskRunResults(gatherResult[i], result)
						Expect(ret).Should(BeTrue())
					}
				}
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
					}
					ecPipelineRunTimeout := int(time.Duration(10 * time.Minute).Seconds())
					pr, err := kubeController.RunPipeline(generator, ecPipelineRunTimeout)
					Expect(err).NotTo(HaveOccurred())

					Expect(kubeController.WatchPipelineRun(pr.Name, ecPipelineRunTimeout)).To(Succeed())

					pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.Name, pr.Namespace)
					Expect(err).NotTo(HaveOccurred())

					tr, err := kubeController.GetTaskRunStatus(pr, "verify-enterprise-contract")
					Expect(err).NotTo(HaveOccurred())
					Expect(tekton.DidTaskSucceed(tr)).To(BeTrue())
					Expect(tr.Status.TaskRunResults).Should(ContainElements(
						tekton.MatchTaskRunResultWithJSONPathValue("HACBS_TEST_OUTPUT", "{$.result}", `["SUCCESS"]`),
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
