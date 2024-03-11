package build

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"

	"github.com/devfile/library/v2/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Ec2ArmTag              = "multi-platform-e2e-arm64"
	HostConfig             = "host-config"
	ControllerNamespace    = "multi-platform-controller"
	AwsSecretName          = "awskeys"
	SshSecretName          = "sshkeys"
	Ec2User                = "ec2-user"
	AwsRegion              = "us-east-1"
	AwsPlatform            = "linux/arm64"
	DynamicInstanceTag     = "dynamic-test-instance"
	MultiPlatformSecretKey = "build.appstudio.redhat.com/multi-platform-secret"
	MultiPlatformConfigKey = "build.appstudio.redhat.com/multi-platform-config"
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

	Describe("aws host-pool allocation", Label("aws-host-pool"), func() {

		var testNamespace, applicationName, componentName, multiPlatformSecretName, host, userDir string
		var component *appservice.Component
		var timeout, interval time.Duration

		AfterAll(func() {
			// Cleanup aws secet and host-config
			Expect(f.AsKubeAdmin.CommonController.DeleteSecret(ControllerNamespace, AwsSecretName)).To(Succeed())
			Expect(f.AsKubeAdmin.CommonController.DeleteConfigMap(HostConfig, ControllerNamespace, true)).To(Succeed())

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

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("multi-platform-host"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace
			Expect(testNamespace).NotTo(BeNil(), "failed to create sandbox user namespace")

			Expect(err).ShouldNot(HaveOccurred())

			err = createConfigMapForHostPool(f)
			Expect(err).ShouldNot(HaveOccurred())

			err = createSecretForHostPool(f)
			Expect(err).ShouldNot(HaveOccurred())

			err = createBuildPipelineSelector(f, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())

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
						taskRun := &pipeline.TaskRun{}
						taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
						err := f.AsKubeAdmin.CommonController.KubeRest().Get(context.TODO(), taskRunKey, taskRun)
						Expect(err).ShouldNot(HaveOccurred())

						prTrStatus := &pipeline.PipelineRunTaskRunStatus{
							PipelineTaskName: chr.PipelineTaskName,
							Status:           &taskRun.Status,
						}

						if chr.PipelineTaskName == constants.BuildTaskRunName && prTrStatus.Status != nil && prTrStatus.Status.TaskSpec != nil && prTrStatus.Status.TaskSpec.Volumes != nil {
							for _, vol := range prTrStatus.Status.TaskSpec.Volumes {
								if vol.Secret != nil && strings.HasPrefix(vol.Secret.SecretName, "multi-platform-ssh-") {
									multiPlatformSecretName = vol.Secret.SecretName
									return nil
								}
							}
						}
					}
					return fmt.Errorf("couldn't find a matching step buildah-remote in task %s in PipelineRun %s/%s", constants.BuildTaskRunName, testNamespace, pr.GetName())
				}, timeout, interval).Should(Succeed(), "timed out when verifying the buildah-remote image reference in pipelinerun")
			})
			It("The multi platform secret is populated", func() {

				var secret *v1.Secret
				Eventually(func() error {
					secret, err = f.AsKubeAdmin.CommonController.GetSecret(testNamespace, multiPlatformSecretName)
					if err != nil {
						return err
					}
					return nil
				}, timeout, interval).Should(Succeed(), "timed out when verifying the secret is created")

				// Get the host and the user directory so we can verify they are cleaned up at the end of the run
				fullHost, present := secret.Data["host"]
				Expect(present).To(BeTrue())

				userDirTmp, present := secret.Data["user-dir"]
				Expect(present).To(BeTrue())
				userDir = string(userDirTmp)
				hostParts := strings.Split(string(fullHost), "@")
				host = strings.TrimSpace(hostParts[1])
			})

			It("that PipelineRun completes successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true})).To(Succeed())
				pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())
				//now delete it so it can't interfere with later test logic
				Expect(f.AsKubeAdmin.TektonController.DeletePipelineRun(pr.Name, testNamespace)).Should(Succeed())
			})

			It("test that cleanup happened successfully", func() {

				// Parse the private key
				signer, err := ssh.ParsePrivateKey([]byte(os.Getenv("MULTI_PLATFORM_AWS_SSH_KEY")))
				if err != nil {
					log.Fatalf("Unable to parse private key: %v", err)
				}

				Eventually(func() error {
					// SSH configuration using public key authentication
					config := &ssh.ClientConfig{
						User: Ec2User,
						Auth: []ssh.AuthMethod{
							ssh.PublicKeys(signer),
						},
						HostKeyCallback: ssh.InsecureIgnoreHostKey(), // #nosec
					}

					client, err := ssh.Dial("tcp", host+":22", config)
					if err != nil {
						return err
					}
					defer client.Close()

					// Create a new session
					session, err := client.NewSession()
					if err != nil {
						return err
					}
					defer session.Close()

					// Check if the file exists
					if dirExists(session, userDir) {
						return fmt.Errorf("cleanup not successful, user dir still exists")
					}
					return nil
				}, timeout, interval).Should(Succeed(), "timed out when verifying that the remote host was cleaned up correctly")
			})
		})
	})
	Describe("aws dynamic allocation", Label("aws-dynamic"), func() {
		var testNamespace, applicationName, componentName, multiPlatformSecretName, multiPlatformTaskName, instanceId string
		var component *appservice.Component
		var timeout, interval time.Duration

		AfterAll(func() {
			// Cleanup aws&ssh secrets and host-config
			Expect(f.AsKubeAdmin.CommonController.DeleteSecret(ControllerNamespace, AwsSecretName)).To(Succeed())
			Expect(f.AsKubeAdmin.CommonController.DeleteSecret(ControllerNamespace, SshSecretName)).To(Succeed())
			Expect(f.AsKubeAdmin.CommonController.DeleteConfigMap(HostConfig, ControllerNamespace, true)).To(Succeed())

			//Forcefully remove instance incase it is not removed by multi-platform-controller
			err = terminateAwsInstance(instanceId)
			if err != nil {
				GinkgoWriter.Printf("error terminating instance again: %v", err)
			}

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

			f, err = framework.NewFramework(utils.GetGeneratedNamespace("multi-platform-dynamic"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace
			Expect(testNamespace).NotTo(BeNil(), "failed to create sandbox user namespace")
			Expect(err).ShouldNot(HaveOccurred())

			// Restart multi-platform-controller pod to reload configMap again
			podList, err := f.AsKubeAdmin.CommonController.ListAllPods(ControllerNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			for i := range podList.Items {
				podName := podList.Items[i].Name
				if strings.HasPrefix(podName, ControllerNamespace) {
					err := f.AsKubeAdmin.CommonController.RestartPod(podName, ControllerNamespace)
					Expect(err).ShouldNot(HaveOccurred())
				}
			}
			Expect(err).ShouldNot(HaveOccurred())

			// sleep for 10 sec so that multi-platform-controller pod comes up
			time.Sleep(10 * time.Second)

			err = createConfigMapForDynamicInstance(f)
			Expect(err).ShouldNot(HaveOccurred())

			err = createSecretsForDynamicInstance(f)
			Expect(err).ShouldNot(HaveOccurred())

			err = createBuildPipelineSelector(f, testNamespace)
			Expect(err).ShouldNot(HaveOccurred())

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
						taskRun := &pipeline.TaskRun{}
						taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
						err := f.AsKubeAdmin.CommonController.KubeRest().Get(context.TODO(), taskRunKey, taskRun)
						Expect(err).ShouldNot(HaveOccurred())

						prTrStatus := &pipeline.PipelineRunTaskRunStatus{
							PipelineTaskName: chr.PipelineTaskName,
							Status:           &taskRun.Status,
						}

						if chr.PipelineTaskName == constants.BuildTaskRunName && prTrStatus.Status != nil && prTrStatus.Status.TaskSpec != nil && prTrStatus.Status.TaskSpec.Volumes != nil {
							multiPlatformTaskName = chr.Name
							for _, vol := range prTrStatus.Status.TaskSpec.Volumes {
								if vol.Secret != nil && strings.HasPrefix(vol.Secret.SecretName, "multi-platform-ssh-") {
									multiPlatformSecretName = vol.Secret.SecretName
									return nil
								}
							}
						}
					}
					return fmt.Errorf("couldn't find a matching step buildah-remote in task %s in PipelineRun %s/%s", constants.BuildTaskRunName, testNamespace, pr.GetName())
				}, timeout, interval).Should(Succeed(), "timed out when verifying the buildah-remote image reference in pipelinerun")
			})

			It("The multi platform secret is populated", func() {
				Eventually(func() error {
					_, err := f.AsKubeAdmin.CommonController.GetSecret(testNamespace, multiPlatformSecretName)
					if err != nil {
						return err
					}
					return nil
				}, timeout, interval).Should(Succeed(), "timed out when verifying the secret is created")

				// Get the instance id from the task so that we can check during cleanup
				taskRun, err := f.AsKubeDeveloper.TektonController.GetTaskRun(multiPlatformTaskName, testNamespace)
				Expect(err).ShouldNot(HaveOccurred())
				instanceId = taskRun.Annotations["build.appstudio.redhat.com/cloud-instance-id"]
				GinkgoWriter.Printf("INSTANCE ID: %s\n", instanceId)
			})

			It("that PipelineRun completes successfully", func() {
				Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", f.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true})).To(Succeed())
				pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				Expect(err).ShouldNot(HaveOccurred())
				//now delete it so it can't interfere with later test logic
				Expect(f.AsKubeAdmin.TektonController.DeletePipelineRun(pr.Name, testNamespace)).Should(Succeed())
			})

			It("check cleanup happened successfully", func() {
				Eventually(func() error {
					instances, err := getDynamicAwsInstance(DynamicInstanceTag)
					if err != nil {
						return err
					}
					if len(instances) != 0 {
						return fmt.Errorf("instance is not cleaned up properly, current running instances: %v", instances)
					}
					return nil
				}, timeout, interval).Should(Succeed(), "timed out when verifying that the remote host was cleaned up correctly")
			})

		})
	})
})

