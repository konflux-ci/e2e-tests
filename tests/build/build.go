package build

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/utils/pointer"

	"github.com/google/go-github/v44/github"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"
)

const (
	containerImageSource                 = "quay.io/redhat-appstudio-qe/busybox-loop:latest"
	helloWorldComponentGitSourceRepoName = "devfile-sample-hello-world"
	pythonComponentGitSourceURL          = "https://github.com/redhat-appstudio-qe/devfile-sample-python-basic.git"
	dummyPipelineBundleRef               = "quay.io/redhat-appstudio-qe/dummy-pipeline-bundle:latest"
	buildTemplatesTestLabel              = "build-templates-e2e"
	buildTemplatesKcpTestLabel           = "build-templates-kcp-e2e"
)

var (
	componentUrls                   = strings.Split(utils.GetEnv(COMPONENT_REPO_URLS_ENV, pythonComponentGitSourceURL), ",") //multiple urls
	componentNames                  []string
	helloWorldComponentGitSourceURL = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), helloWorldComponentGitSourceRepoName)
)

var _ = framework.BuildSuiteDescribe("Build service E2E tests", Label("build", "HACBS"), func() {
	defer GinkgoRecover()
	f, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	Describe("the component with git source (GitHub) is created", Ordered, Label("github-webhook"), func() {
		var applicationName, componentName, pacBranchName, testNamespace, outputContainerImage, pacControllerHost string

		var timeout, interval time.Duration

		var prNumber int
		var prCreationTime time.Time

		BeforeAll(func() {
			consoleRoute, err := f.CommonController.GetOpenshiftRoute("console", "openshift-console")
			Expect(err).ShouldNot(HaveOccurred())

			if utils.IsPrivateHostname(consoleRoute.Spec.Host) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			// Used for identifying related webhook on GitHub - in order to delete it
			pacControllerRoute, err := f.CommonController.GetOpenshiftRoute("pipelines-as-code-controller", "pipelines-as-code")
			Expect(err).ShouldNot(HaveOccurred())
			pacControllerHost = pacControllerRoute.Spec.Host

			applicationName = fmt.Sprintf("build-suite-test-application-%s", util.GenerateRandomString(4))
			testNamespace = utils.GetGeneratedNamespace("build-e2e")

			_, err = f.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)

			app, err := f.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			componentName = fmt.Sprintf("%s-%s", "test-component-pac", util.GenerateRandomString(4))
			pacBranchName = fmt.Sprintf("appstudio-%s", componentName)
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images", utils.GetQuayIOOrganization())
			// TODO: test image naming with provided image tag
			// outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

			// Create a component with Git Source URL being defined
			_, err = f.HasController.CreateComponentWithPaCEnabled(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, outputContainerImage)
			Expect(err).ShouldNot(HaveOccurred())

		})

		AfterAll(func() {
			Expect(f.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
			Expect(f.HasController.DeleteHasComponent(componentName, testNamespace, false)).To(Succeed())
			pacInitTestFiles := []string{
				fmt.Sprintf(".tekton/%s-pull-request.yaml", componentName),
				fmt.Sprintf(".tekton/%s-push.yaml", componentName),
				fmt.Sprintf(".tekton/%s-readme.md", componentName),
			}

			for _, file := range pacInitTestFiles {
				err := f.CommonController.Github.DeleteFile(helloWorldComponentGitSourceRepoName, file, "main")
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("404 Not Found"))
				}
			}

			err = f.CommonController.Github.DeleteRef(helloWorldComponentGitSourceRepoName, pacBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}

			// Delete created webhook from GitHub
			hooks, err := f.CommonController.Github.ListRepoWebhooks(helloWorldComponentGitSourceRepoName)
			Expect(err).NotTo(HaveOccurred())

			for _, h := range hooks {
				hookUrl := h.Config["url"].(string)
				if strings.Contains(hookUrl, pacControllerHost) {
					Expect(f.CommonController.Github.DeleteWebhook(helloWorldComponentGitSourceRepoName, h.GetID())).To(Succeed())
					break
				}
			}
		})

		It("triggers a PipelineRun", func() {
			timeout = time.Second * 120
			interval = time.Second * 1
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, true, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
		})

		When("the PipelineRun has started", func() {
			It("should lead to a PaC init PR creation", func() {
				timeout = time.Second * 60
				interval = time.Second * 1

				Eventually(func() bool {
					prs, err := f.CommonController.Github.ListPullRequests(helloWorldComponentGitSourceRepoName)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == pacBranchName {
							prNumber = pr.GetNumber()
							prCreationTime = pr.GetCreatedAt()
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for init PaC PR to be created")

			})

			It("the PipelineRun should eventually finish successfully", func() {
				timeout = time.Second * 600
				interval = time.Second * 10
				Eventually(func() bool {

					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, true, "")
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if !pipelineRun.IsDone() {
							return false
						}

						if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
							failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
							d := utils.GetFailedPipelineRunDetails(pipelineRun)
							if d.FailedContainerName != "" {
								logs, _ := f.CommonController.GetContainerLogs(d.PodName, d.FailedContainerName, testNamespace)
								failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
							}
							Fail(failMessage)
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})
		})

		When("the PipelineRun is finished", func() {

			It("eventually leads to a creation of a PR comment with the PipelineRun status report", func() {
				var comments []*github.IssueComment
				timeout = time.Minute * 5
				interval = time.Second * 10

				Eventually(func() bool {
					comments, err = f.CommonController.Github.ListPullRequestCommentsSince(helloWorldComponentGitSourceRepoName, prNumber, prCreationTime)
					Expect(err).ShouldNot(HaveOccurred())

					return len(comments) != 0
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PaC PR comment about the pipelinerun status to appear in the component repo")

				// TODO uncomment once https://issues.redhat.com/browse/SRVKP-2471 is sorted
				//Expect(comments).To(HaveLen(1), fmt.Sprintf("the initial PR has more than 1 comment after a single pipelinerun. repo: %s, pr number: %d, comments content: %v", helloWorldComponentGitSourceURL, prNumber, comments))
				Expect(comments[len(comments)-1]).To(ContainSubstring("success"), "the initial PR doesn't contain the info about successful pipelinerun")
			})
		})

		When("the PaC init branch is updated", func() {

			var branchUpdateTimestamp time.Time
			var createdFileSHA string

			BeforeAll(func() {
				fileToCreatePath := fmt.Sprintf(".tekton/%s-readme.md", componentName)
				branchUpdateTimestamp = time.Now()
				createdFile, err := f.CommonController.Github.CreateFile(helloWorldComponentGitSourceRepoName, fileToCreatePath, fmt.Sprintf("test PaC branch %s update", pacBranchName), pacBranchName)
				Expect(err).NotTo(HaveOccurred())

				createdFileSHA = createdFile.GetSHA()
				GinkgoWriter.Println("created file sha:", createdFileSHA)
			})

			It("eventually leads to triggering another PipelineRun", func() {
				timeout = time.Minute * 2
				interval = time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, true, createdFileSHA)
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			})
			It("PipelineRun should eventually finish", func() {
				timeout = time.Minute * 5
				interval = time.Second * 10

				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, true, createdFileSHA)
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if !pipelineRun.IsDone() {
							return false
						}

						if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
							failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
							d := utils.GetFailedPipelineRunDetails(pipelineRun)
							if d.FailedContainerName != "" {
								logs, _ := f.CommonController.GetContainerLogs(d.PodName, d.FailedContainerName, testNamespace)
								failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
							}
							Fail(failMessage)
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})

			It("eventually leads to another update of a PR with a comment about the PipelineRun status report", func() {
				var comments []*github.IssueComment

				timeout = time.Minute * 5
				interval = time.Second * 5

				Eventually(func() bool {
					comments, err = f.CommonController.Github.ListPullRequestCommentsSince(helloWorldComponentGitSourceRepoName, prNumber, branchUpdateTimestamp)
					Expect(err).ShouldNot(HaveOccurred())

					return len(comments) != 0
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PaC PR comment about the pipelinerun status to appear in the component repo")

				// TODO uncomment once https://issues.redhat.com/browse/SRVKP-2471 is sorted
				//Expect(comments).To(HaveLen(1), fmt.Sprintf("the updated PaC PR has more than 1 comment after a single branch update. repo: %s, pr number: %d, comments content: %v", helloWorldComponentGitSourceURL, prNumber, comments))
				Expect(comments[len(comments)-1]).To(ContainSubstring("success"), "the updated PR doesn't contain the info about successful pipelinerun")
			})

		})

		When("the PaC init branch is merged", func() {
			var mergeResult *github.PullRequestMergeResult
			var mergeResultSha string

			BeforeAll(func() {
				Eventually(func() error {
					mergeResult, err = f.CommonController.Github.MergePullRequest(helloWorldComponentGitSourceRepoName, prNumber)
					return err
				}, time.Minute).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v", err))

				mergeResultSha = mergeResult.GetSHA()
				GinkgoWriter.Println("merged result sha:", mergeResultSha)
			})

			It("eventually leads to triggering another PipelineRun", func() {
				timeout = time.Minute * 2
				interval = time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, true, mergeResultSha)
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			})
			It("PipelineRun should eventually finish", func() {
				timeout = time.Minute * 5
				interval = time.Second * 10

				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, true, mergeResultSha)
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if !pipelineRun.IsDone() {
							return false
						}

						if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
							failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
							d := utils.GetFailedPipelineRunDetails(pipelineRun)
							if d.FailedContainerName != "" {
								logs, _ := f.CommonController.GetContainerLogs(d.PodName, d.FailedContainerName, testNamespace)
								failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
							}
							Fail(failMessage)
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})
		})

		When("the component is removed and recreated (with the same name in the same namespace)", func() {
			BeforeAll(func() {
				Expect(f.HasController.DeleteHasComponent(componentName, testNamespace, false)).To(Succeed())

				Eventually(func() bool {
					_, err := f.HasController.GetHasComponent(componentName, testNamespace)
					return errors.IsNotFound(err)
				}, time.Minute*1, time.Second*1).Should(BeTrue(), "timed out when waiting for the app %s to be deleted in %s namespace", applicationName, testNamespace)

				_, err = f.HasController.CreateComponentWithPaCEnabled(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, outputContainerImage)
			})

			It("should no longer lead to a creation of a PaC PR", func() {
				timeout = time.Second * 10
				interval = time.Second * 2
				Consistently(func() bool {
					prs, err := f.CommonController.Github.ListPullRequests(helloWorldComponentGitSourceRepoName)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == pacBranchName {
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeFalse(), "did not expect a new PR created after initial PaC configuration was already merged for the same component name and a namespace")
			})
		})
	})

	Describe("HACBS pipelines", Ordered, func() {

		var applicationName, componentName, testNamespace, outputContainerImage string
		BeforeAll(func() {
			if os.Getenv("APP_SUFFIX") != "" {
				applicationName = fmt.Sprintf("test-app-%s", os.Getenv("APP_SUFFIX"))
			} else {
				applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))
			}
			testNamespace = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, fmt.Sprintf("build-e2e-hacbs-%s", util.GenerateRandomString(4)))

			_, err := f.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)

			_, err = f.HasController.GetHasApplication(applicationName, testNamespace)
			// In case the app with the same name exist in the selected namespace, delete it first
			if err == nil {
				Expect(f.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
				Eventually(func() bool {
					_, err := f.HasController.GetHasApplication(applicationName, testNamespace)
					return errors.IsNotFound(err)
				}, time.Minute*5, time.Second*1).Should(BeTrue(), "timed out when waiting for the app %s to be deleted in %s namespace", applicationName, testNamespace)
			}
			app, err := f.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			for _, gitUrl := range componentUrls {
				gitUrl := gitUrl
				componentName = fmt.Sprintf("%s-%s", "test-component", util.GenerateRandomString(4))
				componentNames = append(componentNames, componentName)
				outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
				// Create a component with Git Source URL being defined
				_, err := f.HasController.CreateComponent(applicationName, componentName, testNamespace, gitUrl, "", "", outputContainerImage, "", false)
				Expect(err).ShouldNot(HaveOccurred())
			}
		})

		AfterAll(func() {
			// Do cleanup only in case the test succeeded
			if !CurrentSpecReport().Failed() {
				// Clean up only Application CR (Component and Pipelines are included) in case we are targeting specific namespace
				// Used e.g. in build-defintions e2e tests, where we are targeting build-templates-e2e namespace
				if os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV) != "" {
					DeferCleanup(f.HasController.DeleteHasApplication, applicationName, testNamespace, false)
				} else {
					Expect(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
					Expect(f.CommonController.DeleteNamespace(testNamespace)).To(Succeed())
				}
			}
		})

		for i, gitUrl := range componentUrls {
			gitUrl := gitUrl
			It(fmt.Sprintf("triggers PipelineRun for component with source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				timeout := time.Minute * 5
				interval := time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, false, "")
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the %s PipelineRun to start", componentNames[i])
			})
		}

		for i, gitUrl := range componentUrls {
			gitUrl := gitUrl

			It(fmt.Sprintf("should eventually finish successfully for component with source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				timeout := time.Second * 900
				interval := time.Second * 10
				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, false, "")
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if !pipelineRun.IsDone() {
							return false
						}

						if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
							failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
							d := utils.GetFailedPipelineRunDetails(pipelineRun)
							if d.FailedContainerName != "" {
								logs, _ := f.CommonController.GetContainerLogs(d.PodName, d.FailedContainerName, testNamespace)
								failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
							}
							Fail(failMessage)
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})

			It("should validate HACBS taskrun results", func() {
				// List Of Taskruns Expected to Get Taskrun Results
				gatherResult := []string{"conftest-clair", "sanity-inspect-image", "sanity-label-check"}
				// TODO: once we migrate "build" e2e tests to kcp, remove this condition
				// and add the 'sbom-json-check' taskrun to gatherResults slice
				s, _ := GinkgoConfiguration()
				if strings.Contains(s.LabelFilter, buildTemplatesKcpTestLabel) {
					gatherResult = append(gatherResult, "sbom-json-check")
				}
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentNames[0], applicationName, testNamespace, false, "")
				Expect(err).ShouldNot(HaveOccurred())

				for i := range gatherResult {
					if gatherResult[i] == "sanity-inspect-image" {
						result, err := build.FetchImageTaskRunResult(pipelineRun, gatherResult[i], "BASE_IMAGE")
						Expect(err).ShouldNot(HaveOccurred())
						ret := build.ValidateImageTaskRunResults(gatherResult[i], result)
						Expect(ret).Should(BeTrue())
						// TODO conftest-clair returns SUCCESS which is not expected
						// } else {
						// 	result, err := build.FetchTaskRunResult(pipelineRun, gatherResult[i], "HACBS_TEST_OUTPUT")
						// 	Expect(err).ShouldNot(HaveOccurred())
						// 	ret := build.ValidateTaskRunResults(gatherResult[i], result)
						// 	Expect(ret).Should(BeTrue())
					}
				}
			})

			When("the container image is created and pushed to container registry", Label("sbom", "slow"), func() {
				It("contains non-empty sbom files", func() {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentNames[0], applicationName, testNamespace, false, "")
					Expect(err).ShouldNot(HaveOccurred())

					var outputImage string
					// Workaround - until https://issues.redhat.com/browse/STONE-300 is solved
					for _, p := range pipelineRun.Spec.Params {
						if p.Name == "output-image" {
							outputImage = p.Value.StringVal
						}
					}
					Expect(outputImage).ToNot(BeEmpty(), "output image of a component could not be found")

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

	Describe("Creating component with container image source", Ordered, func() {

		var applicationName, componentName, testNamespace string
		var timeout, interval time.Duration

		BeforeAll(func() {

			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))
			testNamespace = fmt.Sprintf("build-e2e-container-image-source-%s", util.GenerateRandomString(4))

			_, err = f.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)
			DeferCleanup(f.CommonController.DeleteNamespace, testNamespace)

			app, err := f.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)
			DeferCleanup(f.HasController.DeleteHasApplication, applicationName, testNamespace, false)

			componentName = fmt.Sprintf("build-suite-test-component-image-source-%s", util.GenerateRandomString(4))
			outputContainerImage := ""
			timeout = time.Second * 180
			interval = time.Second * 1
			// Create a component with containerImageSource being defined
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, "", "", containerImageSource, outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace, testNamespace)
		})

		It("should not trigger a PipelineRun", func() {
			Consistently(func() bool {
				_, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
				Expect(err).NotTo(BeNil())
				return strings.Contains(err.Error(), "no pipelinerun found")
			}, timeout, interval).Should(BeTrue(), fmt.Sprintf("expected no PipelineRun to be triggered for the component %s in %s namespace", componentName, testNamespace))
		})
	})

	Describe("PLNSRVCE-799 - test pipeline selector", Label("pipeline-selector"), Ordered, func() {
		var timeout, interval time.Duration
		var componentName, applicationName, testNamespace, outputContainerImage string
		var expectedAdditionalPipelineParam buildservice.PipelineParam

		BeforeAll(func() {

			testNamespace = fmt.Sprintf("build-e2e-pipeline-selector-%s", util.GenerateRandomString(4))
			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))

			_, err = f.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)

			app, err := f.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			componentName = "build-suite-test-bundle-overriding"

			expectedAdditionalPipelineParam = buildservice.PipelineParam{
				Name:  "test-custom-param-name",
				Value: "test-custom-param-value",
			}

			ps := &buildservice.BuildPipelineSelector{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "build-pipeline-selector",
					Namespace: testNamespace,
				},
				Spec: buildservice.BuildPipelineSelectorSpec{Selectors: []buildservice.PipelineSelector{
					{
						Name: "user-custom-selector",
						PipelineRef: v1beta1.PipelineRef{
							Name:   "docker-build",
							Bundle: dummyPipelineBundleRef,
						},
						PipelineParams: []buildservice.PipelineParam{expectedAdditionalPipelineParam},
						WhenConditions: buildservice.WhenCondition{
							ProjectType:        "hello-world",
							DockerfileRequired: pointer.Bool(true),
							ComponentName:      componentName,
							Annotations:        constants.ComponentDefaultAnnotation,
							Labels:             constants.ComponentDefaultLabel,
						},
					},
				}},
			}

			Expect(f.CommonController.KubeRest().Create(context.TODO(), ps)).To(Succeed())

			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

			timeout = time.Second * 360
			interval = time.Second * 1

		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
				Expect(f.CommonController.DeleteNamespace(testNamespace)).To(Succeed())
			}
		})

		It("a specific Pipeline bundle should be used and additional pipeline params should be added to the PipelineRun if all WhenConditions match", func() {
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(dummyPipelineBundleRef))
			Expect(pipelineRun.Spec.Params).To(ContainElement(v1beta1.Param{
				Name:  expectedAdditionalPipelineParam.Name,
				Value: v1beta1.ParamValue{StringVal: expectedAdditionalPipelineParam.Value, Type: "string"}},
			))
		})

		It("default Pipeline bundle should be used and no additional Pipeline params should be added to the PipelineRun if one of the WhenConditions does not match", func() {
			notMatchingComponentName := componentName + util.GenerateRandomString(4)
			_, err = f.HasController.CreateComponent(applicationName, notMatchingComponentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(notMatchingComponentName, applicationName, testNamespace, false, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.HasController.GetComponentPipelineRun(notMatchingComponentName, applicationName, testNamespace, false, "")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).ToNot(Equal(dummyPipelineBundleRef))
			Expect(pipelineRun.Spec.Params).ToNot(ContainElement(v1beta1.Param{
				Name:  expectedAdditionalPipelineParam.Name,
				Value: v1beta1.ParamValue{StringVal: expectedAdditionalPipelineParam.Value, Type: "string"}},
			))
		})
	})

	Describe("A secret with dummy quay.io credentials is created in the testing namespace", Ordered, func() {

		var applicationName, componentName, testNamespace, outputContainerImage string
		var timeout, interval time.Duration

		BeforeAll(func() {

			testNamespace = fmt.Sprintf("build-e2e-dummy-quay-creds-%s", util.GenerateRandomString(4))

			_, err = f.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)
			DeferCleanup(f.CommonController.DeleteNamespace, testNamespace)

			_, err := f.CommonController.GetSecret(testNamespace, constants.RegistryAuthSecretName)
			if err != nil {
				// If we have an error when getting RegistryAuthSecretName, it should be IsNotFound err
				Expect(errors.IsNotFound(err)).To(BeTrue())
			} else {
				Skip("a registry auth secret is already created in testing namespace - skipping....")
			}

			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))

			app, err := f.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)
			DeferCleanup(f.HasController.DeleteHasApplication, applicationName, testNamespace, false)

			timeout = time.Minute * 5
			interval = time.Second * 1

			dummySecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: constants.RegistryAuthSecretName},
				Type:       v1.SecretTypeDockerConfigJson,
				Data:       map[string][]byte{".dockerconfigjson": []byte("{\"auths\":{\"quay.io\":{\"username\":\"test\",\"password\":\"test\",\"auth\":\"dGVzdDp0ZXN0\",\"email\":\"\"}}}")},
			}

			_, err = f.CommonController.CreateSecret(testNamespace, dummySecret)
			Expect(err).ToNot(HaveOccurred())

			componentName = "build-suite-test-secret-overriding"
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace, testNamespace)

		})

		It("should override the shared secret", func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
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
			timeout = time.Minute * 10
			interval = time.Second * 5
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
				Expect(err).ShouldNot(HaveOccurred())

				for _, condition := range pipelineRun.Status.Conditions {
					GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)
					return condition.Reason == "Failed"
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to fail")
		})
	})
})
