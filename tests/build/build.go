package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/utils/pointer"

	"github.com/google/go-github/v44/github"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"

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

var _ = framework.BuildSuiteDescribe("Build service E2E tests", Label("build", "HACBS"), func() {
	var f *framework.Framework
	var err error

	defer GinkgoRecover()

	Describe("test PaC component build", Ordered, Label("github-webhook", "pac-build", "pipeline"), func() {
		var applicationName, componentName, componentBaseBranchName, pacBranchName, testNamespace, outputContainerImage, pacControllerHost, defaultBranchTestComponentName string

		var timeout, interval time.Duration

		var prNumber int
		var prCreationTime time.Time

		BeforeAll(func() {
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			consoleRoute, err := f.AsKubeAdmin.CommonController.GetOpenshiftRoute("console", "openshift-console")
			Expect(err).ShouldNot(HaveOccurred())

			if utils.IsPrivateHostname(consoleRoute.Spec.Host) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			// Used for identifying related webhook on GitHub - in order to delete it
			pacControllerRoute, err := f.AsKubeAdmin.CommonController.GetOpenshiftRoute("pipelines-as-code-controller", "pipelines-as-code")
			Expect(err).ShouldNot(HaveOccurred())
			pacControllerHost = pacControllerRoute.Spec.Host

			applicationName = fmt.Sprintf("build-suite-test-application-%s", util.GenerateRandomString(4))
			app, err := f.AsKubeAdmin.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			componentName = fmt.Sprintf("%s-%s", "test-component-pac", util.GenerateRandomString(4))
			pacBranchName = pacPRBranchPrefix + componentName
			componentBaseBranchName = fmt.Sprintf("base-%s", util.GenerateRandomString(4))
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images", utils.GetQuayIOOrganization())

			err = f.AsKubeAdmin.CommonController.Github.CreateRef(helloWorldComponentGitSourceRepoName, helloWorldComponentDefaultBranch, componentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			defaultBranchTestComponentName = fmt.Sprintf("test-custom-default-branch-%s", util.GenerateRandomString(4))
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}

			// Delete new branches created by PaC and a testing branch used as a component's base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(helloWorldComponentGitSourceRepoName, pacBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(helloWorldComponentGitSourceRepoName, componentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(helloWorldComponentGitSourceRepoName, pacPRBranchPrefix+defaultBranchTestComponentName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}

			// Delete created webhook from GitHub
			hooks, err := f.AsKubeAdmin.CommonController.Github.ListRepoWebhooks(helloWorldComponentGitSourceRepoName)
			Expect(err).NotTo(HaveOccurred())

			for _, h := range hooks {
				hookUrl := h.Config["url"].(string)
				if strings.Contains(hookUrl, pacControllerHost) {
					Expect(f.AsKubeAdmin.CommonController.Github.DeleteWebhook(helloWorldComponentGitSourceRepoName, h.GetID())).To(Succeed())
					break
				}
			}
		})

		When("a new component without specified branch is created", Label("pac-custom-default-branch"), func() {
			BeforeAll(func() {
				_, err = f.AsKubeDeveloper.HasController.CreateComponentWithPaCEnabled(applicationName, defaultBranchTestComponentName, testNamespace, helloWorldComponentGitSourceURL, "", "")
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("correctly targets the default branch (that is not named 'main') with PaC", func() {
				timeout = time.Second * 300
				interval = time.Second * 1
				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(helloWorldComponentGitSourceRepoName)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.Head.GetRef() == pacPRBranchPrefix+defaultBranchTestComponentName {
							Expect(pr.GetBase().GetRef()).To(Equal(helloWorldComponentDefaultBranch))
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for init PaC PR to be created")
			})
			It("triggers a PipelineRun", func() {
				timeout = time.Second * 600
				interval = time.Second * 1
				Eventually(func() bool {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(defaultBranchTestComponentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			})
			It("a related PipelineRun and Github webhook should be deleted after deleting the component", func() {
				timeout = time.Second * 60
				interval = time.Second * 1
				Expect(f.AsKubeAdmin.HasController.DeleteHasComponent(defaultBranchTestComponentName, testNamespace, true)).To(Succeed())
				// Test removal of PipelineRun
				Eventually(func() bool {
					_, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(defaultBranchTestComponentName, applicationName, testNamespace, "")
					return err != nil && strings.Contains(err.Error(), "no pipelinerun found")
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
				// Test removal of related webhook in GitHub repo
				Eventually(func() bool {
					hooks, err := f.AsKubeAdmin.CommonController.Github.ListRepoWebhooks(helloWorldComponentGitSourceRepoName)
					Expect(err).NotTo(HaveOccurred())

					for _, h := range hooks {
						hookUrl := h.Config["url"].(string)
						if strings.Contains(hookUrl, pacControllerHost) {
							return false
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			})
		})

		When("a new component with specified custom branch branch is created", func() {
			BeforeAll(func() {
				// Create a component with Git Source URL and a specified git branch
				_, err = f.AsKubeAdmin.HasController.CreateComponentWithPaCEnabled(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, componentBaseBranchName, outputContainerImage)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("triggers a PipelineRun", func() {
				timeout = time.Second * 600
				interval = time.Second * 1
				Eventually(func() bool {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			})
			It("should lead to a PaC init PR creation", func() {
				timeout = time.Second * 300
				interval = time.Second * 1

				Eventually(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(helloWorldComponentGitSourceRepoName)
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
				timeout = time.Minute * 30
				interval = time.Second * 10
				Eventually(func() bool {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if !pipelineRun.IsDone() {
							return false
						}

						if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
							failMessage := tekton.GetFailedPipelineRunLogs(f.AsKubeAdmin.CommonController, pipelineRun)
							Fail(failMessage)
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})
			It("eventually leads to a creation of a PR comment with the PipelineRun status report", func() {
				var comments []*github.IssueComment
				timeout = time.Minute * 15
				interval = time.Second * 10

				Eventually(func() bool {
					comments, err = f.AsKubeAdmin.CommonController.Github.ListPullRequestCommentsSince(helloWorldComponentGitSourceRepoName, prNumber, prCreationTime)
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
				createdFile, err := f.AsKubeAdmin.CommonController.Github.CreateFile(helloWorldComponentGitSourceRepoName, fileToCreatePath, fmt.Sprintf("test PaC branch %s update", pacBranchName), pacBranchName)
				Expect(err).NotTo(HaveOccurred())

				createdFileSHA = createdFile.GetSHA()
				GinkgoWriter.Println("created file sha:", createdFileSHA)
			})

			It("eventually leads to triggering another PipelineRun", func() {
				timeout = time.Minute * 7
				interval = time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, createdFileSHA)
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			})
			It("PipelineRun should eventually finish", func() {
				timeout = time.Minute * 50
				interval = time.Second * 10

				Eventually(func() bool {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, createdFileSHA)
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if !pipelineRun.IsDone() {
							return false
						}

						if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
							failMessage := tekton.GetFailedPipelineRunLogs(f.AsKubeAdmin.CommonController, pipelineRun)
							Fail(failMessage)
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})
			It("eventually leads to another update of a PR with a comment about the PipelineRun status report", func() {
				var comments []*github.IssueComment

				timeout = time.Minute * 20
				interval = time.Second * 5

				Eventually(func() bool {
					comments, err = f.AsKubeAdmin.CommonController.Github.ListPullRequestCommentsSince(helloWorldComponentGitSourceRepoName, prNumber, branchUpdateTimestamp)
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
					mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(helloWorldComponentGitSourceRepoName, prNumber)
					return err
				}, time.Minute).Should(BeNil(), fmt.Sprintf("error when merging PaC pull request: %+v", err))

				mergeResultSha = mergeResult.GetSHA()
				GinkgoWriter.Println("merged result sha:", mergeResultSha)
			})

			It("eventually leads to triggering another PipelineRun", func() {
				timeout = time.Minute * 10
				interval = time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, mergeResultSha)
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
			})

			It("pipelineRun should eventually finish", func() {
				timeout = time.Minute * 50
				interval = time.Second * 10

				Eventually(func() bool {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, mergeResultSha)
					Expect(err).ShouldNot(HaveOccurred())

					for _, condition := range pipelineRun.Status.Conditions {
						GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

						if !pipelineRun.IsDone() {
							return false
						}

						if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
							failMessage := tekton.GetFailedPipelineRunLogs(f.AsKubeAdmin.CommonController, pipelineRun)
							Fail(failMessage)
						}
					}
					return true
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})
		})

		When("the component is removed and recreated (with the same name in the same namespace)", func() {
			BeforeAll(func() {
				Expect(f.AsKubeAdmin.HasController.DeleteHasComponent(componentName, testNamespace, false)).To(Succeed())

				Eventually(func() bool {
					_, err := f.AsKubeAdmin.HasController.GetHasComponent(componentName, testNamespace)
					return errors.IsNotFound(err)
				}, time.Minute*1, time.Second*1).Should(BeTrue(), "timed out when waiting for the app %s to be deleted in %s namespace", applicationName, testNamespace)

				_, err = f.AsKubeAdmin.HasController.CreateComponentWithPaCEnabled(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, componentBaseBranchName, outputContainerImage)
			})

			It("should no longer lead to a creation of a PaC PR", func() {
				timeout = time.Second * 40
				interval = time.Second * 2
				Consistently(func() bool {
					prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(helloWorldComponentGitSourceRepoName)
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

	Describe("Creating component with container image source", Ordered, func() {

		var applicationName, componentName, testNamespace string
		var timeout, interval time.Duration

		BeforeAll(func() {
			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			app, err := f.AsKubeAdmin.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			componentName = fmt.Sprintf("build-suite-test-component-image-source-%s", util.GenerateRandomString(4))
			outputContainerImage := ""
			timeout = time.Second * 500
			interval = time.Second * 1
			// Create a component with containerImageSource being defined
			_, err = f.AsKubeAdmin.HasController.CreateComponent(applicationName, componentName, testNamespace, "", "", containerImageSource, outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.HasController.DeleteHasComponent(componentName, testNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}
		})

		It("should not trigger a PipelineRun", func() {
			Consistently(func() bool {
				_, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
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
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace
			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))

			app, err := f.AsKubeAdmin.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
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

			Expect(f.AsKubeAdmin.CommonController.KubeRest().Create(context.TODO(), ps)).To(Succeed())

			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

			timeout = time.Second * 600
			interval = time.Second * 1

		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.HasController.DeleteHasComponent(componentName, testNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}
		})

		It("a specific Pipeline bundle should be used and additional pipeline params should be added to the PipelineRun if all WhenConditions match", func() {
			_, err = f.AsKubeAdmin.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(dummyPipelineBundleRef))
			Expect(pipelineRun.Spec.Params).To(ContainElement(v1beta1.Param{
				Name:  expectedAdditionalPipelineParam.Name,
				Value: v1beta1.ParamValue{StringVal: expectedAdditionalPipelineParam.Value, Type: "string"}},
			))
		})

		It("default Pipeline bundle should be used and no additional Pipeline params should be added to the PipelineRun if one of the WhenConditions does not match", func() {
			notMatchingComponentName := componentName + util.GenerateRandomString(4)
			_, err = f.AsKubeAdmin.HasController.CreateComponent(applicationName, notMatchingComponentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func() bool {
				pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(notMatchingComponentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(notMatchingComponentName, applicationName, testNamespace, "")
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
		var err error
		var kc tekton.KubeController
		var pipelineRun *v1beta1.PipelineRun

		BeforeAll(func() {

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			kc = tekton.KubeController{
				Commonctrl: *f.AsKubeAdmin.CommonController,
				Tektonctrl: *f.AsKubeAdmin.TektonController,
				Namespace:  testNamespace,
			}

			_, err := f.AsKubeAdmin.CommonController.GetSecret(testNamespace, constants.RegistryAuthSecretName)
			if err != nil {
				// If we have an error when getting RegistryAuthSecretName, it should be IsNotFound err
				Expect(errors.IsNotFound(err)).To(BeTrue())
			} else {
				Skip("a registry auth secret is already created in testing namespace - skipping....")
			}

			applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))

			app, err := f.AsKubeAdmin.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)
			timeout = time.Minute * 20
			interval = time.Second * 1

			dummySecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: constants.RegistryAuthSecretName},
				Type:       v1.SecretTypeDockerConfigJson,
				Data:       map[string][]byte{".dockerconfigjson": []byte("{\"auths\":{\"quay.io\":{\"username\":\"test\",\"password\":\"test\",\"auth\":\"dGVzdDp0ZXN0\",\"email\":\"\"}}}")},
			}

			_, err = f.AsKubeAdmin.CommonController.CreateSecret(testNamespace, dummySecret)
			Expect(err).ToNot(HaveOccurred())
			err = f.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(testNamespace, dummySecret.Name, "pipeline")
			Expect(err).ToNot(HaveOccurred())

			componentName = "build-suite-test-secret-overriding"
			outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
			_, err = f.AsKubeAdmin.HasController.CreateComponent(applicationName, componentName, testNamespace, helloWorldComponentGitSourceURL, "", "", outputContainerImage, "", true)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.HasController.DeleteHasComponent(componentName, testNamespace, false)).To(Succeed())
				Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}
		})

		It("should override the shared secret", func() {
			Eventually(func() bool {
				pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")

			pipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(pipelineRun.Spec.Workspaces).To(HaveLen(1))
		})

		It("should not be possible to push to quay.io repo (PipelineRun should fail)", func() {
			pipelineRunTimeout := int(time.Duration(20) * time.Minute)

			Expect(kc.WatchPipelineRun(pipelineRun.Name, pipelineRunTimeout)).To(Succeed())
			pipelineRun, err = kc.Tektonctrl.GetPipelineRun(pipelineRun.Name, pipelineRun.Namespace)
			Expect(err).NotTo(HaveOccurred())
			tr, err := kc.GetTaskRunStatus(pipelineRun, constants.BuildTaskRunName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tekton.DidTaskSucceed(tr)).To(BeFalse())
		})
	})
})
