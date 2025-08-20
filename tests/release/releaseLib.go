package common

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ghub "github.com/google/go-github/v44/github"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func NewFramework(workspace string) *framework.Framework {
	var fw *framework.Framework
	var err error
	stageOptions := utils.Options{
		ToolchainApiUrl: os.Getenv(constants.TOOLCHAIN_API_URL_ENV),
		KeycloakUrl:     os.Getenv(constants.KEYLOAK_URL_ENV),
		OfflineToken:    os.Getenv(constants.OFFLINE_TOKEN_ENV),
	}

	fw, err = framework.NewFrameworkWithTimeout(
		workspace,
		time.Minute*60,
		stageOptions,
	)
	Expect(err).NotTo(HaveOccurred())

	// Create a ticker that ticks every 3 minutes
	ticker := time.NewTicker(3 * time.Minute)
	// Schedule the stop of the ticker after 30 minutes
	time.AfterFunc(60*time.Minute, func() {
		ticker.Stop()
		fmt.Println("Stopped executing every 3 minutes.")
	})
	// Run a goroutine to handle the ticker ticks
	go func() {
		for range ticker.C {
			fw, err = framework.NewFrameworkWithTimeout(
				workspace,
				time.Minute*60,
				stageOptions,
			)
			Expect(err).NotTo(HaveOccurred())
		}
	}()
	return fw
}

func CreateComponent(devFw framework.Framework, devNamespace, appName, compName, gitURL, gitRevision, contextDir, dockerFilePath string, buildPipelineBundle map[string]string) *appservice.Component {
	componentObj := appservice.ComponentSpec{
		ComponentName: compName,
		Application:   appName,
		Source: appservice.ComponentSource{
			ComponentSourceUnion: appservice.ComponentSourceUnion{
				GitSource: &appservice.GitSource{
					URL:           gitURL,
					Revision:      gitRevision,
					Context:       contextDir,
					DockerfileURL: dockerFilePath,
				},
			},
		},
	}
	component, err := devFw.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, devNamespace, "", "", appName, true, buildPipelineBundle)
	Expect(err).NotTo(HaveOccurred())
	return component
}

// CreateComponentWithNewBranch will create a new branch, then create the component based on the new branch
func CreateComponentWithNewBranch(f framework.Framework, testNamespace, applicationName, componentRepoName, componentRepoURL, gitRevision, contextDir, dockerFilePath string, buildPipelineBundle map[string]string) (*appservice.Component, string, string) {
	var buildPipelineAnnotation map[string]string

	componentName := fmt.Sprintf("%s-%s", "test-component-pac", util.GenerateRandomString(6))
	testPacBranchName := constants.PaCPullRequestBranchPrefix + componentName
	componentBaseBranchName := fmt.Sprintf("base-%s", util.GenerateRandomString(6))

	err := f.AsKubeAdmin.CommonController.Github.CreateRef(componentRepoName, "main", gitRevision, componentBaseBranchName)
	Expect(err).ShouldNot(HaveOccurred())

	if buildPipelineBundle["build.appstudio.openshift.io/pipeline"] != string(`{"name": "fbc-builder", "bundle": "latest"}`) {
		// deal with some custom pipeline bundle there
		buildPipelineAnnotation = build.GetBuildPipelineBundleAnnotation(constants.DockerBuild)
	} else {
		buildPipelineAnnotation = constants.DefaultFbcBuilderPipelineBundle
	}

	componentObj := appservice.ComponentSpec{
		ComponentName: componentName,
		Application:   applicationName,
		Source: appservice.ComponentSource{
			ComponentSourceUnion: appservice.ComponentSourceUnion{
				GitSource: &appservice.GitSource{
					URL:           componentRepoURL,
					Revision:      componentBaseBranchName,
					Context:       contextDir,
					DockerfileURL: dockerFilePath,
				},
			},
		},
	}

	testComponent, err := f.AsKubeAdmin.HasController.CreateComponentCheckImageRepository(componentObj, testNamespace, "", "", applicationName, true, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), buildPipelineAnnotation))
	Expect(err).NotTo(HaveOccurred())

	return testComponent, testPacBranchName, componentBaseBranchName
}

