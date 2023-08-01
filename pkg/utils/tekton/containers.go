package tekton

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
)

func (s *SuiteController) fetchContainerLog(podName, containerName, namespace string) (string, error) {
	podClient := s.KubeInterface().CoreV1().Pods(namespace)
	req := podClient.GetLogs(podName, &corev1.PodLogOptions{Container: containerName})
	readCloser, err := req.Stream(context.TODO())
	log := ""
	if err != nil {
		return log, err
	}
	defer readCloser.Close()
	b, err := io.ReadAll(readCloser)
	if err != nil {
		return log, err
	}
	return string(b[:]), nil
}
