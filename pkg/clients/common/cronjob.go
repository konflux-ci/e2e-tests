package common

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetCronJob returns cronjob if found in namespace with the given name, else an error will be returned
func (s *SuiteController) GetCronJob(namespace, name string) (*batchv1.CronJob, error) {
	cronJob, err := s.KubeInterface().BatchV1().CronJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CronJob %s/%s: %v", namespace, name, err)
	}
	return cronJob, nil
}
