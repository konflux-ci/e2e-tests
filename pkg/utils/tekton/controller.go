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
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
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

type CosignResult struct {
	signatureImageRef   string
	attestationImageRef string
}

func (c CosignResult) IsPresent() bool {
	return c.signatureImageRef != "" && c.attestationImageRef != ""
}

func (c CosignResult) Missing(prefix string) string {
	var ret []string = make([]string, 0, 2)
	if c.signatureImageRef == "" {
		ret = append(ret, prefix+".sig")
	}

	if c.attestationImageRef == "" {
		ret = append(ret, prefix+".att")
	}

	return strings.Join(ret, " and ")
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
	return wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		if _, err := k.FindCosignResultsForImage(image); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}

			return true, err
		}

		return true, nil
	})
}

func (k KubeController) createAndWait(tr *v1beta1.TaskRun, taskTimeout int) (*v1beta1.TaskRun, error) {
	taskRun, _ := k.Tektonctrl.CreateTaskRun(tr, k.Namespace)
	return taskRun, k.Commonctrl.WaitForPod(k.Tektonctrl.CheckTaskPodExists(taskRun.Name, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

// FindCosignResultsForImage looks for .sig and .att image tags in the OpenShift image stream for the provided image reference.
// If none can be found errors.IsNotFound(err) is true, when err is nil CosignResult contains image references for signature and attestation images, otherwise other errors could be returned.
func (k KubeController) FindCosignResultsForImage(imageRef string) (*CosignResult, error) {
	return findCosignResultsForImage(imageRef, k.Commonctrl.KubeRest())
}

func findCosignResultsForImage(imageRef string, client crclient.Client) (*CosignResult, error) {
	imageInfo := strings.Split(imageRef, "/")
	namespace := imageInfo[1]
	imageName := imageInfo[2]
	latestImageName := imageName + ":latest" // tag added by default when building on OpenShift

	tags := &unstructured.UnstructuredList{}
	tags.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "image.openshift.io",
		Kind:    "ImageStreamTag",
		Version: "v1",
	})

	if err := client.List(context.TODO(), tags, &crclient.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}

	// search for the resulting task image's cosignImagePrefix from the name in the metadata
	var cosignImagePrefix string
	if tag := findTagWithName(tags, latestImageName); tag != nil {
		if name, found, err := unstructured.NestedString(tag.Object, "image", "metadata", "name"); err == nil && found {
			cosignImagePrefix = imageName + ":" + strings.Replace(name, ":", "-", 1)
		}
	} else {
		// we didn't find the image from the task, should we err instead?
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    "image.openshift.io",
			Resource: "ImageStreamTag",
		}, latestImageName)
	}

	// loop again to see if .att and .sig tags are present
	results := CosignResult{}

	if signatureTag := findTagWithName(tags, cosignImagePrefix+".sig"); signatureTag != nil {
		results.signatureImageRef = signatureTag.GetName()
	}

	if attestationTag := findTagWithName(tags, cosignImagePrefix+".att"); attestationTag != nil {
		results.attestationImageRef = attestationTag.GetName()
	}

	// we found both
	if results.IsPresent() {
		return &results, nil
	}

	return nil, errors.NewNotFound(schema.GroupResource{
		Group:    "image.openshift.io",
		Resource: "ImageStreamTag",
	}, results.Missing(cosignImagePrefix))
}

func findTagWithName(tags *unstructured.UnstructuredList, name string) *unstructured.Unstructured {
	for _, tag := range tags.Items {
		if tag.GetName() == name {
			return &tag
		}
	}

	return nil
}
