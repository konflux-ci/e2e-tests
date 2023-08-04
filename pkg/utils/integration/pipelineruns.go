package integration

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateIntegrationPipelineRun creates new integrationPipelineRun.
func (i *IntegrationController) CreateIntegrationPipelineRun(snapshotName, namespace, componentName, integrationTestScenarioName string) (*tektonv1beta1.PipelineRun, error) {
	testpipelineRun := &tektonv1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "component-pipelinerun" + "-",
			Namespace:    namespace,
			Labels: map[string]string{
				"pipelinesascode.tekton.dev/event-type": "push",
				"appstudio.openshift.io/component":      componentName,
				"pipelines.appstudio.openshift.io/type": "test",
				"appstudio.openshift.io/snapshot":       snapshotName,
				"test.appstudio.openshift.io/scenario":  integrationTestScenarioName,
			},
		},
		Spec: tektonv1beta1.PipelineRunSpec{
			PipelineRef: &tektonv1beta1.PipelineRef{
				Name:   "integration-pipeline-pass",
				Bundle: "quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass",
			},
			Params: []tektonv1beta1.Param{
				{
					Name: "output-image",
					Value: tektonv1beta1.ArrayOrString{
						Type:      "string",
						StringVal: "quay.io/redhat-appstudio/sample-image",
					},
				},
			},
		},
	}
	err := i.KubeRest().Create(context.TODO(), testpipelineRun)
	if err != nil {
		return nil, err
	}
	return testpipelineRun, err
}

// GetComponentPipeline returns the pipeline for a given component labels.
func (i *IntegrationController) GetBuildPipelineRun(componentName, applicationName, namespace string, pacBuild bool, sha string) (*tektonv1beta1.PipelineRun, error) {
	pipelineRunLabels := map[string]string{"appstudio.openshift.io/component": componentName, "appstudio.openshift.io/application": applicationName, "pipelines.appstudio.openshift.io/type": "build"}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"pipelines.appstudio.openshift.io/type": "build",
			"appstudio.openshift.io/application":    applicationName,
			"appstudio.openshift.io/component":      componentName,
		},
	}

	if sha != "" {
		pipelineRunLabels["pipelinesascode.tekton.dev/sha"] = sha
	}

	list := &tektonv1beta1.PipelineRunList{}
	err := i.KubeRest().List(context.TODO(), list, opts...)

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing pipelineruns in %s namespace: %v", namespace, err)
	}

	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return &tektonv1beta1.PipelineRun{}, fmt.Errorf("no pipelinerun found for component %s %s", componentName, utils.GetAdditionalInfo(applicationName, namespace))
}

// GetComponentPipeline returns the pipeline for a given component labels.
func (i *IntegrationController) GetIntegrationPipelineRun(integrationTestScenarioName string, snapshotName string, namespace string) (*tektonv1beta1.PipelineRun, error) {

	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"pipelines.appstudio.openshift.io/type": "test",
			"test.appstudio.openshift.io/scenario":  integrationTestScenarioName,
			"appstudio.openshift.io/snapshot":       snapshotName,
		},
	}

	list := &tektonv1beta1.PipelineRunList{}
	err := i.KubeRest().List(context.TODO(), list, opts...)

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing pipelineruns in %s namespace", namespace)
	}

	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return &tektonv1beta1.PipelineRun{}, fmt.Errorf("no pipelinerun found for integrationTestScenario %s (snapshot: %s, namespace: %s)", integrationTestScenarioName, snapshotName, namespace)
}

// WaitForIntegrationPipelineToBeFinished wait for given integration pipeline to finish.
func (i *IntegrationController) WaitForIntegrationPipelineToBeFinished(testScenario *integrationv1beta1.IntegrationTestScenario, snapshot *appstudioApi.Snapshot, appNamespace string) error {
	return wait.PollImmediate(constants.PipelineRunPollingInterval, 20*time.Minute, func() (done bool, err error) {
		pipelineRun, err := i.GetIntegrationPipelineRun(testScenario.Name, snapshot.Name, appNamespace)
		if err != nil {
			GinkgoWriter.Println("PipelineRun has not been created yet for test scenario %s and snapshot %s/%s", testScenario.GetName(), snapshot.GetNamespace(), snapshot.GetName())
			return false, nil
		}
		for _, condition := range pipelineRun.Status.Conditions {
			GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pipelineRun.Name, condition.Reason)

			if !pipelineRun.IsDone() {
				return false, nil
			}

			if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
				return true, nil
			} else {
				return false, fmt.Errorf(tekton.GetFailedPipelineRunLogs(i.KubeRest(), i.KubeInterface(), pipelineRun))
			}
		}
		return false, nil
	})
}
