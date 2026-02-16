package build

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-github/v83/github"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/build-service/controllers"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/image/reference"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/konflux-ci/e2e-tests/pkg/clients/git"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
)

var _ = framework.BuildSuiteDescribe("Build service E2E tests", ginkgo.Label("build-service"), func() {

	var f *framework.Framework
	ginkgo.AfterEach(framework.ReportFailure(&f))
	var err error
	defer ginkgo.GinkgoRecover()

	var gitClient git.Client

	ginkgo.DescribeTableSubtree("test PaC component build", ginkgo.Ordered, ginkgo.Label("github-webhook", "pac-build", "pipeline", "image-controller"), func(gitProvider git.GitProvider, gitPrefix string) {
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

		ginkgo.BeforeAll(func() {
			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				ginkgo.Skip("Skipping this test due to configuration issue with Spray proxy")
			}

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			testNamespace = f.UserNamespace

			if utils.IsPrivateHostname(f.OpenshiftConsoleHost) {
				ginkgo.Skip("Using private cluster (not reachable from Github), skipping...")
			}

			quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "")
			supports, err := build.DoesQuayOrgSupportPrivateRepo()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error while checking if quay org supports private repo: %+v", err))
			if !supports {
				if quayOrg == "redhat-appstudio-qe" {
					ginkgo.Fail("Failed to create private image repo in redhat-appstudio-qe org")
				} else {
					ginkgo.Skip("Quay org does not support private quay repository creation, please add support for private repo creation before running this test")
				}
			}
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			applicationName = fmt.Sprintf("build-suite-test-application-%s", util.GenerateRandomString(4))
			_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			customDefaultComponentName = fmt.Sprintf("%s-%s-%s", gitPrefix, "test-custom-default", util.GenerateRandomString(6))
			customBranchComponentName = fmt.Sprintf("%s-%s-%s", gitPrefix, "test-custom-branch", util.GenerateRandomString(6))
			pacBranchName = constants.PaCPullRequestBranchPrefix + customBranchComponentName
			customDefaultComponentBranch = constants.PaCPullRequestBranchPrefix + customDefaultComponentName
			componentBaseBranchName = fmt.Sprintf("base-%s", util.GenerateRandomString(6))

			gitClient, helloWorldComponentGitSourceURL, helloWorldRepository = setupGitProvider(f, gitProvider)
			// get the build pipeline bundle annotation
			buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

			err = gitClient.CreateBranch(helloWorldRepository, helloWorldComponentDefaultBranch, helloWorldComponentRevision, componentBaseBranchName)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.AfterAll(func() {
			if !ginkgo.CurrentSpecReport().Failed() {
				gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
				gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
				gomega.Expect(gitClient.DeleteRepositoryIfExists(helloWorldRepository)).To(gomega.Succeed())
			}

			ginkgo.When("a new component without specified branch is created and with visibility private", ginkgo.Label("pac-custom-default-branch"), func() {
				var componentObj appservice.ComponentSpec

				ginkgo.BeforeAll(func() {
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
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				ginkgo.It("correctly targets the default branch (that is not named 'main') with PaC", func() {
					timeout = time.Second * 300
					interval = time.Second * 5
					gomega.Eventually(func() bool {
						prs, err := gitClient.ListPullRequests(helloWorldRepository)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						for _, pr := range prs {
							if pr.SourceBranch == customDefaultComponentBranch {
								gomega.Expect(pr.TargetBranch).To(gomega.Equal(helloWorldComponentDefaultBranch))
								return true
							}
						}
						return false
					}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR to be created against %s branch in %s repository", helloWorldComponentDefaultBranch, helloWorldRepository))
				})

				ginkgo.It("workspace parameter is set correctly in PaC repository CR", func() {
					nsObj, err := f.AsKubeAdmin.CommonController.GetNamespace(testNamespace)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					wsName := nsObj.Labels["appstudio.redhat.com/workspace_name"]
					repositoryParams, err := f.AsKubeAdmin.TektonController.GetRepositoryParams(customDefaultComponentName, testNamespace)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "error while trying to get repository params")
					paramExists := false
					for _, param := range repositoryParams {
						if param.Name == "appstudio_workspace" {
							paramExists = true
							gomega.Expect(param.Value).To(gomega.Equal(wsName), fmt.Sprintf("got workspace param value: %s, expected %s", param.Value, wsName))
						}
					}
					gomega.Expect(paramExists).To(gomega.BeTrue(), "appstudio_workspace param does not exists in repository CR")

				})
				ginkgo.It("triggers a PipelineRun", func() {
					timeout = time.Minute * 5
					gomega.Eventually(func() error {
						plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customDefaultComponentName, applicationName, testNamespace, "")
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, customBranchComponentName)
							return err
						}
						if !plr.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", customBranchComponentName, testNamespace))
				})
				ginkgo.It("build pipeline uses the correct serviceAccount", func() {
					serviceAccountName := "build-pipeline-" + customDefaultComponentName
					gomega.Expect(plr.Spec.TaskRunTemplate.ServiceAccountName).Should(gomega.Equal(serviceAccountName))
				})
				ginkgo.It("component build status is set correctly", func() {
					var buildStatus *controllers.BuildStatus
					gomega.Eventually(func() (bool, error) {
						component, err := f.AsKubeAdmin.HasController.GetComponent(customDefaultComponentName, testNamespace)
						if err != nil {
							return false, err
						}

						buildStatusAnnotationValue := component.Annotations[controllers.BuildStatusAnnotationName]
						ginkgo.GinkgoWriter.Printf(buildStatusAnnotationValueLoggingFormat, buildStatusAnnotationValue)
						statusBytes := []byte(buildStatusAnnotationValue)

						err = json.Unmarshal(statusBytes, &buildStatus)
						if err != nil {
							return false, err
						}

						if buildStatus.PaC != nil {
							ginkgo.GinkgoWriter.Printf("state: %s\n", buildStatus.PaC.State)
							ginkgo.GinkgoWriter.Printf("mergeUrl: %s\n", buildStatus.PaC.MergeUrl)
							ginkgo.GinkgoWriter.Printf("errId: %d\n", buildStatus.PaC.ErrId)
							ginkgo.GinkgoWriter.Printf("errMessage: %s\n", buildStatus.PaC.ErrMessage)
							ginkgo.GinkgoWriter.Printf("configurationTime: %s\n", buildStatus.PaC.ConfigurationTime)
						} else {
							ginkgo.GinkgoWriter.Println("build status does not have PaC field")
						}

						return buildStatus.PaC != nil && buildStatus.PaC.State == "enabled" && buildStatus.PaC.MergeUrl != "" && buildStatus.PaC.ErrId == 0 && buildStatus.PaC.ConfigurationTime != "", nil
					}, timeout, interval).Should(gomega.BeTrue(), "component build status has unexpected content")
				})
				ginkgo.It("image repo and robot account created successfully", func() {
					imageRepoName, err = f.AsKubeAdmin.ImageController.GetImageName(testNamespace, customDefaultComponentName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to read image repo for component %s", customDefaultComponentName)
					gomega.Expect(imageRepoName).ShouldNot(gomega.BeEmpty(), "image repo name is empty")

					imageExist, err := build.DoesImageRepoExistInQuay(imageRepoName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed while checking if image repo exists in quay with error: %+v", err)
					gomega.Expect(imageExist).To(gomega.BeTrue(), "quay image does not exists")

					pullRobotAccountName, pushRobotAccountName, err = f.AsKubeAdmin.ImageController.GetRobotAccounts(testNamespace, customDefaultComponentName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to get robot account names")
					pullRobotAccountExist, err := build.DoesRobotAccountExistInQuay(pullRobotAccountName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed while checking if pull robot account exists in quay with error: %+v", err)
					gomega.Expect(pullRobotAccountExist).To(gomega.BeTrue(), "pull robot account does not exists in quay")
					pushRobotAccountExist, err := build.DoesRobotAccountExistInQuay(pushRobotAccountName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed while checking if push robot account exists in quay with error: %+v", err)
					gomega.Expect(pushRobotAccountExist).To(gomega.BeTrue(), "push robot account does not exists in quay")
				})
				ginkgo.It("created image repo is private", func() {
					isPublic, err := build.IsImageRepoPublic(imageRepoName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("failed while checking if the image repo %s is private", imageRepoName))
					gomega.Expect(isPublic).To(gomega.BeFalse(), "Expected image repo to be private, but it is public")
				})

				ginkgo.It("a related PipelineRun should be deleted after deleting the component", func() {
					timeout = time.Second * 180
					interval = time.Second * 5
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteComponent(customDefaultComponentName, testNamespace, true)).To(gomega.Succeed())
					// Test removal of PipelineRun
					gomega.Eventually(func() error {
						plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customDefaultComponentName, applicationName, testNamespace, "")
						if err == nil {
							return fmt.Errorf("pipelinerun %s/%s is not removed yet", plr.GetNamespace(), plr.GetName())
						}
						return err
					}, timeout, interval).Should(gomega.MatchError(gomega.ContainSubstring("no pipelinerun found")), fmt.Sprintf("timed out when waiting for the PipelineRun to be removed for Component %s/%s", testNamespace, customBranchComponentName))
				})

				ginkgo.It("PR branch should not exist in the repo", func() {
					timeout = time.Second * 60
					interval = time.Second * 1
					gomega.Eventually(func() (bool, error) {
						exists, err := gitClient.BranchExists(helloWorldRepository, customDefaultComponentBranch)
						if err != nil {
							gomega.Expect(err.Error()).To(gomega.Or(gomega.ContainSubstring("Reference does not exist"), gomega.ContainSubstring("404")))
							return false, nil
						}
						return exists, nil
					}, timeout, interval).Should(gomega.BeFalse(), fmt.Sprintf("timed out when waiting for the branch %s to be deleted from %s repository", customDefaultComponentBranch, helloWorldRepository))
				})

				ginkgo.It("related image repo and the robot account should be deleted after deleting the component", func() {
					timeout = time.Second * 60
					interval = time.Second * 1
					// Check image repo should be deleted
					gomega.Eventually(func() (bool, error) {
						return build.DoesImageRepoExistInQuay(imageRepoName)
					}, timeout, interval).Should(gomega.BeFalse(), fmt.Sprintf("timed out when waiting for image repo %s to be deleted", imageRepoName))

					// Check robot account should be deleted
					gomega.Eventually(func() (bool, error) {
						pullRobotAccountExists, err := build.DoesRobotAccountExistInQuay(pullRobotAccountName)
						if err != nil {
							return false, err
						}
						pushRobotAccountExists, err := build.DoesRobotAccountExistInQuay(pushRobotAccountName)
						if err != nil {
							return false, err
						}
						return pullRobotAccountExists || pushRobotAccountExists, nil
					}, timeout, interval).Should(gomega.BeFalse(), fmt.Sprintf("timed out when checking if robot accounts %s and %s got deleted", pullRobotAccountName, pushRobotAccountName))

				})
			})

			ginkgo.When("a new Component with specified custom branch is created", ginkgo.Label("build-custom-branch"), func() {
				var outputImage string
				var componentObj appservice.ComponentSpec

				ginkgo.BeforeAll(func() {
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
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})
				ginkgo.AfterAll(func() {
					// Close Pruge PR if exists
					err = gitClient.DeleteBranchAndClosePullRequest(helloWorldRepository, purgePrNumber)
					if err != nil {
						gomega.Expect(err.Error()).To(gomega.Or(gomega.ContainSubstring("Reference does not exist"), gomega.ContainSubstring("404")))
					}
				})

				ginkgo.It("triggers a PipelineRun", func() {
					timeout = time.Second * 600
					interval = time.Second * 1
					gomega.Eventually(func() error {
						plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, "")
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, customBranchComponentName)
							return err
						}
						if !plr.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, customBranchComponentName))
				})
				ginkgo.It("should lead to a PaC init PR creation", func() {
					timeout = time.Second * 300
					interval = time.Second * 5

					gomega.Eventually(func() bool {
						prs, err := gitClient.ListPullRequests(helloWorldRepository)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						for _, pr := range prs {
							if pr.SourceBranch == pacBranchName {
								prNumber = pr.Number
								prHeadSha = pr.HeadSHA
								return true
							}
						}
						return false
					}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchName, helloWorldRepository))
				})
				ginkgo.It("the PipelineRun should eventually finish successfully", func() {
					gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", "", "",
						f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(gomega.Succeed())
					// in case the first pipelineRun attempt has failed and was retried, we need to update the git branch head ref
					prHeadSha = plr.Labels["pipelinesascode.tekton.dev/sha"]
				})
				ginkgo.It("image repo and robot account created successfully", func() {
					imageRepoName, err = f.AsKubeAdmin.ImageController.GetImageName(testNamespace, customBranchComponentName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to read image repo for component %s", customBranchComponentName)
					gomega.Expect(imageRepoName).ShouldNot(gomega.BeEmpty(), "image repo name is empty")

					imageExist, err := build.DoesImageRepoExistInQuay(imageRepoName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed while checking if image repo exists in quay with error: %+v", err)
					gomega.Expect(imageExist).To(gomega.BeTrue(), "quay image does not exists")

					pullRobotAccountName, pushRobotAccountName, err = f.AsKubeAdmin.ImageController.GetRobotAccounts(testNamespace, customBranchComponentName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to get robot account names")
					pullRobotAccountExist, err := build.DoesRobotAccountExistInQuay(pullRobotAccountName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed while checking if pull robot account exists in quay with error: %+v", err)
					gomega.Expect(pullRobotAccountExist).To(gomega.BeTrue(), "pull robot account does not exists in quay")
					pushRobotAccountExist, err := build.DoesRobotAccountExistInQuay(pushRobotAccountName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed while checking if push robot account exists in quay with error: %+v", err)
					gomega.Expect(pushRobotAccountExist).To(gomega.BeTrue(), "push robot account does not exists in quay")

				})
				ginkgo.It("floating tags are created successfully", func() {
					builtImage := build.GetBinaryImage(plr)
					gomega.Expect(builtImage).ToNot(gomega.BeEmpty(), "built image url is empty")
					builtImageRef, err := reference.Parse(builtImage)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(),
						fmt.Sprintf("cannot parse image pullspec: %s", builtImage))
					for _, tagName := range additionalTags {
						_, err := build.GetImageTag(builtImageRef.Namespace, builtImageRef.Name, tagName)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred(),
							fmt.Sprintf("failed to get tag %s from image repo", tagName),
						)
					}
				})
				ginkgo.It("created image repo is public", func() {
					isPublic, err := build.IsImageRepoPublic(imageRepoName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("failed while checking if the image repo %s is public", imageRepoName))
					gomega.Expect(isPublic).To(gomega.BeTrue(), fmt.Sprintf("Expected image repo '%s' to be changed to public, but it is private", imageRepoName))
				})

				ginkgo.It("image tag is updated successfully", func() {
					// check if the image tag exists in quay
					// ✅ CORRECT: Use the prHeadSha to get the specific successful PipelineRun
					plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, prHeadSha)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					for _, p := range plr.Spec.Params {
						if p.Name == "output-image" {
							outputImage = p.Value.StringVal
						}
					}
					gomega.Expect(outputImage).ToNot(gomega.BeEmpty(), "output image %s of the component could not be found", outputImage)

					// Wait for image to be pushed to Quay - there can be a delay after PipelineRun completion
					gomega.Eventually(func() bool {
						isExists, err := build.DoesTagExistsInQuay(outputImage)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("Error checking if image tag exists in Quay: %v\n", err)
							return false
						}
						if !isExists {
							ginkgo.GinkgoWriter.Printf("Image tag %s not yet available in Quay, retrying...\n", outputImage)
						} else {
							ginkgo.GinkgoWriter.Printf("Image tag %s successfully found in Quay\n", outputImage)
						}
						return isExists
					}, time.Minute*3, time.Second*10).Should(gomega.BeTrue(), fmt.Sprintf("image tag %s does not exist in quay after timeout", outputImage))
				})

				ginkgo.It("should ensure pruning labels are set", func() {
					plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, "")
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					image, err := build.ImageFromPipelineRun(plr)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					labels := image.Config.Config.Labels
					gomega.Expect(labels).ToNot(gomega.BeEmpty())

					expiration, ok := labels["quay.expires-after"]
					gomega.Expect(ok).To(gomega.BeTrue())
					gomega.Expect(expiration).To(gomega.Equal(utils.GetEnv(constants.IMAGE_TAG_EXPIRATION_ENV, constants.DefaultImageTagExpiration)))
				})
				ginkgo.It("eventually leads to the PipelineRun status report at Checks tab", func() {
					switch gitProvider {
					case git.GitHubProvider:
						expectedCheckRunName := fmt.Sprintf("%s-%s", customBranchComponentName, "on-pull-request")
						gomega.Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, helloWorldRepository, prHeadSha, prNumber)).To(gomega.Equal(constants.CheckrunConclusionSuccess))
					case git.GitLabProvider:
						expectedStatusName := fmt.Sprintf("%s-%s", customBranchComponentName, "on-pull-request")
						gomega.Expect(f.AsKubeAdmin.HasController.GitLab.GetCommitStatusConclusion(expectedStatusName, helloWorldRepository, prHeadSha, prNumber)).To(gomega.Equal(constants.CheckrunConclusionSuccess))
					}
				})
			})

			ginkgo.When("the PaC init branch is updated", ginkgo.Label("build-custom-branch"), func() {
				var createdFileSHA string

				ginkgo.BeforeAll(func() {
					fileToCreatePath := fmt.Sprintf(".tekton/%s-readme.md", customBranchComponentName)

					createdFile, err := gitClient.CreateFile(helloWorldRepository, fileToCreatePath, fmt.Sprintf("test PaC branch %s update", pacBranchName), pacBranchName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					createdFileSHA = createdFile.CommitSHA
					ginkgo.GinkgoWriter.Println("created file sha:", createdFileSHA)
				})

				ginkgo.It("eventually leads to triggering another PipelineRun", func() {
					timeout = time.Minute * 5

					gomega.Eventually(func() error {
						plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, createdFileSHA)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, customBranchComponentName)
							return err
						}
						if !plr.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, customBranchComponentName))
				})
				ginkgo.It("should lead to a PaC init PR update", func() {
					timeout = time.Second * 300
					interval = time.Second * 5

					gomega.Eventually(func() bool {
						prs, err := gitClient.ListPullRequests(helloWorldRepository)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						for _, pr := range prs {
							if pr.SourceBranch == pacBranchName {
								gomega.Expect(prHeadSha).NotTo(gomega.Equal(pr.HeadSHA))
								prNumber = pr.Number
								prHeadSha = pr.HeadSHA
								return true
							}
						}
						return false
					}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for init PaC PR (branch name '%s') to be created in %s repository", pacBranchName, helloWorldRepository))
				})
				ginkgo.It("PipelineRun should eventually finish", func() {
					gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", createdFileSHA, "",
						f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(gomega.Succeed())
					// in case the first pipelineRun attempt has failed and was retried, we need to update the git branch head ref
					createdFileSHA = plr.Labels["pipelinesascode.tekton.dev/sha"]
				})
				ginkgo.It("eventually leads to another update of a PR about the PipelineRun status report at Checks tab", func() {
					switch gitProvider {
					case git.GitHubProvider:
						expectedCheckRunName := fmt.Sprintf("%s-%s", customBranchComponentName, "on-pull-request")
						gomega.Expect(f.AsKubeAdmin.CommonController.Github.GetCheckRunConclusion(expectedCheckRunName, helloWorldRepository, createdFileSHA, prNumber)).To(gomega.Equal(constants.CheckrunConclusionSuccess))
					case git.GitLabProvider:
						expectedStatusName := fmt.Sprintf("%s-%s", customBranchComponentName, "on-pull-request")
						gomega.Expect(f.AsKubeAdmin.HasController.GitLab.GetCommitStatusConclusion(expectedStatusName, helloWorldRepository, createdFileSHA, prNumber)).To(gomega.Equal(constants.CheckrunConclusionSuccess))
					}
				})
			})

			ginkgo.When("the PaC init branch is merged", ginkgo.Label("build-custom-branch"), func() {
				var mergeResult *git.PullRequest
				var mergeResultSha string

				ginkgo.BeforeAll(func() {
					gomega.Eventually(func() error {
						mergeResult, err = gitClient.MergePullRequest(helloWorldRepository, prNumber)
						return err
					}, time.Minute).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, helloWorldRepository))

					mergeResultSha = mergeResult.MergeCommitSHA
					ginkgo.GinkgoWriter.Println("merged result sha:", mergeResultSha)
				})

				ginkgo.It("eventually leads to triggering another PipelineRun", func() {
					timeout = time.Minute * 10

					gomega.Eventually(func() error {
						plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, mergeResultSha)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, customBranchComponentName)
							return err
						}
						if !plr.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", plr.GetNamespace(), plr.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, customBranchComponentName))
				})

				ginkgo.It("pipelineRun should eventually finish", func() {
					gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "",
						mergeResultSha, "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(gomega.Succeed())
					mergeResultSha = plr.Labels["pipelinesascode.tekton.dev/sha"]
				})

				ginkgo.It("does not have expiration set", func() {
					image, err := build.ImageFromPipelineRun(plr)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					labels := image.Config.Config.Labels
					gomega.Expect(labels).ToNot(gomega.BeEmpty())

					expiration, ok := labels["quay.expires-after"]
					gomega.Expect(ok).To(gomega.BeFalse())
					gomega.Expect(expiration).To(gomega.BeEmpty())
				})

				ginkgo.It("After updating image visibility to private, it should not trigger another PipelineRun", func() {
					gomega.Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(gomega.Succeed())
					// Wait for one minute so that all the pipelineruns deleted successfully
					gomega.Eventually(func() bool {
						componentPipelineRun, _ := f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, "")
						if componentPipelineRun != nil {
							ginkgo.GinkgoWriter.Printf("found pipelinerun: %s\n", componentPipelineRun.GetName())
						}
						return componentPipelineRun == nil
					}, time.Minute*3, time.Second*5).Should(gomega.BeTrue(), "all the pipelineruns are not deleted, still some pipelineruns exists")

					gomega.Eventually(func() error {
						_, err := f.AsKubeAdmin.ImageController.ChangeVisibilityToPrivate(testNamespace, applicationName, customBranchComponentName)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("failed to change visibility to private with error %v\n", err)
							return err
						}
						return nil
					}, time.Second*20, time.Second*1).Should(gomega.Succeed(), fmt.Sprintf("timed out when trying to change visibility of the image repos to private in %s/%s", testNamespace, customBranchComponentName))

					ginkgo.GinkgoWriter.Printf("waiting for one minute and expecting to not trigger a PipelineRun")
					gomega.Consistently(func() bool {
						componentPipelineRun, _ := f.AsKubeAdmin.HasController.GetComponentPipelineRun(customBranchComponentName, applicationName, testNamespace, "")
						if componentPipelineRun != nil {
							ginkgo.GinkgoWriter.Printf("While waiting for no pipeline to be triggered, found Pipelinerun: %s\n", componentPipelineRun.GetName())
						}
						return componentPipelineRun == nil
					}, 2*time.Minute, constants.PipelineRunPollingInterval).Should(gomega.BeTrue(), fmt.Sprintf("expected no PipelineRun to be triggered for the component %s in %s namespace", customBranchComponentName, testNamespace))
				})
				ginkgo.It("image repo is updated to private", func() {
					isPublic, err := build.IsImageRepoPublic(imageRepoName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("failed while checking if the image repo %s is private", imageRepoName))
					gomega.Expect(isPublic).To(gomega.BeFalse(), "Expected image repo to changed to private, but it is public")
				})
				ginkgo.It("retrigger the pipeline manually", func() {
					gomega.Expect(f.AsKubeAdmin.HasController.SetComponentAnnotation(customBranchComponentName, controllers.BuildRequestAnnotationName, controllers.BuildRequestTriggerPaCBuildAnnotationValue, testNamespace)).To(gomega.Succeed())
					// Check the pipelinerun is triggered
					gomega.Eventually(func() error {
						plr, err = f.AsKubeAdmin.HasController.GetComponentPipelineRunWithType(customBranchComponentName, applicationName, testNamespace, "build", "", "incoming")
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun is not been retriggered yet for the component %s/%s\n", testNamespace, customBranchComponentName)
							return err
						}
						if !plr.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't been started yet", plr.GetNamespace(), plr.GetName())
						}
						return nil
					}, 10*time.Minute, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to retrigger for the component %s/%s", testNamespace, customBranchComponentName))
				})
				ginkgo.It("retriggered pipelineRun should eventually finish", func() {
					gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "build", "", "incoming", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, plr)).To(gomega.Succeed())
				})
			})

			ginkgo.When("the component is removed and recreated (with the same name in the same namespace)", ginkgo.Label("build-custom-branch"), func() {
				var componentObj appservice.ComponentSpec

				ginkgo.BeforeAll(func() {
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteComponent(customBranchComponentName, testNamespace, true)).To(gomega.Succeed())

					timeout = 1 * time.Minute
					interval = 1 * time.Second
					gomega.Eventually(func() bool {
						_, err := f.AsKubeAdmin.HasController.GetComponent(customBranchComponentName, testNamespace)
						return k8sErrors.IsNotFound(err)
					}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for the app %s/%s to be deleted", testNamespace, applicationName))
					// Check removal of image repo
					gomega.Eventually(func() (bool, error) {
						return build.DoesImageRepoExistInQuay(imageRepoName)
					}, timeout, interval).Should(gomega.BeFalse(), fmt.Sprintf("timed out when waiting for image repo %s to be deleted", imageRepoName))
					// Check removal of robot accounts
					gomega.Eventually(func() (bool, error) {
						pullRobotAccountExists, err := build.DoesRobotAccountExistInQuay(pullRobotAccountName)
						if err != nil {
							return false, err
						}
						pushRobotAccountExists, err := build.DoesRobotAccountExistInQuay(pushRobotAccountName)
						if err != nil {
							return false, err
						}
						return pullRobotAccountExists || pushRobotAccountExists, nil
					}, timeout, interval).Should(gomega.BeFalse(), fmt.Sprintf("timed out when checking if robot accounts %s and %s got deleted", pullRobotAccountName, pushRobotAccountName))
				})

				ginkgo.BeforeAll(func() {
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
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				ginkgo.AfterAll(func() {
					//Get the Purge PR number created after deleting the component
					gomega.Eventually(func() bool {
						prs, err := gitClient.ListPullRequests(helloWorldRepository)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						for _, pr := range prs {
							if pr.TargetBranch == componentBaseBranchName {
								ginkgo.GinkgoWriter.Printf("Found purge PR with id: %d\n", pr.Number)
								purgePrNumber = pr.Number
								return true
							}
						}
						return false
					}, time.Minute, time.Second*10).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for purge PR with traget branch %s to be created in %s repository", componentBaseBranchName, helloWorldRepository))

				})

				ginkgo.It("should no longer lead to a creation of a PaC PR", func() {
					timeout = time.Second * 10
					interval = time.Second * 2
					gomega.Consistently(func() error {
						prs, err := gitClient.ListPullRequests(helloWorldRepository)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						for _, pr := range prs {
							if pr.SourceBranch == pacBranchName {
								return fmt.Errorf("did not expect a new PR created in %s repository after initial PaC configuration was already merged for the same component name and a namespace", helloWorldRepository)
							}
						}
						return nil
					}, timeout, interval).ShouldNot(gomega.HaveOccurred())
				})
			})
		},
			ginkgo.Entry("github", git.GitHubProvider, "gh"),
			ginkgo.Entry("gitlab", git.GitLabProvider, "gl"),
		)

		ginkgo.Describe("test pac with multiple components using same repository", ginkgo.Ordered, ginkgo.Label("pac-build", "multi-component"), func() {
			var applicationName, testNamespace, multiComponentBaseBranchName, multiComponentPRBranchName, mergeResultSha string
			var pacBranchNames []string
			var prNumber int
			var mergeResult *github.PullRequestMergeResult
			var timeout time.Duration
			var buildPipelineAnnotation map[string]string

			ginkgo.BeforeAll(func() {
				if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
					ginkgo.Skip("Skipping this test due to configuration issue with Spray proxy")
				}
				f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				testNamespace = f.UserNamespace

				if utils.IsPrivateHostname(f.OpenshiftConsoleHost) {
					ginkgo.Skip("Using private cluster (not reachable from Github), skipping...")
				}

				applicationName = fmt.Sprintf("build-suite-positive-mc-%s", util.GenerateRandomString(4))
				_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				multiComponentBaseBranchName = fmt.Sprintf("multi-component-base-%s", util.GenerateRandomString(6))
				err = f.AsKubeAdmin.CommonController.Github.CreateRef(multiComponentGitSourceRepoName, multiComponentDefaultBranch, multiComponentGitRevision, multiComponentBaseBranchName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				//Branch for creating pull request
				multiComponentPRBranchName = fmt.Sprintf("%s-%s", "pr-branch", util.GenerateRandomString(6))

				// get the build pipeline bundle annotation
				buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

			})

			ginkgo.AfterAll(func() {
				if !ginkgo.CurrentSpecReport().Failed() {
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
				}

				// Delete new branches created by PaC and a testing branch used as a component's base branch
				for _, pacBranchName := range pacBranchNames {
					err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentGitSourceRepoName, pacBranchName)
					if err != nil {
						gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
					}
				}
				// Delete the created base branch
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentGitSourceRepoName, multiComponentBaseBranchName)
				if err != nil {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
				}
				// Delete the created pr branch
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(multiComponentGitSourceRepoName, multiComponentPRBranchName)
				if err != nil {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
				}
			})

			ginkgo.When("components are created in same namespace", func() {
				var component *appservice.Component

				for _, contextDir := range multiComponentContextDirs {
					contextDir := contextDir
					componentName := fmt.Sprintf("%s-%s", contextDir, util.GenerateRandomString(6))
					pacBranchName := constants.PaCPullRequestBranchPrefix + componentName
					pacBranchNames = append(pacBranchNames, pacBranchName)

					ginkgo.It(fmt.Sprintf("creates component with context directory %s", contextDir), func() {
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
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					})

					ginkgo.It(fmt.Sprintf("triggers a PipelineRun for component %s", componentName), func() {
						timeout = time.Minute * 5
						gomega.Eventually(func() error {
							pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
							if err != nil {
								ginkgo.GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
								return err
							}
							if !pr.HasStarted() {
								return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
							}
							return nil
						}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", componentName, testNamespace))
					})

					ginkgo.It(fmt.Sprintf("should lead to a PaC PR creation for component %s", componentName), func() {
						timeout = time.Second * 300
						interval := time.Second * 5

						gomega.Eventually(func() bool {
							prs, err := f.AsKubeAdmin.CommonController.Github.ListPullRequests(multiComponentGitSourceRepoName)
							gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

							for _, pr := range prs {
								if pr.Head.GetRef() == pacBranchName {
									prNumber = pr.GetNumber()
									return true
								}
							}
							return false
						}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for PaC PR (branch name '%s') to be created in %s repository", pacBranchName, multiComponentGitSourceRepoName))
					})

					ginkgo.It(fmt.Sprintf("the PipelineRun should eventually finish successfully for component %s", componentName), func() {
						gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", "", "",
							f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true}, nil)).To(gomega.Succeed())
					})

					ginkgo.It("merging the PR should be successful", func() {
						gomega.Eventually(func() error {
							mergeResult, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(multiComponentGitSourceRepoName, prNumber)
							return err
						}, time.Minute).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, multiComponentGitSourceRepoName))

						mergeResultSha = mergeResult.GetSHA()
						ginkgo.GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)

					})
					ginkgo.It("leads to triggering on push PipelineRun", func() {
						timeout = time.Minute * 5

						gomega.Eventually(func() error {
							pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, mergeResultSha)
							if err != nil {
								ginkgo.GinkgoWriter.Printf("Push PipelineRun has not been created yet for the component %s/%s\n", testNamespace, componentName)
								return err
							}
							if !pipelineRun.HasStarted() {
								return fmt.Errorf("push pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
							}
							return nil
						}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, componentName))
					})
				}
				ginkgo.It("only one component is changed", func() {
					//Delete all the pipelineruns in the namespace before sending PR
					gomega.Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(gomega.Succeed())
					//Create the ref, add the file and create the PR
					err = f.AsKubeAdmin.CommonController.Github.CreateRef(multiComponentGitSourceRepoName, multiComponentDefaultBranch, mergeResultSha, multiComponentPRBranchName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					fileToCreatePath := fmt.Sprintf("%s/sample-file.txt", multiComponentContextDirs[0])
					createdFileSha, err := f.AsKubeAdmin.CommonController.Github.CreateFile(multiComponentGitSourceRepoName, fileToCreatePath, fmt.Sprintf("sample test file inside %s", multiComponentContextDirs[0]), multiComponentPRBranchName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error while creating file: %s", fileToCreatePath))
					pr, err := f.AsKubeAdmin.CommonController.Github.CreatePullRequest(multiComponentGitSourceRepoName, "sample pr title", "sample pr body", multiComponentPRBranchName, multiComponentBaseBranchName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					ginkgo.GinkgoWriter.Printf("PR #%d got created with sha %s\n", pr.GetNumber(), createdFileSha.GetSHA())
				})
				ginkgo.It("only related pipelinerun should be triggered", func() {
					gomega.Eventually(func() error {
						pipelineRuns, err := f.AsKubeAdmin.HasController.GetAllPipelineRunsForApplication(applicationName, testNamespace)
						if err != nil {
							ginkgo.GinkgoWriter.Println("on pull PiplelineRun has not been created yet for the PR")
							return err
						}
						if len(pipelineRuns.Items) != 1 || !strings.HasPrefix(pipelineRuns.Items[0].Name, multiComponentContextDirs[0]) {
							return fmt.Errorf("pipelinerun created in the namespace %s is not as expected, got pipelineruns %v", testNamespace, pipelineRuns.Items)
						}
						return nil
					}, time.Minute*5, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), "timeout while waiting for PR pipeline to start")
				})
			})
			ginkgo.When("a components is created with same git url in different namespace", func() {
				var namespace, appName, compName string
				var fw *framework.Framework

				ginkgo.BeforeAll(func() {
					fw, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					namespace = fw.UserNamespace

					appName = fmt.Sprintf("build-suite-negative-mc-%s", util.GenerateRandomString(4))
					_, err = f.AsKubeAdmin.HasController.CreateApplication(appName, namespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

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
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				})

				ginkgo.AfterAll(func() {
					if !ginkgo.CurrentSpecReport().Failed() {
						gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
						gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
					}
				})

				ginkgo.It("should fail to configure PaC for the component", func() {
					var buildStatus *controllers.BuildStatus

					gomega.Eventually(func() (bool, error) {
						component, err := fw.AsKubeAdmin.HasController.GetComponent(compName, namespace)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("error while getting the component: %v\n", err)
							return false, err
						}

						buildStatusAnnotationValue := component.Annotations[controllers.BuildStatusAnnotationName]
						ginkgo.GinkgoWriter.Printf(buildStatusAnnotationValueLoggingFormat, buildStatusAnnotationValue)
						statusBytes := []byte(buildStatusAnnotationValue)

						err = json.Unmarshal(statusBytes, &buildStatus)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("cannot unmarshal build status from component annotation: %v\n", err)
							return false, err
						}

						ginkgo.GinkgoWriter.Printf("build status: %+v\n", buildStatus.PaC)

						return buildStatus.PaC != nil && buildStatus.PaC.State == "error" && strings.Contains(buildStatus.PaC.ErrMessage, "Git repository is already handled by Pipelines as Code"), nil
					}, time.Minute*2, time.Second*2).Should(gomega.BeTrue(), "build status is unexpected")

				})

			})

		})
		ginkgo.Describe("test build secret lookup", ginkgo.Label("pac-build", "secret-lookup"), ginkgo.Ordered, func() {
			var testNamespace, applicationName, firstComponentBaseBranchName, secondComponentBaseBranchName, firstComponentName, secondComponentName, firstPacBranchName, secondPacBranchName string
			var buildPipelineAnnotation map[string]string
			ginkgo.BeforeAll(func() {
				if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
					ginkgo.Skip("Skipping this test due to configuration issue with Spray proxy")
				}
				f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				testNamespace = f.UserNamespace

				applicationName = fmt.Sprintf("build-secret-lookup-%s", util.GenerateRandomString(4))
				_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				// Update the default github org
				f.AsKubeAdmin.CommonController.Github.UpdateGithubOrg(noAppOrgName)

				firstComponentBaseBranchName = fmt.Sprintf("component-one-base-%s", util.GenerateRandomString(6))
				err = f.AsKubeAdmin.CommonController.Github.CreateRef(secretLookupGitSourceRepoOneName, secretLookupDefaultBranchOne, secretLookupGitRevisionOne, firstComponentBaseBranchName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				secondComponentBaseBranchName = fmt.Sprintf("component-two-base-%s", util.GenerateRandomString(6))
				err = f.AsKubeAdmin.CommonController.Github.CreateRef(secretLookupGitSourceRepoTwoName, secretLookupDefaultBranchTwo, secretLookupGitRevisionTwo, secondComponentBaseBranchName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use custom bundle if env defined
				// get the build pipeline bundle annotation
				buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

			})

			ginkgo.AfterAll(func() {
				if !ginkgo.CurrentSpecReport().Failed() {
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
				}

				// Delete new branches created by PaC
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(secretLookupGitSourceRepoOneName, firstPacBranchName)
				if err != nil {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
				}
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(secretLookupGitSourceRepoTwoName, secondPacBranchName)
				if err != nil {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
				}

				// Delete the created first component base branch
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(secretLookupGitSourceRepoOneName, firstComponentBaseBranchName)
				if err != nil {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
				}
				// Delete the created second component base branch
				err = f.AsKubeAdmin.CommonController.Github.DeleteRef(secretLookupGitSourceRepoTwoName, secondComponentBaseBranchName)
				if err != nil {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("Reference does not exist"))
				}

				// Delete created webhook from GitHub
				err = build.CleanupWebhooks(f, secretLookupGitSourceRepoTwoName)
				if err != nil {
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("404 Not Found"))
				}

			})
			ginkgo.When("two secrets are created", func() {
				ginkgo.BeforeAll(func() {
					// create the correct build secret for second component
					secretName1 := "build-secret-1"
					secretAnnotations := map[string]string{
						"appstudio.redhat.com/scm.repository": noAppOrgName + "/" + secretLookupGitSourceRepoTwoName,
					}
					token := os.Getenv("GITHUB_TOKEN")
					err = createBuildSecret(f, secretName1, secretAnnotations, token)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					// create incorrect build-secret for the first component
					secretName2 := "build-secret-2"
					dummyToken := "ghp_dummy_secret"
					err = createBuildSecret(f, secretName2, nil, dummyToken)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					// component names and pac branch names
					firstComponentName = fmt.Sprintf("%s-%s", "component-one", util.GenerateRandomString(4))
					secondComponentName = fmt.Sprintf("%s-%s", "component-two", util.GenerateRandomString(4))
					firstPacBranchName = constants.PaCPullRequestBranchPrefix + firstComponentName
					secondPacBranchName = constants.PaCPullRequestBranchPrefix + secondComponentName
				})

				ginkgo.It("creates first component", func() {
					componentObj1 := appservice.ComponentSpec{
						ComponentName: firstComponentName,
						Application:   applicationName,
						Source: appservice.ComponentSource{
							ComponentSourceUnion: appservice.ComponentSourceUnion{
								GitSource: &appservice.GitSource{
									URL:           secretLookupComponentOneGitSourceURL,
									Revision:      firstComponentBaseBranchName,
									DockerfileURL: constants.DockerFilePath,
								},
							},
						},
					}
					_, err := f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj1, testNamespace, "", "", applicationName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})
				ginkgo.It("creates second component", func() {
					componentObj2 := appservice.ComponentSpec{
						ComponentName: secondComponentName,
						Application:   applicationName,
						Source: appservice.ComponentSource{
							ComponentSourceUnion: appservice.ComponentSourceUnion{
								GitSource: &appservice.GitSource{
									URL:           secretLookupComponentTwoGitSourceURL,
									Revision:      secondComponentBaseBranchName,
									DockerfileURL: constants.DockerFilePath,
								},
							},
						},
					}
					_, err := f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj2, testNamespace, "", "", applicationName, false, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				ginkgo.It("check first component annotation has errors", func() {
					buildStatus := &controllers.BuildStatus{}
					gomega.Eventually(func() (bool, error) {
						component, err := f.AsKubeAdmin.HasController.GetComponent(firstComponentName, testNamespace)
						if err != nil {
							return false, err
						} else if component == nil {
							return false, fmt.Errorf("got component as nil after getting component %s in namespace %s", firstComponentName, testNamespace)
						}
						buildStatusAnnotationValue := component.Annotations[controllers.BuildStatusAnnotationName]
						ginkgo.GinkgoWriter.Printf(buildStatusAnnotationValueLoggingFormat, buildStatusAnnotationValue)
						statusBytes := []byte(buildStatusAnnotationValue)
						err = json.Unmarshal(statusBytes, buildStatus)
						if err != nil {
							return false, err
						}
						return buildStatus.PaC != nil && buildStatus.PaC.State == "error" && strings.Contains(buildStatus.PaC.ErrMessage, "Access token is unrecognizable by GitHub"), nil
					}, time.Minute*2, 5*time.Second).Should(gomega.BeTrue(), "failed while checking build status for component %q is correct", firstComponentName)
				})

				ginkgo.It(fmt.Sprintf("triggered PipelineRun is for component %s", secondComponentName), func() {
					timeout = time.Minute * 5
					gomega.Eventually(func() error {
						pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(secondComponentName, applicationName, testNamespace, "")
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, secondComponentName)
							return err
						}
						if !pr.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", secondComponentName, testNamespace))
				})

				ginkgo.It("check only one pipelinerun should be triggered", func() {
					// Waiting for 2 minute to see if only one pipelinerun is triggered
					gomega.Consistently(func() (bool, error) {
						pipelineRuns, err := f.AsKubeAdmin.HasController.GetAllPipelineRunsForApplication(applicationName, testNamespace)
						if err != nil {
							return false, err
						}
						if len(pipelineRuns.Items) != 1 {
							return false, fmt.Errorf("plr count in the namespace %s is not one, got pipelineruns %v", testNamespace, pipelineRuns.Items)
						}
						return true, nil
					}, time.Minute*2, constants.PipelineRunPollingInterval).Should(gomega.BeTrue(), "timeout while checking if any more pipelinerun is triggered")
				})
				ginkgo.It("when second component is deleted, pac pr branch should not exist in the repo", ginkgo.Pending, func() {
					timeout = time.Second * 60
					interval = time.Second * 1
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteComponent(secondComponentName, testNamespace, true)).To(gomega.Succeed())
					gomega.Eventually(func() bool {
						exists, err := f.AsKubeAdmin.CommonController.Github.ExistsRef(secretLookupGitSourceRepoTwoName, secondPacBranchName)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						return exists
					}, timeout, interval).Should(gomega.BeFalse(), fmt.Sprintf("timed out when waiting for the branch %s to be deleted from %s repository", secondPacBranchName, secretLookupGitSourceRepoTwoName))
				})
			})
		})
		ginkgo.Describe("test build annotations", ginkgo.Label("annotations"), ginkgo.Ordered, func() {
			var testNamespace, componentName, applicationName string
			var componentObj appservice.ComponentSpec
			var component *appservice.Component
			var buildPipelineAnnotation map[string]string
			invalidAnnotation := "foo"

			ginkgo.BeforeAll(func() {
				f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				testNamespace = f.UserNamespace

				applicationName = fmt.Sprintf("build-suite-test-application-%s", util.GenerateRandomString(4))
				_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				componentName = fmt.Sprintf("%s-%s", "test-annotations", util.GenerateRandomString(6))

				// get the build pipeline bundle annotation
				buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

			})

			ginkgo.AfterAll(func() {
				if !ginkgo.CurrentSpecReport().Failed() {
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
				}

			})

			ginkgo.When("component is created with invalid build request annotations", func() {

				invalidBuildAnnotation := map[string]string{
					controllers.BuildRequestAnnotationName: invalidAnnotation,
				}

				ginkgo.BeforeAll(func() {
					componentObj = appservice.ComponentSpec{
						ComponentName: componentName,
						Application:   applicationName,
						Source: appservice.ComponentSource{
							ComponentSourceUnion: appservice.ComponentSourceUnion{
								GitSource: &appservice.GitSource{
									URL:           annotationsTestGitHubURL,
									Revision:      annotationsTestRevision,
									DockerfileURL: constants.DockerFilePath,
								},
							},
						},
					}

					component, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, false, utils.MergeMaps(invalidBuildAnnotation, buildPipelineAnnotation))
					gomega.Expect(component).ToNot(gomega.BeNil())
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				ginkgo.It("handles invalid request annotation", func() {

					expectedInvalidAnnotationMessage := fmt.Sprintf("unexpected build request: %s", invalidAnnotation)

					// Waiting for 1 minute to see if any pipelinerun is triggered
					gomega.Consistently(func() bool {
						_, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
						gomega.Expect(err).To(gomega.HaveOccurred())
						return strings.Contains(err.Error(), "no pipelinerun found")
					}, time.Minute*1, constants.PipelineRunPollingInterval).Should(gomega.BeTrue(), "timeout while checking if any pipelinerun is triggered")

					buildStatus := &controllers.BuildStatus{}
					gomega.Eventually(func() error {
						component, err = f.AsKubeAdmin.HasController.GetComponent(componentName, testNamespace)
						if err != nil {
							return err
						} else if component == nil {
							return fmt.Errorf("got component as nil after getting component %s in namespace %s", componentName, testNamespace)
						}
						buildStatusAnnotationValue := component.Annotations[controllers.BuildStatusAnnotationName]
						ginkgo.GinkgoWriter.Printf(buildStatusAnnotationValueLoggingFormat, buildStatusAnnotationValue)
						statusBytes := []byte(buildStatusAnnotationValue)
						err = json.Unmarshal(statusBytes, buildStatus)
						if err != nil {
							return err
						}
						if !strings.Contains(buildStatus.Message, expectedInvalidAnnotationMessage) {
							return fmt.Errorf("build status message is not as expected, got: %q, expected: %q", buildStatus.Message, expectedInvalidAnnotationMessage)
						}
						return nil
					}, time.Minute*2, 2*time.Second).Should(gomega.Succeed(), "failed while checking build status message for component %q is correct after setting invalid annotations", componentName)
				})
			})
		})

		ginkgo.Describe("Creating component with container image source", ginkgo.Ordered, func() {
			var applicationName, componentName, testNamespace string
			var timeout time.Duration
			var buildPipelineAnnotation map[string]string

			ginkgo.BeforeAll(func() {
				applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))
				f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				testNamespace = f.UserNamespace

				_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				componentName = fmt.Sprintf("build-suite-test-component-image-source-%s", util.GenerateRandomString(6))
				outputContainerImage := ""
				timeout = time.Second * 10
				// Create a component with containerImageSource being defined
				component := appservice.ComponentSpec{
					ComponentName:  fmt.Sprintf("build-suite-test-component-image-source-%s", util.GenerateRandomString(6)),
					ContainerImage: containerImageSource,
				}
				_, err = f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(component, testNamespace, outputContainerImage, "", applicationName, true, buildPipelineAnnotation)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// get the build pipeline bundle annotation
				buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)
			})

			ginkgo.AfterAll(func() {
				if !ginkgo.CurrentSpecReport().Failed() {
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(testNamespace, time.Minute*5)).To(gomega.Succeed())
				}
			})

			ginkgo.It("should not trigger a PipelineRun", func() {
				gomega.Consistently(func() bool {
					_, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
					gomega.Expect(err).To(gomega.HaveOccurred())
					return strings.Contains(err.Error(), "no pipelinerun found")
				}, timeout, constants.PipelineRunPollingInterval).Should(gomega.BeTrue(), fmt.Sprintf("expected no PipelineRun to be triggered for the component %s in %s namespace", componentName, testNamespace))
			})
		})

		ginkgo.DescribeTableSubtree("test of component update with renovate", ginkgo.Ordered, ginkgo.Label("renovate", "multi-component"), func(gitProvider git.GitProvider, gitPrefix string) {
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
			var parentPostPacMergeDigest string
			var parentImageNameWithNoDigest string
			const distributionRepository = "quay.io/redhat-appstudio-qe/release-repository"
			quayOrg := utils.GetEnv("DEFAULT_QUAY_ORG", "")
			var parentRepository, childRepository string

			var managedNamespace string
			var buildPipelineAnnotation map[string]string

			var gitClient git.Client
			var componentDependenciesChildRepository string

			ginkgo.BeforeAll(func() {
				f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				testNamespace = f.UserNamespace

				applicationName = fmt.Sprintf("build-suite-component-update-%s", util.GenerateRandomString(4))
				_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
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
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					// Fork the child repo
					err = gitClient.ForkRepository(componentDependenciesChildRepoName, ChildComponentDef.repoName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				case git.GitLabProvider:
					gitClient = git.NewGitlabClient(f.AsKubeAdmin.CommonController.Gitlab)

					parentRepository = fmt.Sprintf("%s/%s", gitlabOrg, ParentComponentDef.repoName)
					ParentComponentDef.gitRepo = fmt.Sprintf(gitlabUrlFormat, parentRepository)

					childRepository = fmt.Sprintf("%s/%s", gitlabOrg, ChildComponentDef.repoName)
					ChildComponentDef.gitRepo = fmt.Sprintf(gitlabUrlFormat, childRepository)

					// Fork the parent repo
					err = gitClient.ForkRepository(fmt.Sprintf("%s/%s", gitlabOrg, componentDependenciesParentRepoName), parentRepository)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					// Fork the child repo
					err = gitClient.ForkRepository(fmt.Sprintf("%s/%s", gitlabOrg, componentDependenciesChildRepoName), childRepository)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					componentDependenciesChildRepository = childRepository
				}
				ParentComponentDef.componentName = fmt.Sprintf("%s-multi-component-parent-%s", gitPrefix, branchString)
				ChildComponentDef.componentName = fmt.Sprintf("%s-multi-component-child-%s", gitPrefix, branchString)
				ParentComponentDef.pacBranchName = constants.PaCPullRequestBranchPrefix + ParentComponentDef.componentName
				ChildComponentDef.pacBranchName = constants.PaCPullRequestBranchPrefix + ChildComponentDef.componentName

				err = gitClient.CreateBranch(parentRepository, ParentComponentDef.baseBranch, ParentComponentDef.baseRevision, ParentComponentDef.componentBranch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = gitClient.CreateBranch(childRepository, ChildComponentDef.baseBranch, ChildComponentDef.baseRevision, ChildComponentDef.componentBranch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// Also setup a release namespace so we can test nudging of distribution repository images
				managedNamespace = testNamespace + "-managed"
				_, err = f.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

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
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				ginkgo.GinkgoWriter.Printf("ReleaseAdmissionPlan data: %s", string(rawData))
				managedServiceAccount, err := f.AsKubeAdmin.CommonController.CreateServiceAccount("release-service-account", managedNamespace, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				_, err = f.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission("demo", managedNamespace, "", f.UserNamespace, "demo", "release-service-account", []string{applicationName}, true, &tektonutils.PipelineRef{
					Resolver: "git",
					Params: []tektonutils.Param{
						{Name: "url", Value: constants.RELEASE_CATALOG_DEFAULT_URL},
						{Name: "revision", Value: constants.RELEASE_CATALOG_DEFAULT_REVISION},
						{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
					}}, &runtime.RawExtension{Raw: rawData})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				// get the build pipeline bundle annotation
				buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)

				if gitProvider == git.GitLabProvider {
					gitlabToken := utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, "")
					gomega.Expect(gitlabToken).ShouldNot(gomega.BeEmpty())

					secretAnnotations := map[string]string{}

					err = build.CreateGitlabBuildSecret(f, "pipelines-as-code-secret", secretAnnotations, gitlabToken)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.AfterAll(func() {
				if !ginkgo.CurrentSpecReport().Failed() {
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteComponent(ParentComponentDef.componentName, testNamespace, true)).To(gomega.Succeed())
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteComponent(ChildComponentDef.componentName, testNamespace, true)).To(gomega.Succeed())
					gomega.Expect(f.AsKubeAdmin.HasController.DeleteApplication(applicationName, testNamespace, false)).To(gomega.Succeed())
					gomega.Expect(gitClient.DeleteRepositoryIfExists(parentRepository)).To(gomega.Succeed())
					gomega.Expect(gitClient.DeleteRepositoryIfExists(childRepository)).To(gomega.Succeed())
				}
				gomega.Expect(f.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.When("components are created in same namespace", func() {

				ginkgo.It("creates component with nudges", func() {
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
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					}
				})
				// Initial pipeline run, we need this so we have an initial image that we can then update
				ginkgo.It(fmt.Sprintf("triggers a PipelineRun for parent component %s", ParentComponentDef.componentName), func() {
					timeout = time.Minute * 5

					gomega.Eventually(func() error {
						pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(ParentComponentDef.componentName, applicationName, testNamespace, "")
						if err != nil {
							ginkgo.GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, ParentComponentDef.componentName)
							return err
						}
						if !pr.HasStarted() {
							return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", ParentComponentDef.componentName, testNamespace))
				})
				ginkgo.It(fmt.Sprintf("the PipelineRun should eventually finish successfully for parent component %s", ParentComponentDef.componentName), func() {
					gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(ParentComponentDef.component, "", "", "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Always: true, Retries: 2}, nil)).To(gomega.Succeed())
					pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(ParentComponentDef.component.GetName(), ParentComponentDef.component.Spec.Application, ParentComponentDef.component.GetNamespace(), "")
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					for _, result := range pr.Status.Results {
						if result.Name == "IMAGE_DIGEST" {
							parentFirstDigest = result.Value.StringVal
						}
					}
					gomega.Expect(parentFirstDigest).ShouldNot(gomega.BeEmpty(), fmt.Sprintf("pipelinerun status results: %v", pr.Status.Results))
				})

				ginkgo.It(fmt.Sprintf("the PipelineRun should eventually finish successfully for child component %s", ChildComponentDef.componentName), func() {
					gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(ChildComponentDef.component, "", "", "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Always: true, Retries: 2}, nil)).To(gomega.Succeed())
				})

				ginkgo.It(fmt.Sprintf("should lead to a PaC PR creation for child component %s", ChildComponentDef.componentName), func() {
					timeout = time.Second * 300
					interval := time.Second * 5

					gomega.Eventually(func() bool {
						prs, err := gitClient.ListPullRequests(childRepository)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						for _, pr := range prs {
							if pr.SourceBranch == ChildComponentDef.pacBranchName {
								prNumber = pr.Number
								return true
							}
						}
						return false
					}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for PaC PR (branch name '%s') to be created in %s repository", ChildComponentDef.pacBranchName, ChildComponentDef.repoName))
				})

				ginkgo.It(fmt.Sprintf("Merging the PaC PR should be successful for child component %s", ChildComponentDef.componentName), func() {
					gomega.Eventually(func() error {
						mergeResult, err = gitClient.MergePullRequest(childRepository, prNumber)
						return err
					}, time.Minute).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, ChildComponentDef.repoName))

					mergeResultSha = mergeResult.MergeCommitSHA
					ginkgo.GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)
				})
				// Now we have an initial image we create a dockerfile in the child that references this new image
				// This is the file that will be updated by the nudge
				ginkgo.It("create dockerfile and yaml manifest that references build and distribution repositories", func() {

					imageRepoName, err = f.AsKubeAdmin.ImageController.GetImageName(testNamespace, ParentComponentDef.componentName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to read image repo for component %s", ParentComponentDef.componentName)
					gomega.Expect(imageRepoName).ShouldNot(gomega.BeEmpty(), "image repo name is empty")

					parentImageNameWithNoDigest = "quay.io/" + quayOrg + "/" + imageRepoName
					_, err = gitClient.CreateFile(childRepository, "Dockerfile.tmp", "FROM "+parentImageNameWithNoDigest+"@"+parentFirstDigest+"\nRUN echo hello\n", ChildComponentDef.pacBranchName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					_, err = gitClient.CreateFile(childRepository, "manifest.yaml", "image: "+distributionRepository+"@"+parentFirstDigest, ChildComponentDef.pacBranchName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					_, err = gitClient.CreatePullRequest(childRepository, "updated to build repo image", "update to build repo image", ChildComponentDef.pacBranchName, ChildComponentDef.componentBranch)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					prs, err := gitClient.ListPullRequests(childRepository)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					prno := -1
					for _, pr := range prs {
						if pr.SourceBranch == ChildComponentDef.pacBranchName {
							prno = pr.Number
						}
					}
					gomega.Expect(prno).ShouldNot(gomega.Equal(-1))

					// GitLab merge fails if the pipeline run has not finished
					gomega.Eventually(func() error {
						_, err = gitClient.MergePullRequest(childRepository, prno)
						return err
					}, 10*time.Minute, time.Minute).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("unable to merge PR #%d in %s", prno, ChildComponentDef.repoName))

				})
				// This actually happens immediately, but we only need the PR number now
				ginkgo.It(fmt.Sprintf("should lead to a PaC PR creation for parent component %s", ParentComponentDef.componentName), func() {
					timeout = time.Second * 300
					interval := time.Second * 5

					gomega.Eventually(func() bool {
						prs, err := gitClient.ListPullRequests(parentRepository)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						for _, pr := range prs {
							if pr.SourceBranch == ParentComponentDef.pacBranchName {
								prNumber = pr.Number
								return true
							}
						}
						return false
					}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for PaC PR (branch name '%s') to be created in %s repository", ParentComponentDef.pacBranchName, ParentComponentDef.repoName))
				})
				ginkgo.It(fmt.Sprintf("Merging the PaC PR should be successful for parent component %s", ParentComponentDef.componentName), func() {
					gomega.Eventually(func() error {
						mergeResult, err = gitClient.MergePullRequest(parentRepository, prNumber)
						return err
					}, time.Minute).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error when merging PaC pull request #%d in repo %s", prNumber, ParentComponentDef.repoName))

					mergeResultSha = mergeResult.MergeCommitSHA
					ginkgo.GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)
				})
				// Now the PR is merged this will kick off another build. The result of this build is what we want to update in dockerfile we created
				ginkgo.It(fmt.Sprintf("PR merge triggers PAC PipelineRun for parent component %s", ParentComponentDef.componentName), func() {
					timeout = time.Minute * 5

					gomega.Eventually(func() error {
						pipelineRun, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(ParentComponentDef.componentName, applicationName, testNamespace, mergeResultSha)
						if err != nil {
							ginkgo.GinkgoWriter.Printf("Push PipelineRun has not been created yet for the component %s/%s\n", testNamespace, ParentComponentDef.componentName)
							return err
						}
						if !pipelineRun.HasStarted() {
							return fmt.Errorf("push pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
						}
						return nil
					}, timeout, constants.PipelineRunPollingInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, ParentComponentDef.componentName))
				})
				// Wait for this PR to be done and store the digest, we will need it to verify that the nudge was correct
				ginkgo.It(fmt.Sprintf("PAC PipelineRun for parent component %s is successful", ParentComponentDef.componentName), func() {
					pr := &pipeline.PipelineRun{}
					gomega.Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(ParentComponentDef.component, "", mergeResultSha, "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Always: true, Retries: 2}, pr)).To(gomega.Succeed())

					for _, result := range pr.Status.Results {
						if result.Name == "IMAGE_DIGEST" {
							parentPostPacMergeDigest = result.Value.StringVal
						}
					}
					gomega.Expect(parentPostPacMergeDigest).ShouldNot(gomega.BeEmpty())
				})
				ginkgo.It(fmt.Sprintf("should lead to a nudge PR creation for child component %s", ChildComponentDef.componentName), func() {
					timeout = time.Minute * 20
					interval := time.Second * 10

					gomega.Eventually(func() bool {
						prs, err := gitClient.ListPullRequests(componentDependenciesChildRepository)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						for _, pr := range prs {
							if strings.Contains(pr.SourceBranch, ParentComponentDef.componentName) {
								prNumber = pr.Number
								return true
							}
						}
						return false
					}, timeout, interval).Should(gomega.BeTrue(), fmt.Sprintf("timed out when waiting for component nudge PR to be created in %s repository", targetChildRepoName))
				})
				ginkgo.It(fmt.Sprintf("merging the PR should be successful for child component %s", ChildComponentDef.componentName), func() {
					gomega.Eventually(func() error {
						mergeResult, err = gitClient.MergePullRequest(componentDependenciesChildRepository, prNumber)
						return err
					}, time.Minute).ShouldNot(gomega.HaveOccurred(), fmt.Sprintf("error when merging nudge pull request #%d in repo %s", prNumber, targetChildRepoName))

					mergeResultSha = mergeResult.MergeCommitSHA
					ginkgo.GinkgoWriter.Printf("merged result sha: %s for PR #%d\n", mergeResultSha, prNumber)

				})
				// Now the nudge has been merged we verify the dockerfile is what we expected
				ginkgo.It("Verify the nudge updated the contents", func() {

					ginkgo.GinkgoWriter.Printf("Verifying Dockerfile.tmp updated to sha %s", parentPostPacMergeDigest)
					file, err := gitClient.GetFile(childRepository, "Dockerfile.tmp", ChildComponentDef.componentBranch)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					ginkgo.GinkgoWriter.Printf("content: %s\n", file.Content)
					gomega.Expect(file.Content).Should(gomega.Equal("FROM quay.io/" + quayOrg + "/" + imageRepoName + "@" + parentPostPacMergeDigest + "\nRUN echo hello\n"))

					file, err = gitClient.GetFile(childRepository, "manifest.yaml", ChildComponentDef.componentBranch)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(file.Content).Should(gomega.Equal("image: " + distributionRepository + "@" + parentPostPacMergeDigest))

				})
			})
		},
			ginkgo.Entry("github", git.GitHubProvider, "gh"),
			ginkgo.Entry("gitlab", git.GitLabProvider, "gl"),
		)
	})
})

func setupGitProvider(f *framework.Framework, gitProvider git.GitProvider) (git.Client, string, string) {
	switch gitProvider {
	case git.GitHubProvider:
		gitClient := git.NewGitHubClient(f.AsKubeAdmin.CommonController.Github)
		targetRepoName := helloWorldComponentGitSourceRepoName + "-" + util.GenerateRandomString(6)
		targetRepoURL := fmt.Sprintf(githubUrlFormat, githubOrg, targetRepoName)
		err := gitClient.ForkRepository(helloWorldComponentGitSourceRepoName, targetRepoName)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		return gitClient, targetRepoURL, targetRepoName
	case git.GitLabProvider:
		gitClient := git.NewGitlabClient(f.AsKubeAdmin.CommonController.Gitlab)

		gitlabToken := utils.GetEnv(constants.GITLAB_BOT_TOKEN_ENV, "")
		gomega.Expect(gitlabToken).ShouldNot(gomega.BeEmpty())

		secretAnnotations := map[string]string{}

		err := build.CreateGitlabBuildSecret(f, "pipelines-as-code-secret", secretAnnotations, gitlabToken)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		targetGitLabProjectID := fmt.Sprintf("%s/%s", gitlabOrg, helloWorldComponentGitSourceRepoName+"-"+util.GenerateRandomString(6))
		err = gitClient.ForkRepository(helloWorldComponentGitLabProjectID, targetGitLabProjectID)
		targetGitlabURL := fmt.Sprintf(gitlabUrlFormat, targetGitLabProjectID)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		return gitClient, targetGitlabURL, targetGitLabProjectID
	}
	return nil, "", ""
}

func createBuildSecret(f *framework.Framework, secretName string, annotations map[string]string, token string) error {
	buildSecret := v1.Secret{}
	buildSecret.Name = secretName
	buildSecret.Labels = map[string]string{
		"appstudio.redhat.com/credentials": "scm",
		"appstudio.redhat.com/scm.host":    "github.com",
	}
	if annotations != nil {
		buildSecret.Annotations = annotations
	}
	buildSecret.Type = "kubernetes.io/basic-auth"
	buildSecret.StringData = map[string]string{
		"password": token,
	}
	_, err := f.AsKubeAdmin.CommonController.CreateSecret(f.UserNamespace, &buildSecret)
	if err != nil {
		return fmt.Errorf("error creating build secret: %v", err)
	}
	return nil
}