// Function to check if a file exists on the remote host
func dirExists(session *ssh.Session, dirPath string) bool {
	cmd := fmt.Sprintf("[ -d %s ] && echo 'exists'", dirPath)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running command: %s\n", err)
		return false
	}
	return string(output) == "exists\n"
}

// Get AWS instances that are running
// These are identified by tag
func getHostPoolAwsInstances() ([]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(EnvCredentialsProvider{}),
		config.WithRegion(AwsRegion))
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

func createConfigMapForHostPool(f *framework.Framework) error {
	armInstances, err := getHostPoolAwsInstances()
	if err != nil {
		return fmt.Errorf("error getting aws host pool instances: %v", err)
	}
	hostConfig := &v1.ConfigMap{}
	hostConfig.Name = HostConfig
	hostConfig.Namespace = ControllerNamespace
	hostConfig.Labels = map[string]string{MultiPlatformConfigKey: "hosts"}

	hostConfig.Data = map[string]string{}
	count := 0
	for _, instance := range armInstances {
		hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.address", count)] = instance
		hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.platform", count)] = AwsPlatform
		hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.user", count)] = Ec2User
		hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.secret", count)] = AwsSecretName
		hostConfig.Data[fmt.Sprintf("host.aws-arm64-%d.concurrency", count)] = "4"
		count++
	}

	_, err = f.AsKubeAdmin.CommonController.CreateConfigMap(hostConfig, ControllerNamespace)
	if err != nil {
		return fmt.Errorf("error creating host-pool config map: %v", err)
	}
	return nil
}

