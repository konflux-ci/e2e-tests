package tekton

import (
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeController struct {
	C common.SuiteController
	T SuiteController
}

// This is a demo task to create test image and task signing
func kanikoTaskRun(image string) *v1beta1.TaskRun {
	imageInfo := strings.Split(image, "/")
	namespace := imageInfo[1]
	imageName := imageInfo[2]

	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("kaniko-taskrun-%s", imageName),
			Namespace:    namespace,
		},
		Spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{
				{
					Name: "IMAGE",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: image,
					},
				},
			},
			TaskRef: &v1beta1.TaskRef{
				Kind:   v1beta1.NamespacedTaskKind,
				Name:   "kaniko-chains",
				Bundle: "quay.io/jstuart/appstudio-tasks:latest-1",
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name:     "source",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}
}

// image is full url to the image
// Example image: image-registry.openshift-image-registry.svc:5000/tekton-chains/kaniko-chains
func verifyTaskRun(image, taskName string) *v1beta1.TaskRun {
	imageInfo := strings.Split(image, "/")
	namespace := imageInfo[1]
	imageName := imageInfo[2]

	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-%s", taskName, imageName),
			Namespace:    namespace,
		},
		Spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{
				{
					Name: "IMAGE",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: image,
					},
				},
				{
					Name: "PUBLIC_KEY",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "k8s://tekton-chains/signing-secrets",
					},
				},
			},
			TaskRef: &v1beta1.TaskRef{
				Kind:   v1beta1.NamespacedTaskKind,
				Name:   taskName,
				Bundle: "quay.io/jstuart/appstudio-tasks:latest-1",
			},
		},
	}
}

func (k KubeController) RunKanikoTask(image, ns string, taskTimeout int) (*v1beta1.TaskRun, error) {
	tr := kanikoTaskRun(image)
	return k.createAndWait(tr, ns, taskTimeout)
}

func (k KubeController) WatchTaskPod(tr, ns string, taskTimeout int) error {
	trUpdated, _ := k.T.GetTaskRun(tr, ns)
	pod, _ := k.C.GetPod(ns, trUpdated.Status.PodName)
	return k.C.WaitForPod(k.C.IsPodSuccessful(pod.Name, ns), time.Duration(taskTimeout)*time.Second)
}

func (k KubeController) RunVerifyTask(taskName, image, ns string, taskTimeout int) (*v1beta1.TaskRun, error) {
	tr := verifyTaskRun(image, taskName)
	return k.createAndWait(tr, ns, taskTimeout)
}

func (k KubeController) createAndWait(tr *v1beta1.TaskRun, ns string, taskTimeout int) (*v1beta1.TaskRun, error) {
	taskRun, _ := k.T.CreateTaskRun(tr, ns)
	return taskRun, k.C.WaitForPod(k.T.CheckTaskPodExists(taskRun.Name, ns), time.Duration(taskTimeout)*time.Second)
}
