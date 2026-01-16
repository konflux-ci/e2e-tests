package build

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
		"github.com/konflux-ci/e2e-tests/pkg/utils/build"
		ginkgo "github.com/onsi/ginkgo/v2"
		gomega "github.com/onsi/gomega"
		"github.com/tektoncd/cli/pkg/bundle"
		pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
		corev1 "k8s.io/api/core/v1"
		metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
		"k8s.io/apimachinery/pkg/runtime"
	)
	
	/* to run locally on a kind cluster
	1. set environment variables with examples
	  - E2E_APPLICATIONS_NAMESPACE=konflux-tasks
	  - TKN_BUNDLE_REPO=quay.io/my-user-org/tkn-bundle:latest
	2. AFTER the namespace is created, create a docker secret and patch the sa
	  - kubectl create secret generic docker-config --from-file=.dockerconfigjson="$HOME/.docker/config.json" --type=kubernetes.io/dockerconfigjson --dry-run=client -o yaml | kubectl apply -f
	  - kubectl patch serviceaccount appstudio-pipeline -p '{"imagePullSecrets": [{"name": "docker-config"}], "secrets": [{"name": "docker-config"}]}'
	*/
	
	// Re-enable the test when https://issues.redhat.com/browse/KONFLUX-9777 is fixed
	var _ = framework.TknBundleSuiteDescribe("tkn bundle task", ginkgo.Label("build-templates"), ginkgo.Pending, func(){
	
		defer ginkgo.GinkgoRecover()

	var namespace string
	var fwk *framework.Framework
	taskName := "tkn-bundle"
	pathInRepo := fmt.Sprintf("task/%s/0.2/%s.yaml", taskName, taskName)
	pvcName := "source-pvc"
	var pvcAccessMode corev1.PersistentVolumeAccessMode = "ReadWriteOnce"
	var baseTaskRun *pipeline.TaskRun
	qeBundleRepo := fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), taskName)

	var gitRevision, gitURL, bundleImg string

	ginkgo.AfterEach(framework.ReportFailure(&fwk))

	ginkgo.BeforeAll(func() {
		var err error
		fwk, err = framework.NewFramework(utils.GetGeneratedNamespace("konflux-task-runner"))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		namespace = fwk.UserNamespace

		if os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV) == "" {
			gomega.Expect(fwk.AsKubeAdmin.CommonController.CreateQuayRegistrySecret(namespace)).To(gomega.Succeed())
		}

		bundleImg = utils.GetEnv("TKN_BUNDLE_REPO", qeBundleRepo)

		// resolve the gitURL and gitRevision
		gitURL, gitRevision, err = build.ResolveGitDetails(constants.TASK_REPO_URL_ENV, constants.TASK_REPO_REVISION_ENV)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// if pvc does not exist create it
		if _, err := fwk.AsKubeAdmin.TektonController.GetPVC(pvcName, namespace); err != nil {
			_, err = fwk.AsKubeAdmin.TektonController.CreatePVCInAccessMode(pvcName, namespace, pvcAccessMode)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}
		// use a pod to copy test data to the pvc
		testData, err := setupTestData(pvcName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		pod, err := fwk.AsKubeAdmin.CommonController.CreatePod(testData, namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// wait for setup pod. make sure it's successful
		err = fwk.AsKubeAdmin.CommonController.WaitForPod(fwk.AsKubeAdmin.CommonController.IsPodSuccessful(pod.Name, namespace), 300)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})

	ginkgo.BeforeEach(func() {
		resolverRef := pipeline.ResolverRef{
			Resolver: "git",
			Params: []pipeline.Param{
				{Name: "url", Value: *pipeline.NewStructuredValues(gitURL)},
				{Name: "revision", Value: *pipeline.NewStructuredValues(gitRevision)},
				{Name: "pathInRepo", Value: *pipeline.NewStructuredValues(pathInRepo)},
			},
		}
		// get a new taskRun on each Entry
		baseTaskRun = taskRunTemplate(taskName, pvcName, bundleImg, resolverRef)
		baseTaskRun.Spec.Params = []pipeline.Param{
			{
				Name: "URL",
				Value: pipeline.ParamValue{
					Type:      "string",
					StringVal: gitURL,
				},
			},
			{
				Name: "REVISION",
				Value: pipeline.ParamValue{
					Type:      "string",
					StringVal: gitRevision,
				},
			},
			{
				Name: "IMAGE",
				Value: pipeline.ParamValue{
					Type:      "string",
					StringVal: qeBundleRepo,
				},
			},
		}
	})

	ginkgo.DescribeTable("creates Tekton bundles with different params",
		func(params map[string]string, expectedOutput, notExpectedOutput []string, expectedHomeVar, stepImage string) {
			matchOutput := func(output string, expectedOutput []string) {
				for _, expected := range expectedOutput {
					gomega.Expect(output).To(gomega.ContainSubstring(expected))
				}
			}

			notMatchOutput := func(output string, notExpectedOutput []string) {
				for _, notExpected := range notExpectedOutput {
					gomega.Expect(output).NotTo(gomega.ContainSubstring(notExpected))
				}
			}

			for key, val := range params {
				baseTaskRun.Spec.Params = append(baseTaskRun.Spec.Params, pipeline.Param{
					Name: key,
					Value: pipeline.ParamValue{
						Type:      "string",
						StringVal: val,
					},
				})
			}
			tr, err := fwk.AsKubeAdmin.TektonController.RunTaskAndWait(baseTaskRun, namespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// check for a success of the taskRun
			gomega.Eventually(func() bool {
				status, err := fwk.AsKubeAdmin.TektonController.CheckTaskRunSucceeded(tr.Name, namespace)()
				return err == nil && status
			}, time.Minute*2, 2*time.Second).Should(gomega.BeTrue(), fmt.Sprintf("taskRun %q failed", tr.Name))

			// verify taskRun results
			imgUrl, err := fwk.AsKubeAdmin.TektonController.GetResultFromTaskRun(tr, "IMAGE_URL")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(imgUrl).To(gomega.Equal(bundleImg))

			imgDigest, err := fwk.AsKubeAdmin.TektonController.GetResultFromTaskRun(tr, "IMAGE_DIGEST")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(imgDigest).To(gomega.MatchRegexp(`^sha256:[a-fA-F0-9]{64}$`))

			// verify taskRun log output
			podLogs, err := fwk.AsKubeAdmin.CommonController.GetPodLogsByName(tr.Status.PodName, namespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			podLog := fmt.Sprintf("pod-%s-step-build.log", tr.Status.PodName)
			matchOutput(string(podLogs[podLog]), expectedOutput)
			notMatchOutput(string(podLogs[podLog]), notExpectedOutput)

			// verify environment variables
			envVar, err := fwk.AsKubeAdmin.TektonController.GetEnvVariable(tr, "HOME")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(envVar).To(gomega.Equal(expectedHomeVar))

			// verify the step images
			visitor := func(apiVersion, kind, name string, obj runtime.Object, raw []byte) {
				task := &pipeline.Task{}
				err := json.Unmarshal(raw, task)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				for _, step := range task.Spec.Steps {
					gomega.Expect(step.Image).To(gomega.Equal(stepImage))
				}
			}
			bundle := fmt.Sprintf("%s@%s", imgUrl, imgDigest)
			ginkgo.GinkgoWriter.Printf("Fetching bundle image: %s\n", bundle)

			gomega.Eventually(func() error {
				return fetchImage(bundle, visitor)
			}, time.Minute*2, 2*time.Second).Should(gomega.Succeed(), "failed to fetch image %q", bundle)

		},
		ginkgo.Entry("when context points to a file", map[string]string{"CONTEXT": "task2.yaml"},
			[]string{
				"\t- Added Task: task2 to image",
			},
			[]string{
				"\t- Added Task: task1 to image",
				"\t- Added Task: task3 to image",
			},
			"/tekton/home",
			"ubuntu",
		),
		ginkgo.Entry("creates Tekton bundles from specific context", map[string]string{"CONTEXT": "sub"}, []string{
			"\t- Added Task: task3 to image",
		},
			[]string{
				"\t- Added Task: task1 to image",
				"\t- Added Task: task2 to image",
			},
			"/tekton/home",
			"ubuntu",
		),
		ginkgo.Entry("when context is the root directory", map[string]string{}, []string{
			"\t- Added Task: task1 to image",
			"\t- Added Task: task2 to image",
			"\t- Added Task: task3 to image",
		},
			[]string{},
			"/tekton/home",
			"ubuntu",
		),
		ginkgo.Entry("creates Tekton bundles when context points to a file and a directory", map[string]string{"CONTEXT": "task2.yaml,sub"}, []string{
			"\t- Added Task: task2 to image",
			"\t- Added Task: task3 to image",
		},
			[]string{
				"\t- Added Task: task1 to image",
			},
			"/tekton/home",
			"ubuntu",
		),
		ginkgo.Entry("creates Tekton bundles when using negation", map[string]string{"CONTEXT": "!sub"}, []string{
			"\t- Added Task: task1 to image",
			"\t- Added Task: task2 to image",
		},
			[]string{
				"\t- Added Task: task3 to image",
			},
			"/tekton/home",
			"ubuntu",
		),
		ginkgo.Entry("allows overriding HOME environment variable", map[string]string{"CONTEXT": ".", "HOME": "/tekton/summer-home"}, []string{
			"\t- Added Task: task1 to image",
			"\t- Added Task: task2 to image",
			"\t- Added Task: task3 to image",
		},
			[]string{},
			"/tekton/summer-home",
			"ubuntu",
		),
		ginkgo.Entry("allows overriding STEP image", map[string]string{"STEPS_IMAGE": "quay.io/enterprise-contract/contract:latest"}, []string{
			"\t- Added Task: task1 to image",
			"\t- Added Task: task2 to image",
			"\t- Added Task: task3 to image",
		},
			[]string{},
			"/tekton/home",
			"quay.io/enterprise-contract/contract:latest",
		),
	)
})

// fetch the image
func fetchImage(image string, visitor func(version, kind, name string, element runtime.Object, raw []byte)) error {
	img, err := crane.Pull(image, crane.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return fmt.Errorf("failed to pull the image: %v", err)
	}
	err = bundle.List(img, visitor)
	if err != nil {
		return fmt.Errorf("failed to list objects in the image: %v", err)
	}
	return nil
}

// sets the task files on a pvc for use by the task
func setupTestData(pvcName string) (*corev1.Pod, error) {
	// setup test data
	testTasks, err := testData([]string{"task1", "task2", "task3"})
	if err != nil {
		return nil, err
	}

	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "setup-pod-",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: "Never",
			Containers: []corev1.Container{
				{
					Command: []string{
						"bash",
						"-c",
						"mkdir -p source/source/sub; echo $TASK1_JSON > source/source/task1.yaml; echo $TASK2_JSON > source/source/task2.yaml; echo $TASK3_JSON > source/source/sub/task3.yaml",
					},
					Image: "registry.access.redhat.com/ubi9/ubi-minimal:latest",
					Name:  "setup-pod",
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/source",
							Name:      "source",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "TASK1_JSON",
							Value: testTasks["task1"],
						},
						{
							Name:  "TASK2_JSON",
							Value: testTasks["task2"],
						},
						{
							Name:  "TASK3_JSON",
							Value: testTasks["task3"],
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "source",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}, nil
}

// the test tasks
func testData(tasks []string) (map[string]string, error) {
	apiVersion := "tekton.dev/v1"
	allTasks := make(map[string]string)
	for idx, task := range tasks {
		taskJson, err := serializeTask(&pipeline.Task{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Task",
				APIVersion: apiVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: task,
			},
			Spec: pipeline.TaskSpec{
				Steps: []pipeline.Step{
					{
						Name:  fmt.Sprintf("test%d-step", idx),
						Image: "ubuntu",
					},
				},
			},
		})
		if err != nil {
			return nil, err
		}
		allTasks[task] = taskJson
	}
	return allTasks, nil
}

// the taskRun that runs tkn-bundle
func taskRunTemplate(taskName, pvcName, bundleImg string, resolverRef pipeline.ResolverRef) *pipeline.TaskRun {
	return &pipeline.TaskRun{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Task",
			APIVersion: "tekton.dev/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", taskName),
		},
		Spec: pipeline.TaskRunSpec{
			ServiceAccountName: constants.DefaultPipelineServiceAccount,
			TaskRef: &pipeline.TaskRef{
				ResolverRef: resolverRef,
			},
			Params: pipeline.Params{
				{
					Name: "IMAGE",
					Value: pipeline.ParamValue{
						Type:      "string",
						StringVal: bundleImg,
					},
				},
			},
			Workspaces: []pipeline.WorkspaceBinding{
				{
					Name: "source",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			},
		},
	}
}

func serializeTask(task *pipeline.Task) (string, error) {
	taskJson, err := json.Marshal(task)
	if err != nil {
		return "", err
	}
	return string(taskJson), nil
}
