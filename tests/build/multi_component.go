package build

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-github/v44/github"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/build-service/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/konflux-ci/e2e-tests/pkg/clients/git"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
)

var _ = framework.BuildSuiteDescribe("Build service E2E tests", Label("build-service"), func() {

	var f *framework.Framework
	AfterEach(framework.ReportFailure(&f))
	var err error
	defer GinkgoRecover()

	Describe("test pac with multiple components using same repository", Ordered, Label("github", "pac-build", "multi-component"), func() {
		var applicationName, testNamespace, multiComponentBaseBranchName, multiComponentPRBranchName, mergeResultSha string
		var pacBranchNames []string
		var prNumber int
		var mergeResult *github.PullRequestMergeResult
		var timeout time.Duration
		var buildPipelineAnnotation map[string]string

		BeforeAll(func() {
			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				Skip("Skipping this test due to configuration issue with Spray proxy")
			}
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			if utils.IsPrivateHostname(f.OpenshiftConsoleHost) {
				Skip("Using private cluster (not reachable from Github), skipping...")
			}

			applicationName = fmt.Sprintf("build-suite-positive-mc-%s", util.GenerateRandomString(4))
			_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			multiComponentBaseBranchName = fmt.Sprintf("multi-component-base-%s", util.GenerateRandomString(6))
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(multiComponentGitSourceRepoName, multiComponentDefaultBranch, multiComponentGitRevision, multiComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			//Branch for creating pull request
			multiComponentPRBranchName = fmt.Sprintf("%s-%s", "pr-branch", util.GenerateRandomString(6))

			// get the build pipeline bundle annotation
			buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*2)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*2)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
			}

			// Delete new branches created by PaC and a testing branch used as a component's base branch
			for _, pacBranchName := range pacBranchNames {
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentGitSourceRepoName, pacBranchName)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
				}
			}
			// Delete the created base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentGitSourceRepoName, multiComponentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}
			// Delete the created pr branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentGitSourceRepoName, multiComponentPRBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}
		})

		When("components are created in same namespace", func() {
			var component *appservice.Component

			for _, contextDir := range multiComponentContextDirs {
				contextDir := contextDir
				componentName := fmt.Sprintf("%s-%s", contextDir, util.GenerateRandomString(6))
				pacBranchName := constants.PaCPullRequestBranchPrefix + componentName
				pacBranchNames = append(pacBranchNames, pacBranchName)

				It(fmt.Sprintf("creates component with context directory %s", contextDir), func() {
					componentObj := appservice.ComponentSpec{
						ComponentName: componentName,
						Application:   applicationName,
						Source: appservice.ComponentSource{
							ComponentSourceUnion: appservice.ComponentSourceUnion{
								GitSource: &appservice.GitSource{
									URL:           multiComponentGitHubURL,
									Revision:      multiComponentBaseBranchName,
									Context:       contextDir,
									DockerfileURL: constants.DockerFilePath,
								},
							},
						},
					}
					component, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
					Expect(err).ShouldNot(HaveOccurred())
				})

				It(fmt.Sprintf("triggers a PipelineRun for component %s", componentName), func() {
					timeout = time.Minute * 5
					Eventually(func() error {
						pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
						if err != nil {
							GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
							return err
						}
						if !pr.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", componentName, testNamespace))
				})

				It(fmt.Sprintf("should lead to a PaC PR creation for component %s", componentName), func() {
					timeout = time.Second * 300
					interval := time.Second * 5

				Eventually(func() bool {
					gitClient := git.NewGitHubClient(f.AsKubeAdmin.CommonController.Github)
					prs, err := git.ListPullRequestsWithRetry(gitClient, multiComponentGitSourceRepoName)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.SourceBranch == pacBranchName {
							prNumber = pr.Number
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for PaC PR (branch name '%s') to be created in %s repository", pacBranchName, multiComponentGitSourceRepoName))
				})

				It(fmt.Sprintf("the PipelineRun should eventually finish successfully for component %s", componentName), func() {
					Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", "", "",
						f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, nil)).To(Succeed())
				})

				It("merging the PR should be successful", func() {
					Eventually(func() error {
						mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(multiComponentGitSourceRepoName, prNumber)
						return err
					}, time.Minute).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, multiComponentGitSourceRepoName))

					mergeResultSha = mergeResult.GetSHA()
					GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)

				})
				It("leads to triggering on push PipelineRun", func() {
					timeout = time.Minute * 5

					Eventually(func() error {
						pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, mergeResultSha)
						if err != nil {
							GinkgoWriter.Printf("Push PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
							return err
						}
						if !pipelineRun.HasStarted() {
							return fmt.Errorf("push pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, componentName))
				})
			}
			It("only one component is changed", func() {
				//Delete all the pipelineruns in the namespace before sending PR
				Eventually(func() error {
					return f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				//Create the ref, add the file and create the PR
				err = f.AsKubeAdmin.CommonController.Github.CreateRef(multiComponentGitSourceRepoName, multiComponentDefaultBranch, mergeResultSha, multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred())
				fileToCreatePath := fmt.Sprintf("%s/sample-file.txt", multiComponentContextDirs[0])
				createdFileSha, err := f.AsKubeAdmin.CommonController.Github.CreateFile(multiComponentGitSourceRepoName, fileToCreatePath, fmt.Sprintf("sample test file inside %s", multiComponentContextDirs[0]), multiComponentPRBranchName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while creating file: %s", fileToCreatePath))
				pr, err := f.AsKubeAdmin.CommonController.Github.CreatePullRequest(multiComponentGitSourceRepoName, "sample pr title", "sample pr body", multiComponentPRBranchName, multiComponentBaseBranchName)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("PR #%d got created with sha %s\n", pr.GetNumber(), createdFileSha.GetSHA())
			})
			It("only related pipelinerun should be triggered", func() {
				Eventually(func() error {
					pipelineRuns, err := f.AsKubeAdmin.HasController.GetAllPipelineRunsForApplication(applicationName, testNamespace)
					if err != nil {
						GinkgoWriter.Println("on pull PiplelineRun has not been created yet for the PR")
						return err
					}
					if len(pipelineRuns.Items) != 1 || !strings.HasPrefix(pipelineRuns.Items[0].Name, multiComponentContextDirs[0]) {
						return fmt.Errorf("pipelinerun created in the namespace %s is not as expected, got pipelineruns %v", testNamespace, pipelineRuns.Items)
					}
					return nil
				}, time.Minute*5, constants.PipelineRunPollingInterval).Should(Succeed(), "timeout while waiting for PR pipeline to start")
			})
		})
		When("a components is created with same git url in different namespace", func() {
			var namespace, appName, compName string
			var fw *framework.Framework

			BeforeAll(func() {
				fw, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
				Expect(err).NotTo(HaveOccurred())
				namespace = fw.UserNamespace

				appName = fmt.Sprintf("build-suite-negative-mc-%s", util.GenerateRandomString(4))
				_, err = f.AsKubeAdmin.HasController.CreateApplication(appName, namespace)
				Expect(err).NotTo(HaveOccurred())

				compName = fmt.Sprintf("%s-%s", multiComponentContextDirs[0], util.GenerateRandomString(6))

				componentObj := appservice.ComponentSpec{
					ComponentName: compName,
					Application:   appName,
					Source: appservice.ComponentSource{
						ComponentSourceUnion: appservice.ComponentSourceUnion{
							GitSource: &appservice.GitSource{
								URL:           multiComponentGitHubURL,
								Revision:      multiComponentBaseBranchName,
								Context:       multiComponentContextDirs[0],
								DockerfileURL: constants.DockerFilePath,
							},
						},
					},
				}
				_, err = fw.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, namespace, "", "", appName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
				Expect(err).ShouldNot(HaveOccurred())

			})

			AfterAll(func() {
				if !CurrentSpecReport().Failed() {
					Eventually(func() error {
						return fw.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(namespace, time.Minute*2)
					}, 2*time.Minute, 10*time.Second).Should(Succeed())
					Eventually(func() error {
						return fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(namespace, time.Minute*2)
					}, 2*time.Minute, 10*time.Second).Should(Succeed())
				}
			})

			It("should fail to configure PaC for the component", func() {
				var buildStatus *controllers.BuildStatus

				Eventually(func() (bool, error) {
					component, err := fw.AsKubeAdmin.HasController.GetComponent(compName, namespace)
					if err != nil {
						GinkgoWriter.Printf("error while getting the component: %v\n", err)
						return false, err
					}

					buildStatusAnnotationValue := component.Annotations[controllers.BuildStatusAnnotationName]
					GinkgoWriter.Printf(buildStatusAnnotationValueLoggingFormat, buildStatusAnnotationValue)
					statusBytes := []byte(buildStatusAnnotationValue)

					err = json.Unmarshal(statusBytes, &buildStatus)
					if err != nil {
						GinkgoWriter.Printf("cannot unmarshal build status from component annotation: %v\n", err)
						return false, err
					}

					GinkgoWriter.Printf("build status: %+v\n", buildStatus.PaC)

					return buildStatus.PaC != nil && buildStatus.PaC.State == "error" && strings.Contains(buildStatus.PaC.ErrMessage, "Git repository is already handled by Pipelines as Code"), nil
				}, time.Minute*2, time.Second*2).Should(BeTrue(), "build status is unexpected")

			})

		})

	})
})