func createConfigMapForDynamicInstance(f *framework.Framework) error {
	hostConfig := &v1.ConfigMap{}
	hostConfig.Name = HostConfig
	hostConfig.Namespace = ControllerNamespace
	hostConfig.Labels = map[string]string{MultiPlatformConfigKey: "hosts"}

	hostConfig.Data = map[string]string{}
	hostConfig.Data["dynamic-platforms"] = AwsPlatform
	hostConfig.Data["instance-tag"] = DynamicInstanceTag
	hostConfig.Data["dynamic.linux-arm64.type"] = "aws"
	hostConfig.Data["dynamic.linux-arm64.region"] = AwsRegion
	hostConfig.Data["dynamic.linux-arm64.ami"] = "ami-09d5d0912f52f9514"
	hostConfig.Data["dynamic.linux-arm64.instance-type"] = "t4g.medium"
	hostConfig.Data["dynamic.linux-arm64.key-name"] = "multi-platform-e2e"
	hostConfig.Data["dynamic.linux-arm64.aws-secret"] = AwsSecretName
	hostConfig.Data["dynamic.linux-arm64.ssh-secret"] = SshSecretName
	hostConfig.Data["dynamic.linux-arm64.security-group"] = "launch-wizard-7"
	hostConfig.Data["dynamic.linux-arm64.max-instances"] = "1"

	_, err := f.AsKubeAdmin.CommonController.CreateConfigMap(hostConfig, ControllerNamespace)
	if err != nil {
		return fmt.Errorf("error while creating config map for dynamic instance: %v", err)
	}
	return nil
}

