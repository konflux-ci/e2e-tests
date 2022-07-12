package integration

import (
	"context"
//	"fmt"
//	"time"

//	routev1 "github.com/openshift/api/route/v1"
	integrationservice "github.com/redhat-appstudio/integration-service/api/v1alpha1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
//	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
//	appsv1 "k8s.io/api/apps/v1"
//	corev1 "k8s.io/api/core/v1"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"k8s.io/apimachinery/pkg/labels"
//	"k8s.io/apimachinery/pkg/types"
//	"k8s.io/apimachinery/pkg/util/wait"
//	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
//	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

// getApplicationSnapshot returns the all ApplicationSnapshots in the Application's namespace nil if it's not found.
// In the case the List operation fails, an error will be returned.
func (h *SuiteController) GetAllApplicationSnapshots(applicationName, namespace string) (*[]appstudioshared.ApplicationSnapshot, error) {
	applicationSnapshots := &appstudioshared.ApplicationSnapshotList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := h.KubeRest().List(context.TODO(), applicationSnapshots, opts...)
	if err != nil {
		return nil, err
	}

	return &applicationSnapshots.Items, nil
}

// Get return the status from the Application Custom Resource object
func (h *SuiteController) GetIntegrationTestScenarios(applicationName, namespace string) (*[]integrationservice.IntegrationTestScenario, error) {
	opts := []client.ListOption{
                client.InNamespace(namespace),
        }

	integrationTestScenarioList := &integrationservice.IntegrationTestScenarioList{}
        err := h.KubeRest().List(context.TODO(), integrationTestScenarioList, opts...)
        if err != nil {
                return nil, err
        }
        return &integrationTestScenarioList.Items, nil
}

func (h *SuiteController) WaitForIntegrationPipelineToBeFinished(testScenario *integrationservice.IntegrationTestScenario, applicationSnapshots *[]appstudioshared.ApplicationSnapshot, applicationName string, appNamespace string) error {
//	return wait.PollImmediate(20*time.Second, 10*time.Minute, func() (done bool, err error) {
//		pipelineRun, _ := h.GetIntegrationPipelineRun(testScenario.Name, applicationName, appNamespace, false)

//		for _, condition := range pipelineRun.Status.Conditions {
//			klog.Infof("PipelineRun %s reason: %s", pipelineRun.Name, condition.Reason)
//
//			if condition.Reason == "Failed" {
//				return false, fmt.Errorf("component %s pipeline failed", pipelineRun.Name)
//			}
//
//			if condition.Status == corev1.ConditionTrue {
//				return true, nil
//			}
////		}
//		return false, nil
//	})
	return nil
}
