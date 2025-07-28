package build

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

// Re-enable the test when https://issues.redhat.com/browse/KONFLUX-7413 is fixed
var _ = framework.TknBundleSuiteDescribe("tkn bundle task", Label("build-templates"), func() {

	defer GinkgoRecover()

	var namespace string
	var fwk *framework.Framework
	var taskName string = "tkn-bundle"
	var pathInRepo string = fmt.Sprintf("task/%s/0.2/%s.yaml", taskName, taskName)
	var pvcName string = "source-pvc"
	var pvcAccessMode corev1.PersistentVolumeAccessMode = "ReadWriteOnce"
	var baseTaskRun *pipeline.TaskRun
	var qeBundleRepo string = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), taskName)

	var gitRevision, gitURL, bundleImg string

	AfterEach(framework.ReportFailure(&fwk))

	BeforeAll(func() {
		var err error
		fwk, err = framework.NewFramework(utils.GetGeneratedNamespace("konflux-task-runner"))
		Expect(err).NotTo(HaveOccurred())
		namespace = fwk.UserNamespace

		if os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV) == "" {
			Expect(fwk.AsKubeAdmin.CommonController.CreateQuayRegistrySecret(namespace)).To(Succeed())
		}

		bundleImg = utils.GetEnv("TKN_BUNDLE_REPO", qeBundleRepo)

		// resolve the gitURL and gitRevision
		gitURL, gitRevision, err = build.ResolveGitDetails(constants.TASK_REPO_URL_ENV, constants.TASK_REPO_REVISION_ENV)
		Expect(err).NotTo(HaveOccurred())

		// if pvc does not exist create it
		if _, err := fwk.AsKubeAdmin.TektonController.GetPVC(pvcName, namespace); err != nil {
			_, err = fwk.AsKubeAdmin.TektonController.CreatePVCInAccessMode(pvcName, namespace, pvcAccessMode)
			Expect(err).NotTo(HaveOccurred())
		}
		// use a pod to copy test data to the pvc
		testData, err := setupTestData(pvcName)
		Expect(err).NotTo(HaveOccurred())
		pod, err := fwk.AsKubeAdmin.CommonController.CreatePod(testData, namespace)
		Expect(err).NotTo(HaveOccurred())
		// wait for setup pod. make sure it's successful
		err = fwk.AsKubeAdmin.CommonController.WaitForPod(fwk.AsKubeAdmin.CommonController.IsPodSuccessful(pod.Name, namespace), 300)
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
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

	DescribeTable("creates Tekton bundles with different params",
		func(params map[string]string, expectedOutput, notExpectedOutput []string, expectedHomeVar, stepImage string) {
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
			Expect(err).NotTo(HaveOccurred())

			// check for a success of the taskRun
			Eventually(func() bool {
				status, err := fwk.AsKubeAdmin.TektonController.CheckTaskRunSucceeded(tr.Name, namespace)()
				return err == nil && status
			}, time.Minute*2, 2*time.Second).Should(BeTrue(), fmt.Sprintf("taskRun %q failed", tr.Name))

			// verify taskRun results
			imgUrl, err := fwk.AsKubeAdmin.TektonController.GetResultFromTaskRun(tr, "IMAGE_URL")
			Expect(err).NotTo(HaveOccurred())
			Expect(imgUrl).To(Equal(bundleImg))

			imgDigest, err := fwk.AsKubeAdmin.TektonController.GetResultFromTaskRun(tr, "IMAGE_DIGEST")
			Expect(err).NotTo(HaveOccurred())
			Expect(imgDigest).To(MatchRegexp(`^sha256:[a-fA-F0-9]{64}$`))

			// verify taskRun log output
			podLogs, err := fwk.AsKubeAdmin.CommonController.GetPodLogsByName(tr.Status.PodName, namespace)
			Expect(err).NotTo(HaveOccurred())
			podLog := fmt.Sprintf("pod-%s-step-build.log", tr.Status.PodName)
			matchOutput(podLogs[podLog], expectedOutput)
			notMatchOutput(podLogs[podLog], notExpectedOutput)

			// verify environment variables
			envVar, err := fwk.AsKubeAdmin.TektonController.GetEnvVariable(tr, "HOME")
			Expect(err).NotTo(HaveOccurred())
			Expect(envVar).To(Equal(expectedHomeVar))

			// verify the step images
			visitor := func(apiVersion, kind, name string, obj runtime.Object, raw []byte) {
				task := &pipeline.Task{}
				err := json.Unmarshal(raw, task)
				Expect(err).ToNot(HaveOccurred())
				for _, step := range task.Spec.Steps {
					Expect(step.Image).To(Equal(stepImage))
				}
			}
			bundle := fmt.Sprintf("%s@%s", imgUrl, imgDigest)
			GinkgoWriter.Printf("Fetching bundle image: %s\n", bundle)

			Eventually(func() error {
				return fetchImage(bundle, visitor)
			}, time.Minute*2, 2*time.Second).Should(Succeed(), "failed to fetch image %q", bundle)

		},
		Entry("when context points to a file", map[string]string{"CONTEXT": "task2.yaml"},
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
		Entry("creates Tekton bundles from specific context", map[string]string{"CONTEXT": "sub"}, []string{
			"\t- Added Task: task3 to image",
		},
			[]string{
				"\t- Added Task: task1 to image",
				"\t- Added Task: task2 to image",
			},
			"/tekton/home",
			"ubuntu",
		),
		Entry("when context is the root directory", map[string]string{}, []string{
			"\t- Added Task: task1 to image",
			"\t- Added Task: task2 to image",
			"\t- Added Task: task3 to image",
		},
			[]string{},
			"/tekton/home",
			"ubuntu",
		),
		Entry("creates Tekton bundles when context points to a file and a directory", map[string]string{"CONTEXT": "task2.yaml,sub"}, []string{
			"\t- Added Task: task2 to image",
			"\t- Added Task: task3 to image",
		},
			[]string{
				"\t- Added Task: task1 to image",
			},
			"/tekton/home",
			"ubuntu",
		),
		Entry("creates Tekton bundles when using negation", map[string]string{"CONTEXT": "!sub"}, []string{
			"\t- Added Task: task1 to image",
			"\t- Added Task: task2 to image",
		},
			[]string{
				"\t- Added Task: task3 to image",
			},
			"/tekton/home",
			"ubuntu",
		),
		Entry("allows overriding HOME environment variable", map[string]string{"CONTEXT": ".", "HOME": "/tekton/summer-home"}, []string{
			"\t- Added Task: task1 to image",
			"\t- Added Task: task2 to image",
			"\t- Added Task: task3 to image",
		},
			[]string{},
			"/tekton/summer-home",
			"ubuntu",
		),
		Entry("allows overriding STEP image", map[string]string{"STEPS_IMAGE": "quay.io/enterprise-contract/contract:latest"}, []string{
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

// check output that should exist
func matchOutput(logs []byte, expectedOutput []string) {
	for _, out := range expectedOutput {
		Expect(strings.Split(string(logs), "\n")).To(ContainElement(out))
	}
}

// check that output does not exist
func notMatchOutput(logs []byte, expectedOutput []string) {
	for _, out := range expectedOutput {
		Expect(strings.Split(string(logs), "\n")).NotTo(ContainElement(out))
	}
}

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