func createSecretForHostPool(f *framework.Framework) error {
	keys := v1.Secret{}
	keys.Name = AwsSecretName
	keys.Namespace = ControllerNamespace
	keys.Labels = map[string]string{MultiPlatformSecretKey: "true"}
	keys.StringData = map[string]string{"id_rsa": os.Getenv("MULTI_PLATFORM_AWS_SSH_KEY")}
	_, err := f.AsKubeAdmin.CommonController.CreateSecret(ControllerNamespace, &keys)
	if err != nil {
		return fmt.Errorf("error while creating host-pool secret: %v", err)
	}
	return nil
}

func createSecretsForDynamicInstance(f *framework.Framework) error {
	awsKeys := v1.Secret{}
	awsKeys.Name = AwsSecretName
	awsKeys.Namespace = ControllerNamespace
	awsKeys.Labels = map[string]string{MultiPlatformSecretKey: "true"}
	awsKeys.StringData = map[string]string{
		"access-key-id":     os.Getenv("MULTI_PLATFORM_AWS_ACCESS_KEY"),
		"secret-access-key": os.Getenv("MULTI_PLATFORM_AWS_SECRET_ACCESS_KEY"),
	}
	_, err := f.AsKubeAdmin.CommonController.CreateSecret(ControllerNamespace, &awsKeys)
	if err != nil {
		return fmt.Errorf("error creating secret with access_key and secret_key: %v", err)
	}

	sshKeys := v1.Secret{}
	sshKeys.Name = SshSecretName
	sshKeys.Namespace = ControllerNamespace
	sshKeys.Labels = map[string]string{MultiPlatformSecretKey: "true"}
	sshKeys.StringData = map[string]string{"id_rsa": os.Getenv("MULTI_PLATFORM_AWS_SSH_KEY")}
	_, err = f.AsKubeAdmin.CommonController.CreateSecret(ControllerNamespace, &sshKeys)
	if err != nil {
		return fmt.Errorf("error creating secret with ssh private key: %v", err)
	}
	return nil
}

func createBuildPipelineSelector(f *framework.Framework, namespace string) error {
	trueBool := true
	customBuildahRemotePipeline := os.Getenv(constants.CUSTOM_BUILDAH_REMOTE_PIPELINE_BUILD_BUNDLE_ENV)
	Expect(customBuildahRemotePipeline).ShouldNot(BeEmpty())
	if customBuildahRemotePipeline == "" {
		return fmt.Errorf("remote build pipeline bundle is empty")
	}
	ps := &buildservice.BuildPipelineSelector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-pipeline-selector",
			Namespace: namespace,
		},
		Spec: buildservice.BuildPipelineSelectorSpec{Selectors: []buildservice.PipelineSelector{
			{
				Name:           "custom remote-buildah selector",
				PipelineRef:    *tekton.NewBundleResolverPipelineRef("buildah-remote-pipeline", customBuildahRemotePipeline),
				WhenConditions: buildservice.WhenCondition{DockerfileRequired: &trueBool},
			},
		}},
	}
	err := f.AsKubeAdmin.CommonController.KubeRest().Create(context.TODO(), ps)
	if err != nil {
		return fmt.Errorf("error creating build pipeline selector: %v", err)
	}
	return nil
}

func terminateAwsInstance(instanceId string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(EnvCredentialsProvider{}),
		config.WithRegion(AwsRegion))
	if err != nil {
		return err
	}
	// Create an EC2 client
	ec2Client := ec2.NewFromConfig(cfg)
	//Terminate Instance
	_, err = ec2Client.TerminateInstances(context.TODO(), &ec2.TerminateInstancesInput{InstanceIds: []string{string(instanceId)}})
	return err
}

func getDynamicAwsInstance(tagName string) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(EnvCredentialsProvider{}),
		config.WithRegion(AwsRegion))
	if err != nil {
		return nil, err
	}

	// Create an EC2 client
	ec2Client := ec2.NewFromConfig(cfg)
	res, err := ec2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{Filters: []ec2types.Filter{{Name: aws.String("tag:" + "multi-platform-instance"), Values: []string{tagName}}}})
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
