package build

import (
	"context"
	"fmt"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	v1 "k8s.io/api/core/v1"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Ec2ArmTag           = "multi-platform-e2e-arm64"
	HostConfig          = "host-config"
	ControllerNamespace = "multi-platform-controller"
	SecretName          = "awskeys"
)

var (
	multiPlatformProjectGitUrl   = utils.GetEnv("MULTI_PLATFORM_TEST_REPO_URL", "https://github.com/devfile-samples/devfile-sample-go-basic")
	multiPlatformProjectRevision = utils.GetEnv("MULTI_PLATFORM_TEST_REPO_REVISION", "c713067b0e65fb3de50d1f7c457eb51c2ab0dbb0")
)

var _ = framework.MultiPlatformBuildSuiteDescribe("Multi Platform Controller E2E tests", Label("multi-platform"), func() {
	var f *framework.Framework
	AfterEach(framework.ReportFailure(&f))
	var err error

	defer GinkgoRecover()

	var testNamespace, applicationName, componentName string
	var component *appservice.Component
	var timeout, interval time.Duration

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(f.AsKubeAdmin.HasController.DeleteComponent(componentName, testNamespace, false)).To(Succeed())
			Expect(f.AsKubeAdmin.HasController.DeleteApplication(applicationName, testNamespace, false)).To(Succeed())
			Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
		} else {
			Expect(f.AsKubeAdmin.CommonController.StoreAllPods(testNamespace)).To(Succeed())
			Expect(f.AsKubeAdmin.TektonController.StoreAllPipelineRuns(testNamespace)).To(Succeed())
		}
	})

	BeforeAll(func() {

		f, err = framework.NewFramework(utils.GetGeneratedNamespace("multi-platform-build"))
		Expect(err).NotTo(HaveOccurred())
		testNamespace = f.UserNamespace
		Expect(testNamespace).NotTo(BeNil(), "failed to create sandbox user namespace")

		Expect(err).ShouldNot(HaveOccurred())

		armInstances, err := getAwsInstances()
		Expect(err).ShouldNot(HaveOccurred())
		hostConfig := &v1.ConfigMap{}
		hostConfig.Name = HostConfig
		hostConfig.Namespace = ControllerNamespace
		hostConfig.Labels = map[string]string{"build.appstudio.redhat.com/multi-platform-config": "hosts"}

		hostConfig.Data = map[string]string{}
		count := 0
		for _, instance := range armInstances {
			hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.address", count)] = instance
			hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.platform", count)] = "linux/arm64"
			hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.user", count)] = "ec2-user"
			hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.secret", count)] = SecretName
			hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.concurrency", count)] = "4"
			count++
		}

		_, err = f.AsKubeAdmin.CommonController.CreateConfigMap(hostConfig, ControllerNamespace)
		Expect(err).ShouldNot(HaveOccurred())

		keys := v1.Secret{}
		keys.Name = SecretName
		keys.Namespace = ControllerNamespace
		keys.Labels = map[string]string{"build.appstudio.redhat.com/multi-platform-secret": "true"}
		keys.StringData = map[string]string{"id_rsa": os.Getenv("MULTI_PLATFORM_AWS_SSH_KEY")}
		_, err = f.AsKubeAdmin.CommonController.CreateSecret(ControllerNamespace, &keys)
		Expect(err).ShouldNot(HaveOccurred())

		trueBool := true
		customBuildahRemotePipeline := os.Getenv(constants.CUSTOM_BUILDAH_REMOTE_PIPELINE_BUILD_BUNDLE_ENV)
		Expect(customBuildahRemotePipeline).ShouldNot(BeEmpty())
		ps := &buildservice.BuildPipelineSelector{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-pipeline-selector",
				Namespace: testNamespace,
			},
			Spec: buildservice.BuildPipelineSelectorSpec{Selectors: []buildservice.PipelineSelector{
				{
					Name:           "custom remote-buildah selector",
					PipelineRef:    *tekton.NewBundleResolverPipelineRef("buildah-remote-pipeline", customBuildahRemotePipeline),
					WhenConditions: buildservice.WhenCondition{DockerfileRequired: &trueBool},
				},
			}},
		}
		Expect(f.AsKubeAdmin.CommonController.KubeRest().Create(context.TODO(), ps)).To(Succeed())

		timeout = time.Minute * 20
		interval = time.Second * 10

		applicationName = fmt.Sprintf("multi-platform-suite-application-%s", util.GenerateRandomString(4))
		app, err := f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(utils.WaitUntil(f.AsKubeAdmin.HasController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
			Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
		)

		componentName = fmt.Sprintf("multi-platform-suite-component-%s", util.GenerateRandomString(4))

		// Create a component with Git Source URL being defined
		componentObj := appservice.ComponentSpec{
			ComponentName: componentName,
			Source: appservice.ComponentSource{
				ComponentSourceUnion: appservice.ComponentSourceUnion{
					GitSource: &appservice.GitSource{
						URL:      multiPlatformProjectGitUrl,
						Revision: multiPlatformProjectRevision,
					},
				},
			},
		}
		component, err = f.AsKubeAdmin.HasController.CreateComponent(componentObj, testNamespace, "", "", applicationName, true, map[string]string{})
		Expect(err).ShouldNot(HaveOccurred())
	})

	When("the Component with multi-platform-build is created", func() {
		It("a PipelineRun is triggered", func() {
			Eventually(func() error {
				pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Printf("PipelineRun has not been created yet for the component %s/%s", testNamespace, componentName)
					return err
				}
				if !pr.HasStarted() {
					return fmt.Errorf("pipelinerun %s/%s hasn't started yet", pr.GetNamespace(), pr.GetName())
				}
				return nil
			}, timeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the PipelineRun to start for the component %s/%s", testNamespace, componentName))
		})

		It("the build-container task from component pipelinerun is buildah-remote", func() {

			Eventually(func() error {
				pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())

				for _, chr := range pr.Status.ChildReferences {
					taskRun := &v1beta1.TaskRun{}
					taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
					err := f.AsKubeAdmin.CommonController.KubeRest().Get(context.TODO(), taskRunKey, taskRun)
					Expect(err).ShouldNot(HaveOccurred())

					prTrStatus := &v1beta1.PipelineRunTaskRunStatus{
						PipelineTaskName: chr.PipelineTaskName,
						Status:           &taskRun.Status,
					}

					if chr.PipelineTaskName == constants.BuildTaskRunName && prTrStatus.Status != nil && prTrStatus.Status.TaskSpec != nil && prTrStatus.Status.TaskSpec.Volumes != nil {
						for _, vol := range prTrStatus.Status.TaskSpec.Volumes {
							if vol.Secret != nil && strings.HasPrefix(vol.Secret.SecretName, "multi-platform-ssh-") {
								return nil
							}
						}
					}
				}
				return fmt.Errorf("couldn't find a matching step buildah-remote in task %s in PipelineRun %s/%s", constants.BuildTaskRunName, testNamespace, pr.GetName())
			}, timeout, interval).Should(Succeed(), "timed out when verifying the buildah-remote image reference in pipelinerun")
		})

		It("that PipelineRun completes successfully", func() {
			Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true})).To(Succeed())
			pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
			Expect(err).ShouldNot(HaveOccurred())
			//now delete it so it can't interfere with later test logic
			Expect(f.AsKubeAdmin.TektonController.DeletePipelineRun(pr.Name, testNamespace)).Should(Succeed())
		})

	})
})

// Get AWS instances that are running
// These are identified by tag
func getAwsInstances() ([]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(EnvCredentialsProvider{}),
		config.WithRegion("us-east-1"))
	if err != nil {
		return nil, err
	}

	// Create an EC2 client
	ec2Client := ec2.NewFromConfig(cfg)
	res, err := ec2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{Filters: []ec2types.Filter{{Name: aws.String("tag:" + Ec2ArmTag), Values: []string{"true"}}}})
	if err != nil {
		return nil, err
	}
	ret := []string{}
	for _, res := range res.Reservations {
		for _, inst := range res.Instances {
			if inst.State.Name != ec2types.InstanceStateNameTerminated {
				ret = append(ret, *inst.PublicDnsName)
			}
		}
	}
	return ret, nil
}

type EnvCredentialsProvider struct {
}

func (r EnvCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return aws.Credentials{AccessKeyID: os.Getenv("MULTI_PLATFORM_AWS_ACCESS_KEY"), SecretAccessKey: os.Getenv("MULTI_PLATFORM_AWS_SECRET_ACCESS_KEY")}, nil
}