func CreatePushSnapshot(devWorkspace, devNamespace, appName, compRepoName, pacBranchName string, pipelineRun *pipeline.PipelineRun, component *appservice.Component) *appservice.Snapshot {
	var (
		prSHA          string
		mergeResult    *ghub.PullRequestMergeResult
		prNumber       int
		err            error
		snapshot       *appservice.Snapshot
		pacPipelineRun *pipeline.PipelineRun
	)

	devFw := NewFramework(devWorkspace)

	Eventually(func() error {
		prs, err := devFw.AsKubeAdmin.CommonController.Github.ListPullRequests(compRepoName)
		Expect(err).NotTo(HaveOccurred())
		for _, pr := range prs {
			if pr.Head.GetRef() == pacBranchName {
				prNumber = pr.GetNumber()
				prSHA = pr.GetHead().GetSHA()
				return nil
			}
		}
		return fmt.Errorf("could not get the expected PaC branch name %s", pacBranchName)
	}, PullRequestCreationTimeout, DefaultPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for init PaC PR (branch %q) to be created against the %q repo", pacBranchName, compRepoName))

	GinkgoWriter.Printf("PacBranchName: %s, prNumber: %d, prSHA : %s\n", pacBranchName, prNumber, prSHA)

	// We don't need the PipelineRun from a PaC 'pull-request' event to finish, so we can delete it
	Eventually(func() error {
		pacPipelineRun, err = devFw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appName, devNamespace, prSHA)
		if err == nil {
			Expect(devFw.AsKubeAdmin.TektonController.DeletePipelineRun(pacPipelineRun.Name, pacPipelineRun.Namespace)).To(Succeed())
			return nil
		}
		return err
	}, PipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for `pull-request` event type PaC PipelineRun to be present in the user namespace %q for component %q with a label pointing to %q", devNamespace, component.GetName(), appName))

	GinkgoWriter.Printf("Pac PipelineRun in user namespace %s for component %s is deleted\n", devNamespace, component.GetName())

	Eventually(func() error {
		mergeResult, err = devFw.AsKubeAdmin.CommonController.Github.MergePullRequest(compRepoName, prNumber)
		return err
	}, MergePRTimeout).ShouldNot(HaveOccurred(), fmt.Sprintf("error when merging PaC pull request: %+v\n", err))

	headSHA := mergeResult.GetSHA()

	GinkgoWriter.Printf("Pac pull request for component %s is merged, headSHA is %s\n", component.GetName(), headSHA)

	Eventually(func() error {
		pipelineRun, err = devFw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appName, devNamespace, headSHA)
		if err != nil {
			GinkgoWriter.Printf("PipelineRun has not been created yet for component %s/%s\n", devNamespace, component.GetName())
			return err
		}
		if !pipelineRun.HasStarted() {
			return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
		}
		return nil
	}, PipelineRunStartedTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for a PipelineRun in namespace %q with label component label %q and application label %q and sha label %q to start", devNamespace, component.GetName(), appName, headSHA))
	GinkgoWriter.Printf("PipelineRun for merging PR %s/%s is created\n", pipelineRun.GetNamespace(), pipelineRun.GetName())

	Eventually(func() error {
		pipelineRun, err = devFw.AsKubeAdmin.HasController.GetComponentPipelineRun(component.GetName(), appName, devNamespace, headSHA)
		if err != nil {
			GinkgoWriter.Printf("PipelineRun can't be found any more for component %s/%s\n", devNamespace, component.GetName())
			return err
		}
		if !pipelineRun.IsDone() {
			return fmt.Errorf("pipelinerun %s/%s has not finished yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
		}
		if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
			return nil
		} else {
			if err = devFw.AsKubeDeveloper.TektonController.StorePipelineRun(component.GetName(), pipelineRun); err != nil {
				GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pipelineRun.GetNamespace(), pipelineRun.GetName(), err.Error())
			}
			prLogs := ""
			if prLogs, err = tekton.GetFailedPipelineRunLogs(devFw.AsKubeAdmin.ReleaseController.KubeRest(),
				devFw.AsKubeAdmin.ReleaseController.KubeInterface(), pipelineRun); err != nil {
				GinkgoWriter.Printf("failed to get PLR logs: %+v", err)
				Expect(err).ShouldNot(HaveOccurred())
				return nil
			}
			GinkgoWriter.Printf("logs: %s", prLogs)
			Expect(prLogs).To(Equal(""), fmt.Sprintf("PipelineRun %s failed", pipelineRun.Name))
			return nil
		}
	}, BuildPipelineRunCompletionTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for a PipelineRun in namespace %q with label component label %q and application label %q and sha label %q to be finished", devNamespace, component.GetName(), appName, headSHA))

	GinkgoWriter.Printf("PipelineRun for merging PR %s/%s is finished\n", pipelineRun.GetNamespace(), pipelineRun.GetName())

	Eventually(func() error {
		snapshot, err = devFw.AsKubeAdmin.IntegrationController.GetSnapshot("", pipelineRun.Name, "", devNamespace)
		return err
	}, SnapshotTimeout, SnapshotPollingInterval).Should(Succeed(), "timed out when trying to check if the Snapshot exists for PipelineRun %s/%s", devNamespace, pipelineRun.GetName())
	GinkgoWriter.Printf("Snapshot %s/%s is finished\n", pipelineRun.GetNamespace(), snapshot.GetName())
	return snapshot
}

// CreateSnapshotWithImageSource creates a snapshot having two images and sources.
func CreateSnapshotWithImageSource(fw *framework.ControllerHub, componentName, applicationName, namespace, containerImage, gitSourceURL, gitSourceRevision, componentName2, containerImage2, gitSourceURL2, gitSourceRevision2 string) (*appstudioApi.Snapshot, error) {
	snapshotComponents := []appstudioApi.SnapshotComponent{
		{
			Name:           componentName,
			ContainerImage: containerImage,
			Source: appstudioApi.ComponentSource{
				appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						Revision: gitSourceRevision,
						URL:      gitSourceURL,
					},
				},
			},
		},
	}

	if componentName2 != "" && containerImage2 != "" {
		newSnapshotComponent := appstudioApi.SnapshotComponent{
			Name:           componentName2,
			ContainerImage: containerImage2,
			Source: appstudioApi.ComponentSource{
				appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						Revision: gitSourceRevision2,
						URL:      gitSourceURL2,
					},
				},
			},
		}
		snapshotComponents = append(snapshotComponents, newSnapshotComponent)
	}

	snapshotName := "snapshot-sample-" + util.GenerateRandomString(4)

	return fw.IntegrationController.CreateSnapshotWithComponents(snapshotName, componentName, applicationName, namespace, snapshotComponents)
}

