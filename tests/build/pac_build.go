package build

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/build-service/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/image/reference"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

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

	var gitClient git.Client

	DescribeTableSubtree("test PaC component build", Ordered, Label("github-webhook", "pac-build", "pipeline", "image-controller"), func(gitProvider git.GitProvider, gitPrefix string) {
		var applicationName, customDefaultComponentName, customBranchComponentName, componentBaseBranchName string
		var pacBranchName, testNamespace, imageRepoName, pullRobotAccountName, pushRobotAccountName string
		var helloWorldComponentGitSourceURL, customDefaultComponentBranch string
		var component *appservice.Component
		var plr *pipeline.PipelineRun

		var timeout, interval time.Duration

		var prNumber int
		var purgePrNumber int
		var prHeadSha string
		var buildPipelineAnnotation map[string]string

		var helloWorldRepository string

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

			quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "")
			supports, err := build.DoesQuayOrgSupportPrivateRepo()
			Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error while checking if quay org supports private repo: %+v", err))
			if !supports {
				if quayOrg == "redhat-appstudio-qe" {
					Fail("Failed to create private image repo in redhat-appstudio-qe org")
				} else {
					Skip("Quay org does not support private quay repository creation, please add support for private repo creation before running this test")
				}
			}
			Expect(err).ShouldNot(HaveOccurred())

			applicationName = fmt.Sprintf("build-suite-test-application-%s", util.GenerateRandomString(4))
			_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			customDefaultComponentName = fmt.Sprintf("%s-%s-%s", gitPrefix, "test-custom-default", util.GenerateRandomString(6))
			customBranchComponentName = fmt.Sprintf("%s-%s-%s", gitPrefix, "test-custom-branch", util.GenerateRandomString(6))
			pacBranchName = constants.PaCPullRequestBranchPrefix + customBranchComponentName
			customDefaultComponentBranch = constants.PaCPullRequestBranchPrefix + customDefaultComponentName
			componentBaseBranchName = fmt.Sprintf("base-%s", util.GenerateRandomString(6))

			gitClient, helloWorldComponentGitSourceURL, helloWorldRepository = setupGitProvider(f, gitProvider)
			// get the build pipeline bundle annotation
			buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

			err = gitClient.CreateBranch(helloWorldRepository, helloWorldComponentDefaultBranch, helloWorldComponentRevision, componentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*2)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*2)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				Expect(gitClient.DeleteRepositoryIfExists(helloWorldRepository)).To(Succeed())
			}

		})

		When("a new component without specified branch is created and with visibility private", Label("pac-custom-default-branch"), func() {
			var componentObj appservice.ComponentSpec

			BeforeAll(func() {
				componentObj = appservice.ComponentSpec{
					ComponentName: customDefaultComponentName,
					Application:   applicationName,
					Source: appservice.ComponentSource{
						ComponentSourceUnion: appservice.ComponentSourceUnion{
							GitSource: &appservice.GitSource{
								URL:           helloWorldComponentGitSourceURL,
								Revision:      "",
								DockerfileURL: constants.DockerFilePath,
							},
						},
					},
				}

				_, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPrivateRepo), buildPipelineAnnotation))
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("correctly targets the default branch (that is not named 'main') with PaC", func() {
				timeout = time.Second * 300
				interval = time.Second * 5
				Eventually(func() bool {
					prs, err := git.ListPullRequestsWithRetry(gitClient, helloWorldRepository)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.SourceBranch == customDefaultComponentBranch {
							Expect(pr.TargetBranch).To(Equal(helloWorldComponentDefaultBranch))
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR to be created against %s branch in %s repository", helloWorldComponentDefaultBranch, helloWorldRepository))
			})

			It("workspace parameter is set correctly in PaC repository CR", func() {
				nsObj, err := f.AsKubeAdmin.CommonController.GetNamespace(testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				wsName := nsObj.Labels["appstudio.redhat.com/workspace_name"]
				// Build-service creates the PaC Repository CR asynchronously; allow enough time for it to appear.
				Eventually(func() error {
					repositoryParams, getErr := f.AsKubeAdmin.TektonController.GetRepositoryParams(customDefaultComponentName, testNamespace)
					if getErr != nil {
						return getErr
					}
					for _, param := range repositoryParams {
						if param.Name == "appstudio_workspace" {
							if param.Value != wsName {
								return fmt.Errorf("got workspace param value: %s, expected %s", param.Value, wsName)
							}
							return nil
						}
					}
					return fmt.Errorf("appstudio_workspace param does not exist in repository CR")
				}, 5*time.Minute, 5*time.Second).Should(Succeed(), "timed out waiting for PaC repository CR %q in namespace %s with correct workspace parameter (build-service may create it asynchronously)", customDefaultComponentName, testNamespace)

			})

			It("triggers a PipelineRun", func() {
				timeout = time.Minute * 5
				Eventually(func() error {
					plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customDefaultComponentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, customBranchComponentName)
						return err
					}
					if !plr.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", customBranchComponentName, testNamespace))
			})

			It("build pipeline uses the correct serviceAccount", func() {
				serviceAccountName := "build-pipeline-" + customDefaultComponentName
				Expect(plr.Spec.TaskRunTemplate.ServiceAccountName).Should(Equal(serviceAccountName))
			})
			It("component build status is set correctly", func() {
				var buildStatus *controllers.BuildStatus
				Eventually(func() (bool, error) {
					component, err := f.AsKubeAdmin.HasController.GetComponent(customDefaultComponentName, testNamespace)
					if err != nil {
						return false, err
					}

					buildStatusAnnotationValue := component.Annotations[controllers.BuildStatusAnnotationName]
					GinkgoWriter.Printf(buildStatusAnnotationValueLoggingFormat, buildStatusAnnotationValue)
					statusBytes := []byte(buildStatusAnnotationValue)

					err = json.Unmarshal(statusBytes, &buildStatus)
					if err != nil {
						return false, err
					}

					if buildStatus.PaC != nil {
						GinkgoWriter.Printf("state: %s\n", buildStatus.PaC.State)
						GinkgoWriter.Printf("mergeUrl: %s\n", buildStatus.PaC.MergeUrl)
						GinkgoWriter.Printf("errId: %d\n", buildStatus.PaC.ErrId)
						GinkgoWriter.Printf("errMessage: %s\n", buildStatus.PaC.ErrMessage)
						GinkgoWriter.Printf("configurationTime: %s\n", buildStatus.PaC.ConfigurationTime)
					} else {
						GinkgoWriter.Println("build status does not have PaC field")
					}

					return buildStatus.PaC != nil && buildStatus.PaC.State == "enabled" && buildStatus.PaC.MergeUrl != "" && buildStatus.PaC.ErrId == 0 && buildStatus.PaC.ConfigurationTime != "", nil
				}, timeout, interval).Should(BeTrue(), "component build status has unexpected content")
			})
			It("image repo and robot account created successfully", func() {
				imageRepoName, err = f.AsKubeAdmin.ImageController.GetImageName(testNamespace, customDefaultComponentName)
				Expect(err).ShouldNot(HaveOccurred(), "failed to read image repo for component %s", customDefaultComponentName)
				Expect(imageRepoName).ShouldNot(BeEmpty(), "image repo name is empty")

				imageExist, err := build.DoesImageRepoExistInQuay(imageRepoName)
				Expect(err).ShouldNot(HaveOccurred(), "failed while checking if image repo exists in quay with error: %+v", err)
				Expect(imageExist).To(BeTrue(), "quay image does not exists")

				pullRobotAccountName, pushRobotAccountName, err = f.AsKubeAdmin.ImageController.GetRobotAccounts(testNamespace, customDefaultComponentName)
				Expect(err).ShouldNot(HaveOccurred(), "failed to get robot account names")
				pullRobotAccountExist, err := build.DoesRobotAccountExistInQuay(pullRobotAccountName)
				Expect(err).ShouldNot(HaveOccurred(), "failed while checking if pull robot account exists in quay with error: %+v", err)
				Expect(pullRobotAccountExist).To(BeTrue(), "pull robot account does not exists in quay")
				pushRobotAccountExist, err := build.DoesRobotAccountExistInQuay(pushRobotAccountName)
				Expect(err).ShouldNot(HaveOccurred(), "failed while checking if push robot account exists in quay with error: %+v", err)
				Expect(pushRobotAccountExist).To(BeTrue(), "push robot account does not exists in quay")
			})
			It("created image repo is private", func() {
				isPublic, err := build.IsImageRepoPublic(imageRepoName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed while checking if the image repo %s is private", imageRepoName))
				Expect(isPublic).To(BeFalse(), "Expected image repo to be private, but it is public")
			})

			It("a related PipelineRun should be deleted after deleting the component", func() {
				timeout = time.Second * 180
				interval = time.Second * 5
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteComponent(customDefaultComponentName, testNamespace, true)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				// Test removal of PipelineRun
				Eventually(func() error {
					plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customDefaultComponentName, applicationName, testNamespace, "")
					if err == nil {
						return fmt.Errorf("pipelinerun %s/%s is not removed yet", plr.GetNamespace(), plr.GetName())
					}
					return err
				}, timeout, interval).Should(MatchError(ContainSubstring("no pipelinerun found")), fmt.Sprintf("timed out when waiting for the PipelineRun to be removed for Component %s/%s", testNamespace, customBranchComponentName))
			})

			It("PR branch should not exist in the repo", func() {
				timeout = time.Second * 60
				interval = time.Second * 1
				Eventually(func() (bool, error) {
					exists, err := gitClient.BranchExists(helloWorldRepository, customDefaultComponentBranch)
					if err != nil {
						Expect(err.Error()).To(Or(ContainSubstring("Reference does not exist"), ContainSubstring("404")))
						return false, nil
					}
					return exists, nil
				}, timeout, interval).Should(BeFalse(), fmt.Sprintf("timed out when waiting for the branch %s to be deleted from %s repository", customDefaultComponentBranch, helloWorldRepository))
			})

			It("related image repo and the robot account should be deleted after deleting the component", func() {
				timeout = time.Second * 60
				interval = time.Second * 1
				// Check image repo should be deleted
				Eventually(func() (bool, error) {
					return build.DoesImageRepoExistInQuay(imageRepoName)
				}, timeout, interval).Should(BeFalse(), fmt.Sprintf("timed out when waiting for image repo %s to be deleted", imageRepoName))

				// Check robot account should be deleted
				Eventually(func() (bool, error) {
					pullRobotAccountExists, err := build.DoesRobotAccountExistInQuay(pullRobotAccountName)
					if err != nil {
						return false, err
					}
					pushRobotAccountExists, err := build.DoesRobotAccountExistInQuay(pushRobotAccountName)
					if err != nil {
						return false, err
					}
					return pullRobotAccountExists || pushRobotAccountExists, nil
				}, timeout, interval).Should(BeFalse(), fmt.Sprintf("timed out when checking if robot accounts %s and %s got deleted", pullRobotAccountName, pushRobotAccountName))

			})
		})

		When("a new Component with specified custom branch is created", Label("build-custom-branch"), func() {
			var outputImage string
			var componentObj appservice.ComponentSpec

			BeforeAll(func() {
				componentObj = appservice.ComponentSpec{
					ComponentName: customBranchComponentName,
					Application:   applicationName,
					Source: appservice.ComponentSource{
						ComponentSourceUnion: appservice.ComponentSourceUnion{
							GitSource: &appservice.GitSource{
								URL:           helloWorldComponentGitSourceURL,
								Revision:      componentBaseBranchName,
								DockerfileURL: constants.DockerFilePath,
							},
						},
					},
				}
				// Create a component with Git Source URL, a specified git branch and marking delete-repo=true
				component, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
				Expect(err).ShouldNot(HaveOccurred())
			})
			AfterAll(func() {
				// Close Pruge PR if exists
				err = gitClient.DeleteBranchAndClosePullRequest(helloWorldRepository, purgePrNumber)
				if err != nil {
					Expect(err.Error()).To(Or(ContainSubstring("Reference does not exist"), ContainSubstring("404")))
				}
			})

			It("triggers a PipelineRun", func() {
				timeout = time.Second * 600
				interval = time.Second * 1
				Eventually(func() error {
					plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, customBranchComponentName)
						return err
					}
					if !plr.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, customBranchComponentName))
			})
			It("should lead to a PaC init PR creation", func() {
				timeout = time.Second * 300
				interval = time.Second * 5

				Eventually(func() bool {
					prs, err := git.ListPullRequestsWithRetry(gitClient, helloWorldRepository)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.SourceBranch == pacBranchName {
							prNumber = pr.Number
							prHeadSha = pr.HeadSHA
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchName, helloWorldRepository))
			})
			It("the PipelineRun should eventually finish successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", "", "",
					f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(Succeed())
				// in case the first pipelineRun attempt has failed and was retried, we need to update the git branch head ref
				prHeadSha = plr.Labels["pipelinesascode.tekton.dev/sha"]
			})
			It("image repo and robot account created successfully", func() {
				imageRepoName, err = f.AsKubeAdmin.ImageController.GetImageName(testNamespace, customBranchComponentName)
				Expect(err).ShouldNot(HaveOccurred(), "failed to read image repo for component %s", customBranchComponentName)
				Expect(imageRepoName).ShouldNot(BeEmpty(), "image repo name is empty")

				imageExist, err := build.DoesImageRepoExistInQuay(imageRepoName)
				Expect(err).ShouldNot(HaveOccurred(), "failed while checking if image repo exists in quay with error: %+v", err)
				Expect(imageExist).To(BeTrue(), "quay image does not exists")

				pullRobotAccountName, pushRobotAccountName, err = f.AsKubeAdmin.ImageController.GetRobotAccounts(testNamespace, customBranchComponentName)
				Expect(err).ShouldNot(HaveOccurred(), "failed to get robot account names")
				pullRobotAccountExist, err := build.DoesRobotAccountExistInQuay(pullRobotAccountName)
				Expect(err).ShouldNot(HaveOccurred(), "failed while checking if pull robot account exists in quay with error: %+v", err)
				Expect(pullRobotAccountExist).To(BeTrue(), "pull robot account does not exists in quay")
				pushRobotAccountExist, err := build.DoesRobotAccountExistInQuay(pushRobotAccountName)
				Expect(err).ShouldNot(HaveOccurred(), "failed while checking if push robot account exists in quay with error: %+v", err)
				Expect(pushRobotAccountExist).To(BeTrue(), "push robot account does not exists in quay")

			})
			It("floating tags are created successfully", func() {
				builtImage := build.GetBinaryImage(plr)
				Expect(builtImage).ToNot(BeEmpty(), "built image url is empty")
				builtImageRef, err := reference.Parse(builtImage)
				Expect(err).ShouldNot(HaveOccurred(),
					fmt.Sprintf("cannot parse image pullspec: %s", builtImage))
				for _, tagName := range additionalTags {
					_, err := build.GetImageTag(builtImageRef.Namespace, builtImageRef.Name, tagName)
					Expect(err).ShouldNot(HaveOccurred(),
						fmt.Sprintf("failed to get tag %s from image repo", tagName),
					)
				}
			})
			It("created image repo is public", func() {
				isPublic, err := build.IsImageRepoPublic(imageRepoName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed while checking if the image repo %s is public", imageRepoName))
				Expect(isPublic).To(BeTrue(), fmt.Sprintf("Expected image repo '%s' to be changed to public, but it is private", imageRepoName))
			})

			It("image tag is updated successfully", func() {
				// check if the image tag exists in quay
				// ✅ CORRECT: Use the prHeadSha to get the specific successful PipelineRun
				plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, prHeadSha)
				Expect(err).ShouldNot(HaveOccurred())

				for _, p := range plr.Spec.Params {
					if p.Name == "output-image" {
						outputImage = p.Value.StringVal
					}
				}
				Expect(outputImage).ToNot(BeEmpty(), "output image %s of the component could not be found", outputImage)

				// Wait for image to be pushed to Quay - there can be a delay after PipelineRun completion
				Eventually(func() bool {
					isExists, err := build.DoesTagExistsInQuay(outputImage)
					if err != nil {
						GinkgoWriter.Printf("Error checking if image tag exists in Quay: %v\n", err)
						return false
					}
					if !isExists {
						GinkgoWriter.Printf("Image tag %s not yet available in Quay, retrying...\n", outputImage)
					} else {
						GinkgoWriter.Printf("Image tag %s successfully found in Quay\n", outputImage)
					}
					return isExists
				}, time.Minute*3, time.Second*10).Should(BeTrue(), fmt.Sprintf("image tag %s does not exist in quay after timeout", outputImage))
			})

			It("should ensure pruning labels are set", func() {
				plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())

				image, err := build.ImageFromPipelineRun(plr)
				Expect(err).ShouldNot(HaveOccurred())

				labels := image.Config.Config.Labels
				Expect(labels).ToNot(BeEmpty())

				expiration, ok := labels["quay.expires-after"]
				Expect(ok).To(BeTrue())
				Expect(expiration).To(Equal(utils.GetEnv(constants.IMAGE_TAG_EXPIRATION_ENV, constants.DefaultImageTagExpiration)))
			})
			It("eventually leads to the PipelineRun status report at Checks tab", func() {
				switch gitProvider {
				case git.GitHubProvider:
					expectedCheckRunName := fmt.Sprintf("%s-%s", customBranchComponentName, "on-pull-request")
					Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, helloWorldRepository, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
				case git.GitLabProvider:
					expectedStatusName := fmt.Sprintf("%s-%s", customBranchComponentName, "on-pull-request")
					Expect(f.AsKubeAdmin.HasController.GitLab.GetCommitStatusConclusion(expectedStatusName, helloWorldRepository, prHeadSha, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
				}
			})
		})

		When("the PaC init branch is updated", Label("build-custom-branch"), func() {
			var createdFileSHA string

			BeforeAll(func() {
				fileToCreatePath := fmt.Sprintf(".tekton/%s-readme.md", customBranchComponentName)

				createdFile, err := gitClient.CreateFile(helloWorldRepository, fileToCreatePath, fmt.Sprintf("test PaC branch %s update", pacBranchName), pacBranchName)
				Expect(err).ShouldNot(HaveOccurred())

				createdFileSHA = createdFile.CommitSHA
				GinkgoWriter.Println("created file sha:", createdFileSHA)
			})

			It("eventually leads to triggering another PipelineRun", func() {
				timeout = time.Minute * 5

				Eventually(func() error {
					plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, createdFileSHA)
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, customBranchComponentName)
						return err
					}
					if !plr.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, customBranchComponentName))
			})
			It("should lead to a PaC init PR update", func() {
				timeout = time.Second * 300
				interval = time.Second * 5

				Eventually(func() bool {
					prs, err := git.ListPullRequestsWithRetry(gitClient, helloWorldRepository)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.SourceBranch == pacBranchName {
							Expect(prHeadSha).NotTo(Equal(pr.HeadSHA))
							prNumber = pr.Number
							prHeadSha = pr.HeadSHA
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchName, helloWorldRepository))
			})
			It("PipelineRun should eventually finish", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", createdFileSHA, "",
					f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(Succeed())
				// in case the first pipelineRun attempt has failed and was retried, we need to update the git branch head ref
				createdFileSHA = plr.Labels["pipelinesascode.tekton.dev/sha"]
			})
			It("eventually leads to another update of a PR about the PipelineRun status report at Checks tab", func() {
				switch gitProvider {
				case git.GitHubProvider:
					expectedCheckRunName := fmt.Sprintf("%s-%s", customBranchComponentName, "on-pull-request")
					Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, helloWorldRepository, createdFileSHA, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
				case git.GitLabProvider:
					expectedStatusName := fmt.Sprintf("%s-%s", customBranchComponentName, "on-pull-request")
					Expect(f.AsKubeAdmin.HasController.GitLab.GetCommitStatusConclusion(expectedStatusName, helloWorldRepository, createdFileSHA, prNumber)).To(Equal(constants.CheckrunConclusionSuccess))
				}
			})
		})

		When("the PaC init branch is merged", Label("build-custom-branch"), func() {
			var mergeResult *git.PullRequest
			var mergeResultSha string

			BeforeAll(func() {
				Eventually(func() error {
					mergeResult, err = gitClient.MergePullRequest(helloWorldRepository, prNumber)
					return err
				}, time.Minute).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, helloWorldRepository))

				mergeResultSha = mergeResult.MergeCommitSHA
				GinkgoWriter.Println("merged result sha:", mergeResultSha)
			})

			It("eventually leads to triggering another PipelineRun", func() {
				timeout = time.Minute * 10

				Eventually(func() error {
					plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, mergeResultSha)
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, customBranchComponentName)
						return err
					}
					if !plr.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, customBranchComponentName))
			})

			It("pipelineRun should eventually finish", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "",
					mergeResultSha, "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(Succeed())
				mergeResultSha = plr.Labels["pipelinesascode.tekton.dev/sha"]
			})

			It("does not have expiration set", func() {
				image, err := build.ImageFromPipelineRun(plr)
				Expect(err).ShouldNot(HaveOccurred())

				labels := image.Config.Config.Labels
				Expect(labels).ToNot(BeEmpty())

				expiration, ok := labels["quay.expires-after"]
				Expect(ok).To(BeFalse())
				Expect(expiration).To(BeEmpty())
			})

			It("After updating image visibility to private, it should not trigger another PipelineRun", func() {
				Eventually(func() error {
					return f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
				// Wait for one minute so that all the pipelineruns deleted successfully
				Eventually(func() bool {
					componentPipelineRun, _ := f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, "")
					if componentPipelineRun != nil {
						GinkgoWriter.Printf("found pipelinerun: %s\n", componentPipelineRun.GetName())
					}
					return componentPipelineRun == nil
				}, time.Minute*3, time.Second*5).Should(BeTrue(), "all the pipelineruns are not deleted, still some pipelineruns exists")

				Eventually(func() error {
					_, err := f.AsKubeAdmin.ImageController.ChangeVisibilityToPrivate(testNamespace, applicationName, customBranchComponentName)
					if err != nil {
						GinkgoWriter.Printf("failed to change visibility to private with error %v\n", err)
						return err
					}
					return nil
				}, time.Second*20, time.Second*1).Should(Succeed(), fmt.Sprintf("timed out when trying to change visibility of the image repos to private in %s/%s", testNamespace, customBranchComponentName))

				GinkgoWriter.Printf("waiting for one minute and expecting to not trigger a PipelineRun")
				Consistently(func() bool {
					componentPipelineRun, _ := f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, "")
					if componentPipelineRun != nil {
						GinkgoWriter.Printf("While waiting for no pipeline to be triggered, found Pipelinerun: %s\n", componentPipelineRun.GetName())
					}
					return componentPipelineRun == nil
				}, 2*time.Minute, constants.PipelineRunPollingInterval).Should(BeTrue(), fmt.Sprintf("expected no PipelineRun to be triggered for the component %s in %s namespace", customBranchComponentName, testNamespace))
			})
			It("image repo is updated to private", func() {
				isPublic, err := build.IsImageRepoPublic(imageRepoName)
				Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("failed while checking if the image repo %s is private", imageRepoName))
				Expect(isPublic).To(BeFalse(), "Expected image repo to changed to private, but it is public")
			})
			It("retrigger the pipeline manually", func() {
				// Record existing PLR name so we can distinguish old from new after retrigger
				existingPLRName := ""
				if plr != nil {
					existingPLRName = plr.GetName()
				}

				Expect(f.AsKubeAdmin.HasController.SetComponentAnnotation(customBranchComponentName, controllers.BuildRequestAnnotationName, controllers.BuildRequestTriggerPaCBuildAnnotationValue, testNamespace)).To(Succeed())
				// Check the pipelinerun is triggered.
				// If build-service consumed the annotation (fire-and-forget HTTP POST to PaC)
				// but PaC didn't create the PipelineRun, re-set the annotation to retry.
				retriesLeft := 2
				Eventually(func() error {
					prs, err := f.AsKubeAdmin.HasController.GetComponentPipelineRunsWithType(customBranchComponentName, applicationName, testNamespace, "build", "", "incoming")
					if err != nil {
						GinkgoWriter.Printf("PipelineRun is not been retriggered yet for the component %s/%s\n", testNamespace, customBranchComponentName)
						// Check if the annotation was already consumed; if so, re-set it
						if retriesLeft > 0 {
							comp, getErr := f.AsKubeAdmin.HasController.GetComponent(customBranchComponentName, testNamespace)
							if getErr == nil {
								annotations := comp.GetAnnotations()
								if annotations == nil || annotations[controllers.BuildRequestAnnotationName] == "" {
									GinkgoWriter.Printf("Build request annotation was consumed but no PipelineRun appeared. Re-setting annotation (retries left: %d)\n", retriesLeft)
									_ = f.AsKubeAdmin.HasController.SetComponentAnnotation(customBranchComponentName, controllers.BuildRequestAnnotationName, controllers.BuildRequestTriggerPaCBuildAnnotationValue, testNamespace)
									retriesLeft--
								}
							}
						}
						return err
					}
					// Look for a NEW PipelineRun (different name from the existing one)
					for i := range *prs {
						candidate := &(*prs)[i]
						if candidate.GetName() != existingPLRName && candidate.HasStarted() {
							plr = candidate
							GinkgoWriter.Printf("New PipelineRun %s found after retrigger (old: %s)\n", plr.GetName(), existingPLRName)
							return nil
						}
					}
					return fmt.Errorf("no new PipelineRun found yet (existing: %s)", existingPLRName)
				}, 10*time.Minute, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to retrigger for the component %s/%s", testNamespace, customBranchComponentName))
			})
			It("retriggered pipelineRun should eventually finish", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "build", "", "incoming", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(Succeed())
			})
		})

		When("the component is removed and recreated (with the same name in the same namespace)", Label("build-custom-branch"), func() {
			var componentObj appservice.ComponentSpec

			BeforeAll(func() {
				Eventually(func() error {
					return f.AsKubeAdmin.HasController.DeleteComponent(customBranchComponentName, testNamespace, true)
				}, 2*time.Minute, 10*time.Second).Should(Succeed())

				timeout = 1 * time.Minute
				interval = 1 * time.Second
				Eventually(func() bool {
					_, err := f.AsKubeAdmin.HasController.GetComponent(customBranchComponentName, testNamespace)
					return k8sErrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("timed out when waiting for the app %s/%s to be deleted", testNamespace, applicationName))
				// Check removal of image repo
				Eventually(func() (bool, error) {
					return build.DoesImageRepoExistInQuay(imageRepoName)
				}, timeout, interval).Should(BeFalse(), fmt.Sprintf("timed out when waiting for image repo %s to be deleted", imageRepoName))
				// Check removal of robot accounts
				Eventually(func() (bool, error) {
					pullRobotAccountExists, err := build.DoesRobotAccountExistInQuay(pullRobotAccountName)
					if err != nil {
						return false, err
					}
					pushRobotAccountExists, err := build.DoesRobotAccountExistInQuay(pushRobotAccountName)
					if err != nil {
						return false, err
					}
					return pullRobotAccountExists || pushRobotAccountExists, nil
				}, timeout, interval).Should(BeFalse(), fmt.Sprintf("timed out when checking if robot accounts %s and %s got deleted", pullRobotAccountName, pushRobotAccountName))
			})

			BeforeAll(func() {
				componentObj = appservice.ComponentSpec{
					ComponentName: customBranchComponentName,
					Source: appservice.ComponentSource{
						ComponentSourceUnion: appservice.ComponentSourceUnion{
							GitSource: &appservice.GitSource{
								URL:           helloWorldComponentGitSourceURL,
								Revision:      componentBaseBranchName,
								DockerfileURL: constants.DockerFilePath,
							},
						},
					},
				}

				_, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterAll(func() {
				//Get the Purge PR number created after deleting the component
				Eventually(func() bool {
					prs, err := git.ListPullRequestsWithRetry(gitClient, helloWorldRepository)
					Expect(err).ShouldNot(HaveOccurred())

					for _, pr := range prs {
						if pr.TargetBranch == componentBaseBranchName {
							GinkgoWriter.Printf("Found purge PR with id: %d\n", pr.Number)
							purgePrNumber = pr.Number
							return true
						}
					}
					return false
				}, time.Minute, time.Second*10).Should(BeTrue(), fmt.Sprintf("timed out when waiting for purge PR with traget branch %s to be created in %s repository", componentBaseBranchName, helloWorldRepository))

			})

			It("should no longer lead to a creation of a PaC PR", func() {
				timeout = time.Second * 10
				interval = time.Second * 2
				Consistently(func() error {
					prs, err := git.ListPullRequestsWithRetry(gitClient, helloWorldRepository)
					if err != nil {
						// Transient API errors should not fail a Consistently check
						GinkgoWriter.Printf("error listing PRs in %s after retries (ignoring): %v\n", helloWorldRepository, err)
						return nil
					}

					for _, pr := range prs {
						if pr.SourceBranch == pacBranchName {
							return fmt.Errorf("did not expect a new PR created in %s repository after initial PaC configuration was already merged for the same component name and a namespace", helloWorldRepository)
						}
					}
					return nil
				}, timeout, interval).ShouldNot(HaveOccurred())
			})
		})
	},
		Entry("github", Label("github"), git.GitHubProvider, "gh"),
		Entry("gitlab", Label("gitlab"), git.GitLabProvider, "gl"),
	)
})
