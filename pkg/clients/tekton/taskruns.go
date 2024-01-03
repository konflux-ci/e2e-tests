package tekton

import (
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateTaskRunCopy creates a TaskRun that copies one image to a second image repository.
func (t *TektonController) CreateTaskRunCopy(name, namespace, serviceAccountName, srcImageURL, destImageURL string) (*tektonv1.TaskRun, error) {
	taskRun := tektonv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: tektonv1.TaskRunSpec{
			ServiceAccountName: serviceAccountName,
			TaskRef: &tektonv1.TaskRef{
				Name: "skopeo-copy",
				Kind: tektonv1.ClusterTaskRefKind,
			},
			Params: []tektonv1.Param{
				{
					Name: "srcImageURL",
					Value: tektonv1.ParamValue{
						StringVal: srcImageURL,
						Type:      tektonv1.ParamTypeString,
					},
				},
				{
					Name: "destImageURL",
					Value: tektonv1.ParamValue{
						StringVal: destImageURL,
						Type:      tektonv1.ParamTypeString,
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
			Workspaces: []tektonv1.WorkspaceBinding{
				{
					Name:     "images-url",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}

	err := t.KubeRest().Create(context.Background(), &taskRun)
	if err != nil {
		return nil, err
	}
	return &taskRun, nil
}

// GetTaskRun returns the requested TaskRun object.
func (t *TektonController) GetTaskRun(name, namespace string) (*tektonv1.TaskRun, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	taskRun := tektonv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := t.KubeRest().Get(context.Background(), namespacedName, &taskRun)
	if err != nil {
		return nil, err
	}
	return &taskRun, nil
}

// GetTaskRunLogs returns logs of a specified taskRun.
func (t *TektonController) GetTaskRunLogs(pipelineRunName, pipelineTaskName, namespace string) (map[string]string, error) {
	tektonClient := t.PipelineClient().TektonV1beta1().PipelineRuns(namespace)
	pipelineRun, err := tektonClient.Get(context.Background(), pipelineRunName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	podName := ""
	for _, childStatusReference := range pipelineRun.Status.ChildReferences {
		if childStatusReference.PipelineTaskName == pipelineTaskName {
			taskRun := &tektonv1.TaskRun{}
			taskRunKey := types.NamespacedName{Namespace: pipelineRun.Namespace, Name: childStatusReference.Name}
			if err := t.KubeRest().Get(context.Background(), taskRunKey, taskRun); err != nil {
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
	pod, err := podClient.Get(context.Background(), podName, metav1.GetOptions{})
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

func (t *TektonController) GetTaskRunFromPipelineRun(c crclient.Client, pr *tektonv1.PipelineRun, pipelineTaskName string) (*tektonv1.TaskRun, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName != pipelineTaskName {
			continue
		}

		taskRun := &tektonv1.TaskRun{}
		taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
		if err := c.Get(context.Background(), taskRunKey, taskRun); err != nil {
			return nil, err
		}
		return taskRun, nil
	}

	return nil, fmt.Errorf("task %q not found in PipelineRun %q/%q", pipelineTaskName, pr.Namespace, pr.Name)
}

func (t *TektonController) GetTaskRunResult(c crclient.Client, pr *tektonv1.PipelineRun, pipelineTaskName string, result string) (string, error) {
	taskRun, err := t.GetTaskRunFromPipelineRun(c, pr, pipelineTaskName)
	if err != nil {
		return "", err
	}

	for _, trResult := range taskRun.Status.Results {
		if trResult.Name == result {
			// for some reason the result might contain \n suffix
			return strings.TrimSuffix(trResult.Value.StringVal, "\n"), nil
		}
	}
	return "", fmt.Errorf(
		"result %q not found in TaskRuns of PipelineRun %s/%s", result, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

// GetTaskRunStatus returns the status of a specified taskRun.
func (t *TektonController) GetTaskRunStatus(c crclient.Client, pr *tektonv1.PipelineRun, pipelineTaskName string) (*tektonv1.PipelineRunTaskRunStatus, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName == pipelineTaskName {
			taskRun := &tektonv1.TaskRun{}
			taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
			if err := c.Get(context.Background(), taskRunKey, taskRun); err != nil {
				return nil, err
			}
			return &tektonv1.PipelineRunTaskRunStatus{PipelineTaskName: chr.PipelineTaskName, Status: &taskRun.Status}, nil
		}
	}
	return nil, fmt.Errorf(
		"TaskRun status for pipeline task name %q not found in the status of PipelineRun %s/%s", pipelineTaskName, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

// DeleteAllTaskRunsInASpecificNamespace removes all TaskRuns from a given repository. Useful when creating a lot of resources and wanting to remove all of them.
func (t *TektonController) DeleteAllTaskRunsInASpecificNamespace(namespace string) error {
	return t.KubeRest().DeleteAllOf(context.Background(), &tektonv1.TaskRun{}, crclient.InNamespace(namespace))
}
