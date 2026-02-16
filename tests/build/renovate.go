package build

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"

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

	DescribeTableSubtree("test of component update with renovate", Ordered, Label("renovate", "multi-component"), func(gitProvider git.GitProvider, gitPrefix string) {
		type multiComponent struct {
			repoName        string
			baseBranch      string
			componentBranch string
			baseRevision    string
			componentName   string
			gitRepo         string
			pacBranchName   string
			component       *appservice.Component
		}

		nameSuffix := util.GenerateRandomString(6)
		targetChildRepoName := componentDependenciesChildRepoName + "-" + nameSuffix
		targetParentRepoName := componentDependenciesParentRepoName + "-" + nameSuffix
		ChildComponentDef := multiComponent{repoName: targetChildRepoName, baseRevision: componentDependenciesChildGitRevision, baseBranch: componentDependenciesChildDefaultBranch}
		ParentComponentDef := multiComponent{repoName: targetParentRepoName, baseRevision: componentDependenciesParentGitRevision, baseBranch: componentDependenciesParentDefaultBranch}
		components := []*multiComponent{&ChildComponentDef, &ParentComponentDef}
		var applicationName, testNamespace, mergeResultSha, imageRepoName string
		var prNumber int
		var mergeResult *git.PullRequest
		var timeout time.Duration
		var parentFirstDigest string
		var parentPLR *pipeline.PipelineRun
		var parentPostPacMergeDigest string
		var parentImageNameWithNoDigest string
		const distributionRepository = "quay.io/redhat-appstudio-qe/release-repository"
		quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "")
		var parentRepository, childRepository string

		var managedNamespace string
		var buildPipelineAnnotation map[string]string

		var gitClient git.Client
		var componentDependenciesChildRepository string

		BeforeAll(func() {
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			applicationName = fmt.Sprintf("build-suite-component-update-%s", util.GenerateRandomString(4))
			_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			branchString := util.GenerateRandomString(4)
			ParentComponentDef.componentBranch = fmt.Sprintf("multi-component-parent-base-%s", branchString)
			ChildComponentDef.componentBranch = fmt.Sprintf("multi-component-child-base-%s", branchString)
			switch gitProvider {
			case git.GitHubProvider:
				gitClient = git.NewGitHubClient(f.AsKubeAdmin.CommonController.Github)

				ParentComponentDef.gitRepo = fmt.Sprintf(githubUrlFormat, githubOrg, ParentComponentDef.repoName)
				parentRepository = ParentComponentDef.repoName

				ChildComponentDef.gitRepo = fmt.Sprintf(githubUrlFormat, githubOrg, ChildComponentDef.repoName)
				childRepository = ChildComponentDef.repoName

				componentDependenciesChildRepository = ChildComponentDef.repoName

				// Fork the parent repo
				err = gitClient.ForkRepository(componentDependenciesParentRepoName, ParentComponentDef.repoName)
				Expect(err).ShouldNot(HaveOccurred())
				// Fork the child repo
				err = gitClient.ForkRepository(componentDependenciesChildRepoName, ChildComponentDef.repoName)
				Expect(err).ShouldNot(HaveOccurred())

			case git.GitLabProvider:
				gitClient = git.NewGitlabClient(f.AsKubeAdmin.CommonController.Gitlab)

				parentRepository = fmt.Sprintf("%s/%s", gitlabOrg, ParentComponentDef.repoName)
				ParentComponentDef.gitRepo = fmt.Sprintf(gitlabUrlFormat, parentRepository)

				childRepository = fmt.Sprintf("%s/%s", gitlabOrg, ChildComponentDef.repoName)
				ChildComponentDef.gitRepo = fmt.Sprintf(gitlabUrlFormat, childRepository)

				// Fork the parent repo
				err = gitClient.ForkRepository(fmt.Sprintf("%s/%s", gitlabOrg, componentDependenciesParentRepoName), parentRepository)
				Expect(err).ShouldNot(HaveOccurred())
				// Fork the child repo
				err = gitClient.ForkRepository(fmt.Sprintf("%s/%s", gitlabOrg, componentDependenciesChildRepoName), childRepository)
				Expect(err).ShouldNot(HaveOccurred())

				componentDependenciesChildRepository = childRepository
			}
			ParentComponentDef.componentName = fmt.Sprintf("%s-multi-component-parent-%s", gitPrefix, branchString)
			ChildComponentDef.componentName = fmt.Sprintf("%s-multi-component-child-%s", gitPrefix, branchString)
			ParentComponentDef.pacBranchName = constants.PaCPullRequestBranchPrefix + ParentComponentDef.componentName
			ChildComponentDef.pacBranchName = constants.PaCPullRequestBranchPrefix + ChildComponentDef.componentName

			err = gitClient.CreateBranch(parentRepository, ParentComponentDef.baseBranch, ParentComponentDef.baseRevision, ParentComponentDef.componentBranch)
			Expect(err).ShouldNot(HaveOccurred())

			err = gitClient.CreateBranch(childRepository, ChildComponentDef.baseBranch, ChildComponentDef.baseRevision, ChildComponentDef.componentBranch)
			Expect(err).ShouldNot(HaveOccurred())

			// Also setup a release namespace so we can test nudging of distribution repository images
			managedNamespace = testNamespace + "-managed"
			_, err = f.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
			Expect(err).ShouldNot(HaveOccurred())

			// We just need the ReleaseAdmissionPlan to contain a mapping between component and distribution repositories
			data := struct {
				Mapping struct {
					Components []struct {
						Name       string
						Repository string
					}
				}
			}{}
			data.Mapping.Components = append(data.Mapping.Components, struct {
				Name       string
				Repository string
			}{Name: ParentComponentDef.componentName, Repository: distributionRepository})
			rawData, err := json.Marshal(&data)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Printf("ReleaseAdmissionPlan data: %s", string(rawData))
			managedServiceAccount, err := f.AsKubeAdmin.CommonController.CreateServiceAccount("release-service-account", managedNamespace, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
			Expect(err).NotTo(HaveOccurred())

			_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission("demo", managedNamespace, "", f.UserNamespace, "demo", "release-service-account", []string{applicationName}, true, &tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: constants.RELEASE_CATALOG_DEFAULT_URL},
					{Name: "revision", Value: constants.RELEASE_CATALOG_DEFAULT_REVISION},
					{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
				}}, &runtime.RawExtension{Raw: rawData})
			Expect(err).NotTo(HaveOccurred())

			// get the build pipeline bundle annotation
			buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

			if gitProvider == git.GitLabProvider {
				gitlabToken := utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, "")
				Expect(gitlabToken).ShouldNot(BeEmpty())

				secretAnnotations := map[string]string{}

				err = build.CreateGitlabBuildSecret(f, "pipelines-as-code-secret", secretAnnotations, gitlabToken)
				Expect(err).ShouldNot(HaveOccurred())
			}
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteComponent(ParentComponentDef.componentName, testNamespace, true)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteComponent(ChildComponentDef.componentName, testNamespace, true)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteApplication(applicationName, testNamespace, false)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				Expect(gitClient.DeleteRepositoryIfExists(parentRepository)).To(Succeed())
				Expect(gitClient.DeleteRepositoryIfExists(childRepository)).To(Succeed())
			}
			Eventually(func() error {
				return f.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})

		When("components are created in same namespace", func() {

			It("creates component with nudges", func() {
				for _, comp := range components {
					componentObj := appservice.ComponentSpec{
						ComponentName: comp.componentName,
						Application:   applicationName,
						Source: appservice.ComponentSource{
							ComponentSourceUnion: appservice.ComponentSourceUnion{
								GitSource: &appservice.GitSource{
									URL:           comp.gitRepo,
									Revision:      comp.componentBranch,
									DockerfileURL: "Dockerfile",
								},
							},
						},
					}
					//make the parent repo nudge the child repo
					if comp.repoName == targetParentRepoName {
						componentObj.BuildNudgesRef = []string{ChildComponentDef.componentName}
					}
					comp.component, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, true, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
					Expect(err).ShouldNot(HaveOccurred())
				}
			})
			// Initial pipeline run, we need this so we have an initial image that we can then update
			It(fmt.Sprintf("triggers a PipelineRun for parent component %s", ParentComponentDef.componentName), func() {
				timeout = time.Minute * 5

				Eventually(func() error {
					pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(ParentComponentDef.componentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, ParentComponentDef.componentName)
						return err
					}
					if !pr.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", ParentComponentDef.componentName, testNamespace))
			})
		It(fmt.Sprintf("the PipelineRun should eventually finish successfully for parent component %s", ParentComponentDef.componentName), func() {
			parentPLR = &pipeline.PipelineRun{}
			Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(ParentComponentDef.component, "", "", "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Always: true, Retries: 2}, parentPLR)).To(Succeed())
		for _, result := range parentPLR.Status.Results {
			if result.Name == "IMAGE_DIGEST" {
				parentFirstDigest = result.Value.StringVal
			}
		}
		Expect(parentFirstDigest).ShouldNot(BeEmpty(), fmt.Sprintf("pipelinerun %s status results: %v", parentPLR.Name, parentPLR.Status.Results))
		})

			It(fmt.Sprintf("the PipelineRun should eventually finish successfully for child component %s", ChildComponentDef.componentName), func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(ChildComponentDef.component, "", "", "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Always: true, Retries: 2}, nil)).To(Succeed())
			})

			It(fmt.Sprintf("should lead to a PaC PR creation for child component %s", ChildComponentDef.componentName), func() {
				timeout = time.Second * 300
				interval := time.Second * 5

			Eventually(func() bool {
				prs, err := git.ListPullRequestsWithRetry(gitClient, childRepository)
				Expect(err).ShouldNot(HaveOccurred())

				for _, pr := range prs {
					if pr.SourceBranch == ChildComponentDef.pacBranchName {
						prNumber = pr.Number
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for PaC PR (branch name '%s') to be created in %s repository", ChildComponentDef.pacBranchName, ChildComponentDef.repoName))
			})

		It(fmt.Sprintf("Merging the PaC PR should be successful for child component %s", ChildComponentDef.componentName), func() {
			Eventually(func() error {
				mergeResult, err = gitClient.MergePullRequest(childRepository, prNumber)
				return err
			}, time.Minute).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, ChildComponentDef.repoName))

			mergeResultSha = mergeResult.MergeCommitSHA
			GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)
		})
			// Now we have an initial image we create a dockerfile in the child that references this new image
			// This is the file that will be updated by the nudge
			It("create dockerfile and yaml manifest that references build and distribution repositories", func() {

				imageRepoName, err = f.AsKubeAdmin.ImageController.GetImageName(testNamespace, ParentComponentDef.componentName)
				Expect(err).ShouldNot(HaveOccurred(), "failed to read image repo for component %s", ParentComponentDef.componentName)
				Expect(imageRepoName).ShouldNot(BeEmpty(), "image repo name is empty")

				parentImageNameWithNoDigest = "quay.io/" + quayOrg + "/" + imageRepoName
				_, err = gitClient.CreateFile(childRepository, "Dockerfile.tmp", "FROM "+parentImageNameWithNoDigest+"@"+parentFirstDigest+"\nRUN echo hello\n", ChildComponentDef.pacBranchName)
				Expect(err).ShouldNot(HaveOccurred())

				_, err = gitClient.CreateFile(childRepository, "manifest.yaml", "image: "+distributionRepository+"@"+parentFirstDigest, ChildComponentDef.pacBranchName)
				Expect(err).ShouldNot(HaveOccurred())

				_, err = gitClient.CreatePullRequest(childRepository, "updated to build repo image", "update to build repo image", ChildComponentDef.pacBranchName, ChildComponentDef.componentBranch)
				Expect(err).ShouldNot(HaveOccurred())

				prs, err := git.ListPullRequestsWithRetry(gitClient, childRepository)
				Expect(err).ShouldNot(HaveOccurred())

				prno := -1
				for _, pr := range prs {
					if pr.SourceBranch == ChildComponentDef.pacBranchName {
						prno = pr.Number
					}
				}
				Expect(prno).ShouldNot(Equal(-1))

			// GitLab merge fails if the pipeline run has not finished
			Eventually(func() error {
				_, err = gitClient.MergePullRequest(childRepository, prno)
				return err
			}, 10*time.Minute, time.Minute).ShouldNot(HaveOccurred(), fmt.Sprintf("unable to merge PR #%d in %s", prno, ChildComponentDef.repoName))

			})
			// This actually happens immediately, but we only need the PR number now
			It(fmt.Sprintf("should lead to a PaC PR creation for parent component %s", ParentComponentDef.componentName), func() {
				timeout = time.Second * 300
				interval := time.Second * 5

			Eventually(func() bool {
				prs, err := git.ListPullRequestsWithRetry(gitClient, parentRepository)
				Expect(err).ShouldNot(HaveOccurred())

				for _, pr := range prs {
					if pr.SourceBranch == ParentComponentDef.pacBranchName {
						prNumber = pr.Number
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for PaC PR (branch name '%s') to be created in %s repository", ParentComponentDef.pacBranchName, ParentComponentDef.repoName))
			})
		It(fmt.Sprintf("Merging the PaC PR should be successful for parent component %s", ParentComponentDef.componentName), func() {
			Eventually(func() error {
				mergeResult, err = gitClient.MergePullRequest(parentRepository, prNumber)
				return err
			}, time.Minute).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, ParentComponentDef.repoName))

			mergeResultSha = mergeResult.MergeCommitSHA
			GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)
		})
			// Now the PR is merged this will kick off another build. The result of this build is what we want to update in dockerfile we created
			It(fmt.Sprintf("PR merge triggers PAC PipelineRun for parent component %s", ParentComponentDef.componentName), func() {
				timeout = time.Minute * 5

				Eventually(func() error {
					pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(ParentComponentDef.componentName, applicationName, testNamespace, mergeResultSha)
					if err != nil {
						GinkgoWriter.Printf("Push PipelineRun has not been created yet for the component %s/%s\n", testNamespace, ParentComponentDef.componentName)
						return err
					}
					if !pipelineRun.HasStarted() {
						return fmt.Errorf("push pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, ParentComponentDef.componentName))
			})
			// Wait for this PR to be done and store the digest, we will need it to verify that the nudge was correct
			It(fmt.Sprintf("PAC PipelineRun for parent component %s is successful", ParentComponentDef.componentName), func() {
				pr := &pipeline.PipelineRun{}
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(ParentComponentDef.component, "", mergeResultSha, "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Always: true, Retries: 2}, pr)).To(Succeed())

				for _, result := range pr.Status.Results {
					if result.Name == "IMAGE_DIGEST" {
						parentPostPacMergeDigest = result.Value.StringVal
					}
				}
				Expect(parentPostPacMergeDigest).ShouldNot(BeEmpty())
			})
			It(fmt.Sprintf("should lead to a nudge PR creation for child component %s", ChildComponentDef.componentName), func() {
				timeout = time.Minute * 20
				interval := time.Second * 10

			Eventually(func() bool {
				prs, err := git.ListPullRequestsWithRetry(gitClient, componentDependenciesChildRepository)
				Expect(err).ShouldNot(HaveOccurred())

				for _, pr := range prs {
					if strings.Contains(pr.SourceBranch, ParentComponentDef.componentName) {
						prNumber = pr.Number
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for component nudge PR to be created in %s repository", targetChildRepoName))
			})
		It(fmt.Sprintf("merging the PR should be successful for child component %s", ChildComponentDef.componentName), func() {
			Eventually(func() error {
				mergeResult, err = gitClient.MergePullRequest(componentDependenciesChildRepository, prNumber)
				return err
			}, time.Minute).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging nudge pull request #%d in repo %s", prNumber, targetChildRepoName))

				mergeResultSha = mergeResult.MergeCommitSHA
				GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)

			})
			// Now the nudge has been merged we verify the dockerfile is what we expected
			It("Verify the nudge updated the contents", func() {

				GinkgoWriter.Printf("Verifying Dockerfile.tmp updated to sha %s", parentPostPacMergeDigest)
				file, err := gitClient.GetFile(childRepository, "Dockerfile.tmp", ChildComponentDef.componentBranch)
				Expect(err).ShouldNot(HaveOccurred())
				GinkgoWriter.Printf("content: %s\n", file.Content)
				Expect(file.Content).Should(Equal("FROM quay.io/" + quayOrg + "/" + imageRepoName + "@" + parentPostPacMergeDigest + "\nRUN echo hello\n"))

				file, err = gitClient.GetFile(childRepository, "manifest.yaml", ChildComponentDef.componentBranch)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(file.Content).Should(Equal("image: " + distributionRepository + "@" + parentPostPacMergeDigest))

			})
		})
	},
		Entry("github", Label("github"), git.GitHubProvider, "gh"),
		Entry("gitlab", Label("gitlab"), git.GitLabProvider, "gl"),
	)
})

