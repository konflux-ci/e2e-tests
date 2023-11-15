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
	"k8s.io/apimachinery/pkg/labels"
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
			PipelineRef: utils.NewBundleResolverPipelineRef(
				"integration-pipeline-pass",
				"quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass",
			),
			Params: []tektonv1beta1.Param{
				{
					Name: "output-image",
					Value: tektonv1beta1.ParamValue{
						Type:      "string",
						StringVal: "quay.io/redhat-appstudio/sample-image",
					},
				},
			},
		},
	}
	err := i.KubeRest().Create(context.Background(), testpipelineRun)
	if err != nil {
		return nil, err
	}
	return testpipelineRun, err
}

// GetComponentPipeline returns the pipeline for a given component labels.
// In case of failure, this function retries till it gets timed out.
func (i *IntegrationController) GetBuildPipelineRun(componentName, applicationName, namespace string, pacBuild bool, sha string) (*tektonv1beta1.PipelineRun, error) {
	var pipelineRun *tektonv1beta1.PipelineRun

	err := wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 20*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		pipelineRunLabels := map[string]string{"appstudio.openshift.io/component": componentName, "appstudio.openshift.io/application": applicationName, "pipelines.appstudio.openshift.io/type": "build"}

		if sha != "" {
			pipelineRunLabels["pipelinesascode.tekton.dev/sha"] = sha
		}

		list := &tektonv1beta1.PipelineRunList{}
		err = i.KubeRest().List(context.Background(), list, &client.ListOptions{LabelSelector: labels.SelectorFromSet(pipelineRunLabels), Namespace: namespace})

		if err != nil && !k8sErrors.IsNotFound(err) {
			GinkgoWriter.Printf("error listing pipelineruns in %s namespace: %v", namespace, err)
			return false, nil
		}

		if len(list.Items) > 0 {
			pipelineRun = &list.Items[0]
			return true, nil
		}

		pipelineRun = &tektonv1beta1.PipelineRun{}
		GinkgoWriter.Printf("no pipelinerun found for component %s %s", componentName, utils.GetAdditionalInfo(applicationName, namespace))
		return false, nil
	})

	return pipelineRun, err
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
	err := i.KubeRest().List(context.Background(), list, opts...)

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing pipelineruns in %s namespace", namespace)
	}

	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return &tektonv1beta1.PipelineRun{}, fmt.Errorf("no pipelinerun found for integrationTestScenario %s (snapshot: %s, namespace: %s)", integrationTestScenarioName, snapshotName, namespace)
}

// WaitForIntegrationPipelineToGetStarted wait for given integration pipeline to get started.
// In case of failure, this function retries till it gets timed out.
func (i *IntegrationController) WaitForIntegrationPipelineToGetStarted(testScenarioName, snapshotName, appNamespace string) (*tektonv1beta1.PipelineRun, error) {
	var testPipelinerun *tektonv1beta1.PipelineRun

	err := wait.PollUntilContextTimeout(context.Background(), time.Second*2, time.Minute*5, true, func(ctx context.Context) (done bool, err error) {
		testPipelinerun, err = i.GetIntegrationPipelineRun(testScenarioName, snapshotName, appNamespace)
		if err != nil {
			GinkgoWriter.Println("PipelineRun has not been created yet for test scenario %s and snapshot %s/%s", testScenarioName, appNamespace, snapshotName)
			return false, nil
		}
		if !testPipelinerun.HasStarted() {
			GinkgoWriter.Println("pipelinerun %s/%s hasn't started yet", testPipelinerun.GetNamespace(), testPipelinerun.GetName())
			return false, nil
		}
		return true, nil
	})

	return testPipelinerun, err
}

// WaitForIntegrationPipelineToBeFinished wait for given integration pipeline to finish.
// In case of failure, this function retries till it gets timed out.
func (i *IntegrationController) WaitForIntegrationPipelineToBeFinished(testScenario *integrationv1beta1.IntegrationTestScenario, snapshot *appstudioApi.Snapshot, appNamespace string) error {
	return wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 20*time.Minute, true, func(ctx context.Context) (done bool, err error) {
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

// WaitForAllIntegrationPipelinesToBeFinished wait for all integration pipelines to finish.
func (i *IntegrationController) WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName string, snapshot *appstudioApi.Snapshot) error {
	integrationTestScenarios, err := i.GetIntegrationTestScenarios(applicationName, testNamespace)
	if err != nil {
		return fmt.Errorf("unable to get IntegrationTestScenarios for Application %s/%s. Error: %v", testNamespace, applicationName, err)
	}

	for _, testScenario := range *integrationTestScenarios {
		GinkgoWriter.Printf("Integration test scenario %s is found\n", testScenario.Name)
		err = i.WaitForIntegrationPipelineToBeFinished(&testScenario, snapshot, testNamespace)
		if err != nil {
			return fmt.Errorf("error occurred while waiting for Integration PLR (associated with IntegrationTestScenario: %s) to get finished in %s namespace. Error: %v", testScenario.Name, testNamespace, err)
		}
	}

	return nil
}

// WaitForBuildPipelineRunToBeSigned waits for given build pipeline to get signed.
// In case of failure, this function retries till it gets timed out.
func (i *IntegrationController) WaitForBuildPipelineRunToBeSigned(testNamespace, applicationName, componentName string) error {
	return wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 10*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		pipelineRun, err := i.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
		if err != nil {
			GinkgoWriter.Printf("pipelinerun for Component %s/%s can't be gotten successfully. Error: %v", testNamespace, componentName, err)
			return false, nil
		}
		if pipelineRun.Annotations["chains.tekton.dev/signed"] != "true" {
			GinkgoWriter.Printf("pipelinerun %s/%s hasn't been signed yet", pipelineRun.GetNamespace(), pipelineRun.GetName())
			return false, nil
		}
		return true, nil
	})
}

// GetAnnotationIfExists returns the value of a given annotation within a pipelinerun, if it exists.
func (i *IntegrationController) GetAnnotationIfExists(testNamespace, applicationName, componentName, annotationKey string) (string, error) {
	pipelineRun, err := i.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
	if err != nil {
		return "", fmt.Errorf("pipelinerun for Component %s/%s can't be gotten successfully. Error: %v", testNamespace, componentName, err)
	}
	return pipelineRun.Annotations[annotationKey], nil
}

// WaitForBuildPipelineRunToGetAnnotated waits for given build pipeline to get annotated with a specific annotation.
// In case of failure, this function retries till it gets timed out.
func (i *IntegrationController) WaitForBuildPipelineRunToGetAnnotated(testNamespace, applicationName, componentName, annotationKey string) error {
	return wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		pipelineRun, err := i.GetBuildPipelineRun(componentName, applicationName, testNamespace, false, "")
		if err != nil {
			GinkgoWriter.Printf("pipelinerun for Component %s/%s can't be gotten successfully. Error: %v", testNamespace, componentName, err)
			return false, nil
		}

		annotationValue, _ := i.GetAnnotationIfExists(testNamespace, applicationName, componentName, annotationKey)
		if annotationValue == "" {
			GinkgoWriter.Printf("build pipelinerun %s/%s doesn't contain annotation %s yet", testNamespace, pipelineRun.Name, annotationKey)
			return false, nil
		}
		return true, nil
	})
}
