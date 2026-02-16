package build

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/build-service/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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

	Describe("test build secret lookup", Label("github", "pac-build", "secret-lookup"), Ordered, func() {
		var testNamespace, applicationName, firstComponentBaseBranchName, secondComponentBaseBranchName, firstComponentName, secondComponentName, firstPacBranchName, secondPacBranchName string
		var buildPipelineAnnotation map[string]string
		BeforeAll(func() {
			if os.Getenv(constants.SKIP_PAC_TESTS_ENV) == "true" {
				Skip("Skipping this test due to configuration issue with Spray proxy")
			}
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("build-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			applicationName = fmt.Sprintf("build-secret-lookup-%s", util.GenerateRandomString(4))
			_, err = f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Update the default github org
			f.AsKubeAdmin.CommonController.Github.UpdateGithubOrg(noAppOrgName)

			firstComponentBaseBranchName = fmt.Sprintf("component-one-base-%s", util.GenerateRandomString(6))
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(secretLookupGitSourceRepoOneName, secretLookupDefaultBranchOne, secretLookupGitRevisionOne, firstComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			secondComponentBaseBranchName = fmt.Sprintf("component-two-base-%s", util.GenerateRandomString(6))
			err = f.AsKubeAdmin.CommonController.Github.CreateRef(secretLookupGitSourceRepoTwoName, secretLookupDefaultBranchTwo, secretLookupGitRevisionTwo, secondComponentBaseBranchName)
			Expect(err).ShouldNot(HaveOccurred())

			// use custom bundle if env defined
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

			// Delete new branches created by PaC
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(secretLookupGitSourceRepoOneName, firstPacBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(secretLookupGitSourceRepoTwoName, secondPacBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}

			// Delete the created first component base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(secretLookupGitSourceRepoOneName, firstComponentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}
			// Delete the created second component base branch
			err = f.AsKubeAdmin.CommonController.Github.DeleteRef(secretLookupGitSourceRepoTwoName, secondComponentBaseBranchName)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
			}

		// Delete created webhook from GitHub
		err = build.CleanupWebhooks(f, secretLookupGitSourceRepoTwoName)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("404 Not Found"))
		}

		})
		When("two secrets are created", func() {
			BeforeAll(func() {
				// create the correct build secret for second component
				secretName1 := "build-secret-1"
				secretAnnotations := map[string]string{
					"appstudio.redhat.com/scm.repository": noAppOrgName + "/" + secretLookupGitSourceRepoTwoName,
				}
				token := os.Getenv("GITHUB_TOKEN")
				err = createBuildSecret(f, secretName1, secretAnnotations, token)
				Expect(err).ShouldNot(HaveOccurred())

				// create incorrect build-secret for the first component
				secretName2 := "build-secret-2"
				dummyToken := "ghp_dummy_secret"
				err = createBuildSecret(f, secretName2, nil, dummyToken)
				Expect(err).ShouldNot(HaveOccurred())

				// component names and pac branch names
				firstComponentName = fmt.Sprintf("%s-%s", "component-one", util.GenerateRandomString(4))
				secondComponentName = fmt.Sprintf("%s-%s", "component-two", util.GenerateRandomString(4))
				firstPacBranchName = constants.PaCPullRequestBranchPrefix + firstComponentName
				secondPacBranchName = constants.PaCPullRequestBranchPrefix + secondComponentName
			})

			It("creates first component", func() {
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
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("creates second component", func() {
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
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("check first component annotation has errors", func() {
				buildStatus := &controllers.BuildStatus{}
				Eventually(func() (bool, error) {
					component, err := f.AsKubeAdmin.HasController.GetComponent(firstComponentName, testNamespace)
					if err != nil {
						return false, err
					} else if component == nil {
						return false, fmt.Errorf("got component as nil after getting component %s in namespace %s", firstComponentName, testNamespace)
					}
					buildStatusAnnotationValue := component.Annotations[controllers.BuildStatusAnnotationName]
					GinkgoWriter.Printf(buildStatusAnnotationValueLoggingFormat, buildStatusAnnotationValue)
					statusBytes := []byte(buildStatusAnnotationValue)
					err = json.Unmarshal(statusBytes, buildStatus)
					if err != nil {
						return false, err
					}
					return buildStatus.PaC != nil && buildStatus.PaC.State == "error" && strings.Contains(buildStatus.PaC.ErrMessage, "Access token is unrecognizable by GitHub"), nil
				}, time.Minute*2, 5*time.Second).Should(BeTrue(), "failed while checking build status for component %q is correct", firstComponentName)
			})

			It(fmt.Sprintf("triggered PipelineRun is for component %s", secondComponentName), func() {
				timeout := time.Minute * 5
				Eventually(func() error {
					pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(secondComponentName, applicationName, testNamespace, "")
					if err != nil {
						GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s\n", testNamespace, secondComponentName)
						return err
					}
					if !pr.HasStarted() {
						return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
					}
					return nil
				}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", secondComponentName, testNamespace))
			})

			It("check only one pipelinerun should be triggered", func() {
				// Waiting for 2 minute to see if only one pipelinerun is triggered
				Consistently(func() (bool, error) {
					pipelineRuns, err := f.AsKubeAdmin.HasController.GetAllPipelineRunsForApplication(applicationName, testNamespace)
					if err != nil {
						return false, err
					}
					if len(pipelineRuns.Items) != 1 {
						return false, fmt.Errorf("plr count in the namespace %s is not one, got pipelineruns %v", testNamespace, pipelineRuns.Items)
					}
					return true, nil
				}, time.Minute*2, constants.PipelineRunPollingInterval).Should(BeTrue(), "timeout while checking if any more pipelinerun is triggered")
			})
			It("when second component is deleted, pac pr branch should not exist in the repo", Pending, func() {
				timeout := time.Second * 60
				interval := time.Second * 1
				Expect(f.AsKubeAdmin.HasController.DeleteComponent(secondComponentName, testNamespace, true)).To(Succeed())
				Eventually(func() bool {
					exists, err := f.AsKubeAdmin.CommonController.Github.ExistsRef(secretLookupGitSourceRepoTwoName, secondPacBranchName)
					Expect(err).ShouldNot(HaveOccurred())
					return exists
				}, timeout, interval).Should(BeFalse(), fmt.Sprintf("timed out when waiting for the branch %s to be deleted from %s repository", secondPacBranchName, secretLookupGitSourceRepoTwoName))
			})
		})
	})
})

