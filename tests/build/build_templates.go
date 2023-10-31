package build

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	kubeapi "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/pipeline"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"
)

var (
	ecPipelineRunTimeout     = time.Duration(10 * time.Minute)
	chainsAttestationTimeout = time.Duration(10 * time.Minute)
)

const pipelineCompletionRetries = 2

var _ = framework.BuildSuiteDescribe("Build templates E2E test", Label("build", "HACBS"), func() {
	var f *framework.Framework
	var err error
	AfterEach(framework.ReportFailure(&f))

	defer GinkgoRecover()
	Describe("HACBS pipelines", Ordered, Label("pipeline"), func() {

		var applicationName, componentName, symlinkComponentName, testNamespace string
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

			_, err = kubeadminClient.HasController.GetApplication(applicationName, testNamespace)
			// In case the app with the same name exist in the selected namespace, delete it first
			if err == nil {
				Expect(kubeadminClient.HasController.DeleteApplication(applicationName, testNamespace, false)).To(Succeed())
				Eventually(func() bool {
					_, err := kubeadminClient.HasController.GetApplication(applicationName, testNamespace)
					return errors.IsNotFound(err)
				}, time.Minute*5, time.Second*1).Should(BeTrue(), fmt.Sprintf("timed out when waiting for the app %s to be deleted in %s namespace", applicationName, testNamespace))
			}
			app, err := kubeadminClient.HasController.CreateApplication(applicationName, testNamespace)
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
				Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "Expected length of the detected Components was not 1")

				for _, compDetected := range cdq.Status.ComponentDetected {
					c, err := kubeadminClient.HasController.CreateComponent(compDetected.ComponentStub, testNamespace, "", "", applicationName, false, map[string]string{})
					Expect(err).ShouldNot(HaveOccurred())
					componentNames = append(componentNames, c.Name)
				}
			}

			// Create component for the repo containing symlink
			symlinkComponentName = fmt.Sprintf("%s-%s", "test-symlink-comp", util.GenerateRandomString(4))
			cdq, err := kubeadminClient.HasController.CreateComponentDetectionQuery(symlinkComponentName, testNamespace, pythonComponentGitSourceURL, gitRepoContainsSymlinkBranchName, "", "", false)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "Expected length of the detected Components was not 1")

			for _, compDetected := range cdq.Status.ComponentDetected {
				c, err := kubeadminClient.HasController.CreateComponent(compDetected.ComponentStub, testNamespace, "", "", applicationName, false, map[string]string{})
				Expect(err).ShouldNot(HaveOccurred())
				symlinkComponentName = c.Name
			}
		})

		AfterAll(func() {
			// Do cleanup only in case the test succeeded
			if !CurrentSpecReport().Failed() {
				// Clean up only Application CR (Component and Pipelines are included) in case we are targeting specific namespace
				// Used e.g. in build-definitions e2e tests, where we are targeting build-templates-e2e namespace
				if os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV) != "" {
					DeferCleanup(kubeadminClient.HasController.DeleteApplication, applicationName, testNamespace, false)
				} else {
					Expect(kubeadminClient.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
					Expect(f.SandboxController.DeleteUserSignup(f.UserName)).To(BeTrue())
				}
			}
		})

		for i, gitUrl := range componentUrls {
			i := i
			gitUrl := gitUrl
			It(fmt.Sprintf("triggers PipelineRun for component with source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				timeout := time.Minute * 5

				Eventually(func() error {
					pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for Component %s/%s\n", testNamespace, componentNames[i])
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s has not started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the Component %s/%s", testNamespace, componentNames[i]))
			})

			It(fmt.Sprintf("should eventually finish successfully for component with Git source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				component, err := kubeadminClient.HasController.GetComponent(componentNames[i], testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kubeadminClient.HasController.WaitForComponentPipelineToBeFinished(component, "", pipelineCompletionRetries, kubeadminClient.TektonController)).To(Succeed())
			})

			It(fmt.Sprintf("should ensure SBOM is shown for component with Git source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				pr, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())
				Expect(pr).ToNot(BeNil(), fmt.Sprintf("PipelineRun for the component %s/%s not found", testNamespace, componentNames[i]))

				logs, err := kubeadminClient.TektonController.GetTaskRunLogs(pr.GetName(), "show-sbom", testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(logs).To(HaveLen(1))
				var sbomTaskLog string
				for _, log := range logs {
					sbomTaskLog = log
				}

				sbom := &build.SbomCyclonedx{}
				err = json.Unmarshal([]byte(sbomTaskLog), sbom)
				Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("failed to parse SBOM from show-sbom task output from %s/%s PipelineRun", pr.GetNamespace(), pr.GetName()))
				Expect(sbom.BomFormat).ToNot(BeEmpty())
				Expect(sbom.SpecVersion).ToNot(BeEmpty())
				Expect(sbom.Components).ToNot(BeEmpty())
			})

			When(fmt.Sprintf("Pipeline Results are stored for component with Git source URL %s", gitUrl), Label("pipeline"), func() {
				var resultClient *pipeline.ResultClient
				var pr *v1beta1.PipelineRun

				BeforeAll(func() {
					// create the proxyplugin for tekton-results
					_, err = kubeadminClient.CommonController.CreateProxyPlugin("tekton-results", "toolchain-host-operator", "tekton-results", "tekton-results")
					Expect(err).NotTo(HaveOccurred())

					regProxyUrl := fmt.Sprintf("%s/plugins/tekton-results", f.ProxyUrl)
					resultClient = pipeline.NewClient(regProxyUrl, f.UserToken)

					pr, err = kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
					Expect(err).ShouldNot(HaveOccurred())
				})

				AfterAll(func() {
					Expect(kubeadminClient.CommonController.DeleteProxyPlugin("tekton-results", "toolchain-host-operator")).To(BeTrue())
				})

				It("should have Pipeline Records", func() {
					records, err := resultClient.GetRecords(testNamespace, string(pr.GetUID()))
					// temporary logs due to RHTAPBUGS-213
					GinkgoWriter.Printf("records for PipelineRun %s:\n%s\n", pr.Name, records)
					Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("got error getting records for PipelineRun %s: %v", pr.Name, err))
					Expect(records.Record).NotTo(BeEmpty(), fmt.Sprintf("No records found for PipelineRun %s", pr.Name))
				})

				It("should have Pipeline Logs", func() {
					// Verify if result is stored in Database
					// temporary logs due to RHTAPBUGS-213
					logs, err := resultClient.GetLogs(testNamespace, string(pr.GetUID()))
					GinkgoWriter.Printf("logs for PipelineRun %s:\n%s\n", pr.GetName(), logs)
					Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("got error getting logs for PipelineRun %s: %v", pr.Name, err))

					timeout := time.Minute * 2
					interval := time.Second * 10
					// temporary timeout  due to RHTAPBUGS-213
					Eventually(func() error {
						// temporary logs due to RHTAPBUGS-213
						logs, err = resultClient.GetLogs(testNamespace, string(pr.GetUID()))
						if err != nil {
							return fmt.Errorf("failed to get logs for PipelineRun %s: %v", pr.Name, err)
						}
						GinkgoWriter.Printf("logs for PipelineRun %s:\n%s\n", pr.Name, logs)

						if len(logs.Record) == 0 {
							return fmt.Errorf("logs for PipelineRun %s/%s are empty", pr.GetNamespace(), pr.GetName())
						}
						return nil
					}, timeout, interval).Should(Succeed(), fmt.Sprintf("timed out when getting logs for PipelineRun %s/%s", pr.GetNamespace(), pr.GetName()))

					// Verify if result is stored in S3
					// temporary logs due to RHTAPBUGS-213
					log, err := resultClient.GetLogByName(logs.Record[0].Name)
					GinkgoWriter.Printf("log for record %s:\n%s\n", logs.Record[0].Name, log)
					Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("got error getting log '%s' for PipelineRun %s: %v", logs.Record[0].Name, pr.GetName(), err))
					Expect(log).NotTo(BeEmpty(), fmt.Sprintf("no log content '%s' found for PipelineRun %s", logs.Record[0].Name, pr.GetName()))
				})
			})

			It(fmt.Sprintf("should validate tekton taskrun test results for component with Git source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				pr, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())
				Expect(build.ValidateBuildPipelineTestResults(pr, kubeadminClient.CommonController.KubeRest())).To(Succeed())
			})

			When(fmt.Sprintf("the container image for component with Git source URL %s is created and pushed to container registry", gitUrl), Label("sbom", "slow"), Pending, func() {
				var imageWithDigest string
				var pr *v1beta1.PipelineRun

				BeforeAll(func() {
					var err error
					imageWithDigest, err = getImageWithDigest(kubeadminClient, componentNames[i], applicationName, testNamespace)
					Expect(err).NotTo(HaveOccurred())
				})
				AfterAll(func() {
					if !CurrentSpecReport().Failed() {
						Expect(kubeadminClient.TektonController.DeletePipelineRun(pr.GetName(), pr.GetNamespace())).To(Succeed())
					}
				})

				It("verify-enterprise-contract check should pass", Label(buildTemplatesTestLabel), func() {
					// If the Tekton Chains controller is busy, it may take longer than usual for it
					// to sign and attest the image built in BeforeAll.
					err = kubeadminClient.TektonController.AwaitAttestationAndSignature(imageWithDigest, chainsAttestationTimeout)
					Expect(err).ToNot(HaveOccurred())

					cm, err := kubeadminClient.CommonController.GetConfigMap("ec-defaults", "enterprise-contract-service")
					Expect(err).ToNot(HaveOccurred())

					verifyECTaskBundle := cm.Data["verify_ec_task_bundle"]
					Expect(verifyECTaskBundle).ToNot(BeEmpty())

					publicSecretName := "cosign-public-key"
					publicKey, err := kubeadminClient.TektonController.GetTektonChainsPublicKey()
					Expect(err).ToNot(HaveOccurred())

					Expect(kubeadminClient.TektonController.CreateOrUpdateSigningSecret(
						publicKey, publicSecretName, testNamespace)).To(Succeed())

					defaultEcp, err := kubeadminClient.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
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
					Expect(kubeadminClient.TektonController.CreateOrUpdatePolicyConfiguration(testNamespace, policy)).To(Succeed())

					pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, "")
					Expect(err).ToNot(HaveOccurred())

					rev := pipelineRun.Annotations["pipelinesascode.tekton.dev/sha"]

					generator := tekton.VerifyEnterpriseContract{
						Snapshot: v1alpha1.SnapshotSpec{
							Application: applicationName,
							Components: []v1alpha1.SnapshotComponent{
								{
									Name:           componentNames[i],
									ContainerImage: imageWithDigest,
									Source: v1alpha1.ComponentSource{
										ComponentSourceUnion: v1alpha1.ComponentSourceUnion{
											GitSource: &v1alpha1.GitSource{
												URL:      gitUrl,
												Revision: rev,
											},
										},
									},
								},
							},
						},
						TaskBundle:          verifyECTaskBundle,
						Name:                "verify-enterprise-contract",
						Namespace:           testNamespace,
						PolicyConfiguration: "ec-policy",
						PublicKey:           fmt.Sprintf("k8s://%s/%s", testNamespace, publicSecretName),
						Strict:              true,
						EffectiveTime:       "now",
						IgnoreRekor:         true,
					}

					pr, err = kubeadminClient.TektonController.RunPipeline(generator, testNamespace, int(ecPipelineRunTimeout.Seconds()))
					Expect(err).NotTo(HaveOccurred())

					Expect(kubeadminClient.TektonController.WatchPipelineRun(pr.Name, testNamespace, int(ecPipelineRunTimeout.Seconds()))).To(Succeed())

					pr, err = kubeadminClient.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
					Expect(err).NotTo(HaveOccurred())

					tr, err := kubeadminClient.TektonController.GetTaskRunStatus(kubeadminClient.CommonController.KubeRest(), pr, "verify-enterprise-contract")
					Expect(err).NotTo(HaveOccurred())
					Expect(tekton.DidTaskRunSucceed(tr)).To(BeTrue())
					Expect(tr.Status.TaskRunResults).Should(Or(
						// TODO: delete the first option after https://issues.redhat.com/browse/RHTAP-810 is completed
						ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.OldTektonTaskTestOutputName, "{$.result}", `["SUCCESS"]`)),
						ContainElements(tekton.MatchTaskRunResultWithJSONPathValue(constants.TektonTaskTestOutputName, "{$.result}", `["SUCCESS"]`)),
					))
				})
				It("contains non-empty sbom files", Label(buildTemplatesTestLabel), func() {
					purl, cyclonedx, err := build.GetParsedSbomFilesContentFromImage(imageWithDigest)
					Expect(err).NotTo(HaveOccurred())

					Expect(cyclonedx.BomFormat).To(Equal("CycloneDX"))
					Expect(cyclonedx.SpecVersion).ToNot(BeEmpty())
					Expect(cyclonedx.Version).ToNot(BeZero())
					Expect(cyclonedx.Components).ToNot(BeEmpty())

					numberOfLibraryComponents := 0
					for _, component := range cyclonedx.Components {
						Expect(component.Name).ToNot(BeEmpty())
						Expect(component.Type).ToNot(BeEmpty())

						if component.Type == "library" || component.Type == "application" {
							Expect(component.Purl).ToNot(BeEmpty())
							numberOfLibraryComponents++
						}
					}

					Expect(purl.ImageContents.Dependencies).ToNot(BeEmpty())
					Expect(purl.ImageContents.Dependencies).To(HaveLen(numberOfLibraryComponents))

					for _, dependency := range purl.ImageContents.Dependencies {
						Expect(dependency.Purl).ToNot(BeEmpty())
					}
				})
			})

			Context("build-definitions ec pipelines", Label(buildTemplatesTestLabel), func() {
				ecPipelines := []string{
					"pipelines/enterprise-contract.yaml",
					"pipelines/enterprise-contract-everything.yaml",
					"pipelines/enterprise-contract-redhat.yaml",
					"pipelines/enterprise-contract-redhat-no-hermetic.yaml",
					"pipelines/enterprise-contract-slsa1.yaml",
					"pipelines/enterprise-contract-slsa2.yaml",
					"pipelines/enterprise-contract-slsa3.yaml",
				}

				var gitRevision, gitURL, imageWithDigest string

				defaultGHOrg := "redhat-appstudio"
				defaultGHRepo := "build-definitions"
				defaultGitURL := fmt.Sprintf("https://github.com/%s/%s", defaultGHOrg, defaultGHRepo)
				defaultGitRevision := "main"

				BeforeAll(func() {
					// If we are testing the changes from a pull request, APP_SUFFIX may contain the
					// pull request ID. If it looks like an ID, then fetch information about the pull
					// request and use it to determine which git URL and revision to use for the EC
					// pipelines. NOTE: This is a workaround until Pipeline as Code supports passing
					// the source repo URL: https://issues.redhat.com/browse/SRVKP-3427. Once that's
					// implemented, remove the APP_SUFFIX support below and simply rely on the other
					// environment variables to set the git revision and URL directly.
					appSuffix := os.Getenv("APP_SUFFIX")
					if pullRequestID, err := strconv.ParseInt(appSuffix, 10, 64); err == nil {
						gh, err := github.NewGithubClient(utils.GetEnv(constants.GITHUB_TOKEN_ENV, ""), defaultGHOrg)
						Expect(err).NotTo(HaveOccurred())
						pullRequest, err := gh.GetPullRequest(defaultGHRepo, int(pullRequestID))
						Expect(err).NotTo(HaveOccurred())
						gitURL = *pullRequest.Head.Repo.CloneURL
						gitRevision = *pullRequest.Head.Ref
					} else {
						gitRevision = utils.GetEnv(constants.EC_PIPELINES_REPO_REVISION_ENV, defaultGitRevision)
						gitURL = utils.GetEnv(constants.EC_PIPELINES_REPO_URL_ENV, defaultGitURL)
					}

					// Double check that the component has finished. There's an earlier test that
					// verifies this so this should be a no-op. It is added here in order to avoid
					// unnecessary coupling of unrelated tests.
					component, err := kubeadminClient.HasController.GetComponent(componentNames[i], testNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(kubeadminClient.HasController.WaitForComponentPipelineToBeFinished(
						component, "", pipelineCompletionRetries, kubeadminClient.TektonController)).To(Succeed())

					imageWithDigest, err = getImageWithDigest(kubeadminClient, componentNames[i], applicationName, testNamespace)
					Expect(err).NotTo(HaveOccurred())

					err = kubeadminClient.TektonController.AwaitAttestationAndSignature(imageWithDigest, chainsAttestationTimeout)
					Expect(err).NotTo(HaveOccurred())
				})

				for _, pathInRepo := range ecPipelines {
					pathInRepo := pathInRepo
					It(fmt.Sprintf("runs ec pipeline %s", pathInRepo), func() {
						generator := tekton.ECIntegrationTestScenario{
							Image:                 imageWithDigest,
							Namespace:             testNamespace,
							PipelineGitURL:        gitURL,
							PipelineGitRevision:   gitRevision,
							PipelineGitPathInRepo: pathInRepo,
						}

						pr, err := kubeadminClient.TektonController.RunPipeline(generator, testNamespace, int(ecPipelineRunTimeout.Seconds()))
						Expect(err).NotTo(HaveOccurred())
						defer func(pr *v1beta1.PipelineRun) {
							// Avoid blowing up PipelineRun usage
							err := kubeadminClient.TektonController.DeletePipelineRun(pr.Name, pr.Namespace)
							Expect(err).NotTo(HaveOccurred())
						}(pr)
						Expect(kubeadminClient.TektonController.WatchPipelineRun(pr.Name, testNamespace, int(ecPipelineRunTimeout.Seconds()))).To(Succeed())

						// Refresh our copy of the PipelineRun for latest results
						pr, err = kubeadminClient.TektonController.GetPipelineRun(pr.Name, pr.Namespace)
						Expect(err).NotTo(HaveOccurred())

						// The UI uses this label to display additional information.
						Expect(pr.Labels["build.appstudio.redhat.com/pipeline"]).To(Equal("enterprise-contract"))

						// The UI uses this label to display additional information.
						tr, err := kubeadminClient.TektonController.GetTaskRunFromPipelineRun(kubeadminClient.CommonController.KubeRest(), pr, "verify")
						Expect(err).NotTo(HaveOccurred())
						Expect(tr.Labels["build.appstudio.redhat.com/pipeline"]).To(Equal("enterprise-contract"))

						logs, err := kubeadminClient.TektonController.GetTaskRunLogs(pr.Name, "verify", pr.Namespace)
						Expect(err).NotTo(HaveOccurred())

						// The logs from the report step are used by the UI to display validation
						// details. Let's make sure it has valid YAML.
						reportLogs := logs["step-report"]
						Expect(reportLogs).NotTo(BeEmpty())
						var reportYAML any
						err = yaml.Unmarshal([]byte(reportLogs), &reportYAML)
						Expect(err).NotTo(HaveOccurred())

						// The logs from the summary step are used by the UI to display an overview of
						// the validation.
						summaryLogs := logs["step-summary"]
						Expect(summaryLogs).NotTo(BeEmpty())
						var summary build.TestOutput
						err = json.Unmarshal([]byte(summaryLogs), &summary)
						Expect(err).NotTo(HaveOccurred())
						Expect(summary).NotTo(Equal(build.TestOutput{}))
					})
				}
			})
		}

		It(fmt.Sprintf("triggers PipelineRun for symlink component with source URL %s", pythonComponentGitSourceURL), Label(buildTemplatesTestLabel), func() {
			timeout := time.Minute * 5

			Eventually(func() error {
				pipelineRun, err := kubeadminClient.HasController.GetComponentPipelineRun(symlinkComponentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Printf("PipelineRun has not been created yet for symlink Component %s/%s\n", testNamespace, symlinkComponentName)
					return err
				}
				if !pipelineRun.HasStarted() {
					return fmt.Errorf("pipelinerun %s/%s has not started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
				}
				return nil
			}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the Component %s/%s", testNamespace, symlinkComponentName))
		})

		It(fmt.Sprintf("pipelineRun should fail for symlink component with Git source URL %s", pythonComponentGitSourceURL), Label(buildTemplatesTestLabel), func() {
			component, err := kubeadminClient.HasController.GetComponent(symlinkComponentName, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(kubeadminClient.HasController.WaitForComponentPipelineToBeFinished(component, "", pipelineCompletionRetries, kubeadminClient.TektonController)).Should(MatchError(ContainSubstring("cloned repository contains symlink pointing outside of the cloned repository")))
		})
	})
})

func getImageWithDigest(c *framework.ControllerHub, componentName, applicationName, namespace string) (string, error) {
	var url string
	var digest string
	pipelineRun, err := c.HasController.GetComponentPipelineRun(componentName, applicationName, namespace, "")
	if err != nil {
		return "", err
	}

	for _, p := range pipelineRun.Spec.Params {
		if p.Name == "output-image" {
			url = p.Value.StringVal
		}
	}
	if url == "" {
		return "", fmt.Errorf("output-image of a component %q could not be found", componentName)
	}

	for _, r := range pipelineRun.Status.PipelineResults {
		if r.Name == "IMAGE_DIGEST" {
			digest = r.Value.StringVal
		}
	}
	if digest == "" {
		return "", fmt.Errorf("IMAGE_DIGEST for component %q could not be found", componentName)
	}
	return fmt.Sprintf("%s@%s", url, digest), nil
}
