package common

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

// GetPod returns the pod object from a given namespace and pod name
func (s *SuiteController) GetPod(namespace, podName string) (*corev1.Pod, error) {
	return s.KubeInterface().CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
}

func (s *SuiteController) IsPodRunning(podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := s.GetPod(namespace, podName)
		if err != nil {
			return false, nil
		}
		switch pod.Status.Phase {
		case corev1.PodRunning:
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, fmt.Errorf("pod %q ran to completion", pod.Name)
		}
		return false, nil
	}
}

// Checks phases of a given pod name in a given namespace
func (s *SuiteController) IsPodSuccessful(podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := s.GetPod(namespace, podName)
		if err != nil {
			return false, nil
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			return true, nil
		case corev1.PodFailed:
			return false, fmt.Errorf("pod %q has failed", pod.Name)
		}
		return false, nil
	}
}

// TaskPodExists checks if a task have a pod
func TaskPodExists(tr *v1beta1.TaskRun) wait.ConditionFunc {
	return func() (bool, error) {
		if tr.Status.PodName != "" {
			return true, nil
		}
		return false, nil
	}
}

// ListPods return a list of pods from a namespace by labels and selection limits
func (s *SuiteController) ListPods(namespace, labelKey, labelValue string, selectionLimit int64) (*corev1.PodList, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         selectionLimit,
	}
	return s.KubeInterface().CoreV1().Pods(namespace).List(context.TODO(), listOptions)
}

// Wait for a pod selector until exists
func (s *SuiteController) WaitForPodSelector(
	fn func(podName, namespace string) wait.ConditionFunc, namespace, labelKey string, labelValue string,
	timeout int, selectionLimit int64) error {
	podList, err := s.ListPods(namespace, labelKey, labelValue, selectionLimit)
	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods in %s with label key %s and label value %s", namespace, labelKey, labelValue)
	}

	for i := range podList.Items {
		if err := utils.WaitUntil(fn(podList.Items[i].Name, namespace), time.Duration(timeout)*time.Second); err != nil {
			return err
		}
	}
	return nil
}

// Stores logs of all pods of the job
func (cs *SuiteController) StorePodLogs(testNamespace, jobName, testLogsDir string) error {
	if err := os.MkdirAll(testLogsDir, os.ModePerm); err != nil {
		return err
	}

	podList, err := cs.KubeInterface().CoreV1().Pods(jobName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing %s pods: %s\n", testNamespace, err.Error())
	}

	GinkgoWriter.Printf("found %d pods in namespace: %s\n", len(podList.Items), testNamespace)

	for _, pod := range podList.Items {
		var containers []corev1.Container
		containers = append(containers, pod.Spec.InitContainers...)
		containers = append(containers, pod.Spec.Containers...)
		for _, c := range containers {
			log, err := utils.GetContainerLogs(cs.KubeInterface(), pod.Name, c.Name, pod.Namespace)
			if err != nil {
				GinkgoWriter.Printf("error getting logs for pod/container %s/%s: %v\n", pod.Name, c.Name, err.Error())
				continue
			}

			filepath := fmt.Sprintf("%s/%s-pod-%s-%s.log", testLogsDir, pod.Namespace, pod.Name, c.Name)
			if err := os.WriteFile(filepath, []byte(log), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