func CheckReleaseStatus(releaseCR *releaseApi.Release) error {
	GinkgoWriter.Println("ReleaseCR: %s", releaseCR.Name)
	conditions := releaseCR.Status.Conditions
	GinkgoWriter.Println("Length of Release CR conditions: %d", len(conditions))
	if len(conditions) > 0 {
		for _, c := range conditions {
			GinkgoWriter.Println("Type of Release CR condition: %s", c.Type)
			if c.Type == "Released" {
				GinkgoWriter.Println("Status of Release CR condition: %s", c.Status)
				if c.Status == "True" {
					GinkgoWriter.Println("Release CR is released")
					return nil
				} else if c.Status == "False" && c.Reason == "Progressing" {
					return fmt.Errorf("Release %s/%s is in progressing", releaseCR.GetNamespace(), releaseCR.GetName())
				} else {
					GinkgoWriter.Println("Release CR failed/skipped")
					Expect(string(c.Status)).To(Equal("True"), fmt.Sprintf("Release %s failed/skipped", releaseCR.Name))
					return nil
				}
			}
		}
	}
	return nil
}

// CreateOpaqueSecret creates a k8s Secret in a workspace if it doesn't exist
// and updates it if a Secret with the same name exists. It populates the
// Secret data fields based on the mapping of fields to environment variables
// containing the base64 encoded field data.
func CreateOpaqueSecret(
	fw *framework.Framework,
	namespace, secretName string,
	fieldEnvMap map[string]string,
) {
	secretData := make(map[string][]byte)

	for field, envVar := range fieldEnvMap {
		envValue := os.Getenv(envVar)
		Expect(envValue).ToNot(BeEmpty())

		decodedValue, err := base64.StdEncoding.DecodeString(envValue)
		Expect(err).ToNot(HaveOccurred())

		secretData[field] = decodedValue
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	_, err := fw.AsKubeAdmin.CommonController.GetSecret(namespace, secretName)
	if errors.IsNotFound(err) {
		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(namespace, secret)
		Expect(err).ToNot(HaveOccurred())
		return
	}
	Expect(err).ToNot(HaveOccurred())

	_, err = fw.AsKubeAdmin.CommonController.UpdateSecret(namespace, secret)
	Expect(err).ToNot(HaveOccurred())
}
