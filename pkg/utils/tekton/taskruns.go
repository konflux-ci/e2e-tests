package tekton

import (
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateTaskRunCopy creates a TaskRun that copies one image to a second image repository.
func (t *TektonController) CreateTaskRunCopy(name, namespace, serviceAccountName, srcImageURL, destImageURL string) (*v1beta1.TaskRun, error) {
	taskRun := v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.TaskRunSpec{
			ServiceAccountName: serviceAccountName,
			TaskRef: &v1beta1.TaskRef{
				Name: "skopeo-copy",
			},
			Params: []v1beta1.Param{
				{
					Name: "srcImageURL",
					Value: v1beta1.ParamValue{
						StringVal: srcImageURL,
						Type:      v1beta1.ParamTypeString,
					},
				},
				{
					Name: "destImageURL",
					Value: v1beta1.ParamValue{
						StringVal: destImageURL,
						Type:      v1beta1.ParamTypeString,
					},
				},
			},
			// workaround to avoid the error "container has runAsNonRoot and image will run as root"
			PodTemplate: &pod.Template{
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: pointer.Bool(true),
					RunAsUser:    pointer.Int64(65532),
				},
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name:     "images-url",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}

	err := t.KubeRest().Create(context.TODO(), &taskRun)
	if err != nil {
		return nil, err
	}
	return &taskRun, nil
}

// GetTaskRun returns the requested TaskRun object.
func (t *TektonController) GetTaskRun(name, namespace string) (*v1beta1.TaskRun, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	taskRun := v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := t.KubeRest().Get(context.TODO(), namespacedName, &taskRun)
	if err != nil {
		return nil, err
	}
	return &taskRun, nil
}

// GetTaskRunLogs returns logs of a specified taskRun.
func (t *TektonController) GetTaskRunLogs(pipelineRunName, pipelineTaskName, namespace string) (map[string]string, error) {
	tektonClient := t.PipelineClient().TektonV1beta1().PipelineRuns(namespace)
	pipelineRun, err := tektonClient.Get(context.TODO(), pipelineRunName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	podName := ""
	for _, childStatusReference := range pipelineRun.Status.ChildReferences {
		if childStatusReference.PipelineTaskName == pipelineTaskName {
			taskRun := &v1beta1.TaskRun{}
			taskRunKey := types.NamespacedName{Namespace: pipelineRun.Namespace, Name: childStatusReference.Name}
			if err := t.KubeRest().Get(context.TODO(), taskRunKey, taskRun); err != nil {
				return nil, err
			}
			podName = taskRun.Status.PodName
			break
		}
	}
	if podName == "" {
		return nil, fmt.Errorf("task with %s name doesn't exist in %s pipelinerun", pipelineTaskName, pipelineRunName)
	}

	podClient := t.KubeInterface().CoreV1().Pods(namespace)
	pod, err := podClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	logs := make(map[string]string)
	for _, container := range pod.Spec.Containers {
		containerName := container.Name
		if containerLogs, err := t.fetchContainerLog(podName, containerName, namespace); err == nil {
			logs[containerName] = containerLogs
		} else {
			logs[containerName] = "failed to get logs"
		}
	}
	return logs, nil
}

// GetTaskRunResult returns the result of a specified taskRun.
func (t *TektonController) GetTaskRunResult(c crclient.Client, pr *v1beta1.PipelineRun, pipelineTaskName string, result string) (string, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName != pipelineTaskName {
			continue
		}

		taskRun := &v1beta1.TaskRun{}
		taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
		if err := c.Get(context.TODO(), taskRunKey, taskRun); err != nil {
			return "", err
		}

		for _, trResult := range taskRun.Status.TaskRunResults {
			if trResult.Name == result {
				// for some reason the result might contain \n suffix
				return strings.TrimSuffix(trResult.Value.StringVal, "\n"), nil
			}
		}
	}
	return "", fmt.Errorf(
		"result %q not found in TaskRuns of PipelineRun %s/%s", result, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

// GetTaskRunStatus returns the status of a specified taskRun.
func (t *TektonController) GetTaskRunStatus(c crclient.Client, pr *v1beta1.PipelineRun, pipelineTaskName string) (*v1beta1.PipelineRunTaskRunStatus, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName == pipelineTaskName {
			taskRun := &v1beta1.TaskRun{}
			taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
			if err := c.Get(context.TODO(), taskRunKey, taskRun); err != nil {
				return nil, err
			}
			return &v1beta1.PipelineRunTaskRunStatus{PipelineTaskName: chr.PipelineTaskName, Status: &taskRun.Status}, nil
		}
	}
	return nil, fmt.Errorf(
		"TaskRun status for pipeline task name %q not found in the status of PipelineRun %s/%s", pipelineTaskName, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

// DeleteAllTaskRunsInASpecificNamespace removes all TaskRuns from a given repository. Useful when creating a lot of resources and wanting to remove all of them.
func (t *TektonController) DeleteAllTaskRunsInASpecificNamespace(namespace string) error {
	return t.KubeRest().DeleteAllOf(context.TODO(), &v1beta1.TaskRun{}, crclient.InNamespace(namespace))
}
