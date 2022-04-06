package tekton

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/client"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type KubeController struct {
	Commonctrl common.SuiteController
	Tektonctrl SuiteController
	Namespace  string
}

// Create the struct for kubernetes clients
type SuiteController struct {
	*client.K8sClient
}

// Create controller for Application/Component crud operations
func NewSuiteController() (*SuiteController, error) {
	client, err := client.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client-go %v", err)
	}
	return &SuiteController{
		client,
	}, nil
}

func (s *SuiteController) GetTaskRun(taskName, namespace string) (*v1beta1.TaskRun, error) {
	return s.PipelineClient().TektonV1beta1().TaskRuns(namespace).Get(context.TODO(), taskName, metav1.GetOptions{})
}

func (s *SuiteController) CheckTaskPodExists(taskName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		tr, err := s.GetTaskRun(taskName, namespace)
		if err != nil {
			return false, nil
		}
		if tr.Status.PodName != "" {
			return true, nil
		}
		return false, nil
	}
}

// Create a tekton task and return the task or error
func (s *SuiteController) CreateTask(task *v1beta1.Task, ns string) (*v1beta1.Task, error) {
	return s.PipelineClient().TektonV1beta1().Tasks(ns).Create(context.TODO(), task, metav1.CreateOptions{})
}

// Create a tekton taskRun and return the taskRun or error
func (s *SuiteController) CreateTaskRun(taskRun *v1beta1.TaskRun, ns string) (*v1beta1.TaskRun, error) {
	return s.PipelineClient().TektonV1beta1().TaskRuns(ns).Create(context.TODO(), taskRun, metav1.CreateOptions{})
}

func (s *SuiteController) ListTaskRuns(ns string, labelKey string, labelValue string, selectorLimit int64) (*v1beta1.TaskRunList, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         selectorLimit,
	}
	return s.PipelineClient().TektonV1beta1().TaskRuns(ns).List(context.TODO(), listOptions)
}

func (k KubeController) RunBuildahDemoTask(image string, taskTimeout int) (*v1beta1.TaskRun, error) {
	tr := buildahDemoTaskRun(image)
	return k.createAndWait(tr, taskTimeout)
}

func (k KubeController) WatchTaskPod(tr string, taskTimeout int) error {
	trUpdated, _ := k.Tektonctrl.GetTaskRun(tr, k.Namespace)
	pod, _ := k.Commonctrl.GetPod(k.Namespace, trUpdated.Status.PodName)
	return k.Commonctrl.WaitForPod(k.Commonctrl.IsPodSuccessful(pod.Name, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

func (k KubeController) RunVerifyTask(taskName, image string, taskTimeout int) (*v1beta1.TaskRun, error) {
	tr := verifyTaskRun(image, taskName)
	return k.createAndWait(tr, taskTimeout)
}

func (k KubeController) AwaitAttestationAndSignature(image string, timeout time.Duration) error {
	imageInfo := strings.Split(image, "/")
	namespace := imageInfo[1]
	imageName := imageInfo[2]
	latestImageName := imageName + ":latest"

	tags := &unstructured.UnstructuredList{}
	tags.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "image.openshift.io",
		Kind:    "ImageStreamTag",
		Version: "v1",
	})

	return wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		err = k.Commonctrl.KubeRest().List(context.TODO(), tags, &k8sclient.ListOptions{Namespace: namespace})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			} else {
				return true, err
			}
		}

		// search for the resulting task image's hash from the name in the metadata
		var hash string
		for _, tag := range tags.Items {
			if tag.GetName() == latestImageName {
				if name, found, err := unstructured.NestedString(tag.Object, "image", "metadata", "name"); err == nil && found {
					hash = imageName + ":" + strings.Replace(name, ":", "-", 1)
					break
				}
			}
		}

		// we didn't find the image from the task, should we err instead?
		if hash == "" {
			return false, nil
		}

		// loop again to see if .att and .sig tags are present
		found := 0
		for _, tag := range tags.Items {
			tagName := tag.GetName()
			if strings.HasPrefix(tagName, hash) && (strings.HasSuffix(tagName, ".sig") || strings.HasSuffix(tagName, ".att")) {
				found++
			}
		}

		// we found both
		if found == 2 {
			return true, nil
		}

		return false, nil
	})
}

func (k KubeController) createAndWait(tr *v1beta1.TaskRun, taskTimeout int) (*v1beta1.TaskRun, error) {
	taskRun, _ := k.Tektonctrl.CreateTaskRun(tr, k.Namespace)
	return taskRun, k.Commonctrl.WaitForPod(k.Tektonctrl.CheckTaskPodExists(taskRun.Name, k.Namespace), time.Duration(taskTimeout)*time.Second)
}
