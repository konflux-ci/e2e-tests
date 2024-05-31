package jvmbuildservice

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/konflux-ci/e2e-tests/pkg/clients/common"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// WaitForCache waits for cache to exist.
func (j *JvmbuildserviceController) WaitForCache(commonctrl *common.SuiteController, testNamespace string) error {
	return wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		cache, err := commonctrl.GetDeployment(v1alpha1.CacheDeploymentName, testNamespace)
		if err != nil {
			GinkgoWriter.Printf("failed to get JBS cache deployment: %s\n", err.Error())
			return false, nil
		}
		if cache.Status.AvailableReplicas > 0 {
			GinkgoWriter.Printf("JBS cache is available\n")
			return true, nil
		}
		for _, cond := range cache.Status.Conditions {
			if cond.Type == v1.DeploymentProgressing && cond.Status == "False" {
				return false, fmt.Errorf("JBS cache %s/%s deployment failed", testNamespace, v1alpha1.CacheDeploymentName)
			}
		}
		GinkgoWriter.Printf("JBS cache %s/%s is progressing\n", testNamespace, v1alpha1.CacheDeploymentName)
		return false, nil
	})
}
