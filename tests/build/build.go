package build

import (
	"fmt"
	"os"
	"strings"
	"time"

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
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"knative.dev/pkg/apis"
)

const (
	containerImageSource                 = "quay.io/redhat-appstudio-qe/busybox-loop:latest"
	helloWorldComponentGitSourceRepoName = "devfile-sample-hello-world"
	pythonComponentGitSourceURL          = "https://github.com/redhat-appstudio-qe/devfile-sample-python-basic"
	dummyPipelineBundleRef               = "quay.io/redhat-appstudio-qe/dummy-pipeline-bundle:latest"
	buildTemplatesTestLabel              = "build-templates-e2e"
)

var (
	componentUrls                   = strings.Split(utils.GetEnv(COMPONENT_REPO_URLS_ENV, pythonComponentGitSourceURL), ",") //multiple urls
	componentNames                  []string
	helloWorldComponentGitSourceURL = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv("GITHUB_E2E_ORGANIZATION", "redhat-appstudio-qe"), helloWorldComponentGitSourceRepoName)
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
				klog.Infoln(hookUrl, pacControllerHost)
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
					klog.Infoln("PipelineRun has not been created yet")
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
						klog.Infof("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

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
				klog.Infoln("created file sha:", createdFileSHA)

			})

			It("eventually leads to triggering another PipelineRun", func() {
				timeout = time.Minute * 2
				interval = time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, true, createdFileSHA)
					if err != nil {
						klog.Infoln("PipelineRun has not been created yet")
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
						klog.Infof("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

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
				klog.Infoln("merged result sha:", mergeResultSha)
			})

			It("eventually leads to triggering another PipelineRun", func() {
				timeout = time.Minute * 2
				interval = time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, true, mergeResultSha)
					if err != nil {
						klog.Infoln("PipelineRun has not been created yet")
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
						klog.Infof("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

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
		var defaultBundleConfigMap *v1.ConfigMap
		var defaultBundleRef, customBundleRef string

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

			customBundleConfigMap, err := f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, testNamespace)
			if err != nil {
				if errors.IsNotFound(err) {
					defaultBundleConfigMap, err = f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace)
					Expect(err).ToNot(HaveOccurred())

					bundlePullSpec := defaultBundleConfigMap.Data["default_build_bundle"]
					hacbsBundleConfigMap := &v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: constants.BuildPipelinesConfigMapName},
						Data:       map[string]string{"default_build_bundle": strings.Replace(bundlePullSpec, "build-", "hacbs-", 1)},
					}
					_, err = f.CommonController.CreateConfigMap(hacbsBundleConfigMap, testNamespace)
					Expect(err).ToNot(HaveOccurred())
					DeferCleanup(f.CommonController.DeleteConfigMap, constants.BuildPipelinesConfigMapName, testNamespace, false)
				} else {
					Fail(fmt.Sprintf("error occured when trying to get configmap %s in %s namespace: %v", constants.BuildPipelinesConfigMapName, testNamespace, err))
				}
			} else {
				bundlePullSpec := customBundleConfigMap.Data["default_build_bundle"]
				hacbsBundleConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: constants.BuildPipelinesConfigMapName},
					Data:       map[string]string{"default_build_bundle": bundlePullSpec},
				}

				_, err = f.CommonController.UpdateConfigMap(hacbsBundleConfigMap, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				DeferCleanup(func() error {
					hacbsBundleConfigMap.Data = customBundleConfigMap.Data
					_, err := f.CommonController.UpdateConfigMap(hacbsBundleConfigMap, testNamespace)
					if err != nil {
						return err
					}
					return nil
				})
			}

			for _, gitUrl := range componentUrls {
				gitUrl := gitUrl
				componentName = fmt.Sprintf("%s-%s", "test-component", util.GenerateRandomString(4))
				componentNames = append(componentNames, componentName)
				outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
				// Create a component with Git Source URL being defined
				_, err := f.HasController.CreateComponent(applicationName, componentName, testNamespace, gitUrl, "", "", outputContainerImage, "")
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
			} else {
				// Workaround: We cannot keep applications/components present in the specific namespace due to
				// an issue reported here: https://issues.redhat.com/browse/PLNSRVCE-484
				// TODO: delete the whole 'else' block after the issue is resolved
				if os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV) != "" {
					DeferCleanup(f.HasController.DeleteHasApplication, applicationName, testNamespace, false)
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
						klog.Infoln("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the %s PipelineRun to start", componentNames[i])
			})
		}

		It("should reference the custom pipeline bundle in a PipelineRun", Label(buildTemplatesTestLabel), func() {
			customBundleConfigMap, err := f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, testNamespace)
			if err != nil {
				if errors.IsNotFound(err) {
					klog.Infof("configmap with custom pipeline bundle not found in %s namespace\n", testNamespace)
				} else {
					Fail(fmt.Sprintf("error occured when trying to get configmap %s in %s namespace: %v", constants.BuildPipelinesConfigMapName, testNamespace, err))
				}
			} else {
				customBundleRef = customBundleConfigMap.Data["default_build_bundle"]
			}

			if customBundleRef == "" {
				Skip("skipping the specs - custom pipeline bundle is not defined")
			}
			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentNames[0], applicationName, testNamespace, false, "")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(customBundleRef))
		})

		It("should reference the default pipeline bundle in a PipelineRun", func() {
			defaultBundleConfigMap, err = f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace)
			if err != nil {
				if errors.IsForbidden(err) {
					klog.Infof("don't have enough permissions to get a configmap with default pipeline in %s namespace\n", constants.BuildPipelinesConfigMapDefaultNamespace)
				} else {
					Fail(fmt.Sprintf("error occured when trying to get configmap %s in %s namespace: %v", constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace, err))
				}
			} else {
				defaultBundleRef = defaultBundleConfigMap.Data["default_build_bundle"]
			}

			if customBundleRef != "" {
				Skip("skipping - custom pipeline bundle bundle (that overrides the default one) is defined")
			}
			if defaultBundleRef == "" {
				Skip("skipping - default pipeline bundle cannot be fetched")
			}
			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentNames[0], applicationName, testNamespace, false, "")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(defaultBundleRef))
		})

		for i, gitUrl := range componentUrls {
			gitUrl := gitUrl

			It(fmt.Sprintf("should eventually finish successfully for component with source URL %s", gitUrl), Label(buildTemplatesTestLabel), func() {
				timeout := time.Second * 600
				interval := time.Second * 10
				Eventually(func() bool {
					pipelineRun, err := f.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, false, "")
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						klog.Infof("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

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

			It("should validate HACBS taskrun results", Label(buildTemplatesTestLabel), func() {
				// List Of Taskruns Expected to Get Taskrun Results
				gatherResult := []string{"conftest-clair", "sanity-inspect-image", "sanity-label-check"}
				/*
				  Workaround for including "sbom-json-check" to gatherResults slice.

				 "sbom-json-check" wouldn't work in e2e-tests repo's pre-kcp branch, because required
				  updates to build templates won't be added to pre-kcp branch of infra-deployments repo
				  (which pre-kcp branch of e2e-tests is using)
				  This workaround allows us to use the "sbom-json-check" only in case the test is triggered
				  from build-definitions repository's e2e-test, which always uses the latest version
				  of build templates
				*/
				// TODO: once we migrate "build" e2e tests to kcp, remove this condition
				// and add the 'sbom-json-check' taskrun to gatherResults slice
				s, _ := GinkgoConfiguration()
				if strings.Contains(s.LabelFilter, buildTemplatesTestLabel) {
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
					} else {
						result, err := build.FetchTaskRunResult(pipelineRun, gatherResult[i], "HACBS_TEST_OUTPUT")
						Expect(err).ShouldNot(HaveOccurred())
						ret := build.ValidateTaskRunResults(gatherResult[i], result)
						Expect(ret).Should(BeTrue())
					}
				}
			})

			When("the container image is created and pushed to container registry", Label("sbom", "slow"), func() {
				It("contains non-empty sbom files", func() {
					component, err := f.HasController.GetHasComponent(componentName, testNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					purl, cyclonedx, err := build.GetParsedSbomFilesContentFromImage(component.Spec.ContainerImage)
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
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, "", "", containerImageSource, outputContainerImage, "")
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

	Describe("Creating a configmap with 'dummy' custom pipeline bundle in the testing namespace", Ordered, func() {
		var timeout, interval time.Duration

		var componentName, applicationName, testNamespace string

		BeforeAll(func() {

			testNamespace := fmt.Sprintf("build-e2e-dummy-custom-pipeline-%s", util.GenerateRandomString(4))
			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))

			_, err = f.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)
			DeferCleanup(f.CommonController.DeleteNamespace, testNamespace)

			app, err := f.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: constants.BuildPipelinesConfigMapName},
				Data:       map[string]string{"default_build_bundle": dummyPipelineBundleRef},
			}
			_, err = f.CommonController.CreateConfigMap(cm, testNamespace)
			Expect(err).ToNot(HaveOccurred())

			componentName = "build-suite-test-bundle-overriding"
			outputContainerImage := fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace, testNamespace)

			timeout = time.Second * 360
			interval = time.Second * 1

		})

		// AfterAll(func() {
		// 	f.CommonController.DeleteNamespace(testNamespace)
		// })

		It("should be referenced in a PipelineRun", Label("build-bundle-overriding"), func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(dummyPipelineBundleRef))
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
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace, testNamespace)

		})

		It("should override the shared secret", func() {
			Eventually(func() bool {
				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false, "")
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
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
					klog.Infof("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)
					return condition.Reason == "Failed"
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to fail")
		})
	})

	Describe("Creating a component with a specific container image URL", Ordered, func() {

		var applicationName, componentName, testNamespace, outputContainerImage string

		BeforeAll(func() {

			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))
			testNamespace = fmt.Sprintf("build-e2e-specific-image-url-%s", util.GenerateRandomString(4))

			_, err = f.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)
			app, err := f.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

		})

		AfterAll(func() {
			Expect(f.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, 30*time.Second)).To(Succeed())
			Expect(f.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, 30*time.Second)).To(Succeed())
			Expect(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
			Expect(f.CommonController.DeleteNamespace(testNamespace)).To(Succeed())
		})

		JustBeforeEach(func() {
			componentName = fmt.Sprintf("build-suite-test-component-image-url-%s", util.GenerateRandomString(4))
		})
		It("should fail for ContainerImage field set to a protected repository (without an image tag)", func() {
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images-protected", utils.GetQuayIOOrganization())
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "")
			Expect(err).ToNot(BeNil())

		})
		It("should fail for ContainerImage field set to a protected repository followed by a random tag", func() {
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images-protected:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "")
			Expect(err).ToNot(BeNil())
		})
		It("should succeed for ContainerImage field set to a protected repository followed by a namespace prefix + dash + string", func() {
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images-protected:%s-%s", utils.GetQuayIOOrganization(), testNamespace, strings.Replace(uuid.New().String(), "-", "", -1))
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("should succeed for ContainerImage field set to a custom (unprotected) repository without a tag being specified", func() {
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images", utils.GetQuayIOOrganization())
			_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "")
			Expect(err).ShouldNot(HaveOccurred())
		})

	})
})
