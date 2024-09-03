package integration

import (
	"context"
	"fmt"
	"sort"
	"time"

	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CreateIntegrationPipelineRun creates new integrationPipelineRun.
func (i *IntegrationController) CreateIntegrationPipelineRun(snapshotName, namespace, componentName, integrationTestScenarioName string) (*tektonv1.PipelineRun, error) {
	testpipelineRun := &tektonv1.PipelineRun{
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
		Spec: tektonv1.PipelineRunSpec{
			PipelineRef: tekton.NewBundleResolverPipelineRef(
				"integration-pipeline-pass",
				"quay.io/redhat-appstudio/example-tekton-bundle:integration-pipeline-pass",
			),
			Params: []tektonv1.Param{
				{
					Name: "output-image",
					Value: tektonv1.ParamValue{
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
func (i *IntegrationController) GetBuildPipelineRun(componentName, applicationName, namespace string, pacBuild bool, sha string) (*tektonv1.PipelineRun, error) {
	var pipelineRun *tektonv1.PipelineRun

	err := wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 20*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		pipelineRunLabels := map[string]string{"appstudio.openshift.io/component": componentName, "appstudio.openshift.io/application": applicationName, "pipelines.appstudio.openshift.io/type": "build"}

		if sha != "" {
			pipelineRunLabels["pipelinesascode.tekton.dev/sha"] = sha
		}

		list := &tektonv1.PipelineRunList{}
		err = i.KubeRest().List(context.Background(), list, &client.ListOptions{LabelSelector: labels.SelectorFromSet(pipelineRunLabels), Namespace: namespace})

		if err != nil && !k8sErrors.IsNotFound(err) {
			GinkgoWriter.Printf("error listing pipelineruns in %s namespace: %v", namespace, err)
			return false, nil
		}

		if len(list.Items) > 0 {
			// sort PipelineRuns by StartTime in ascending order
			sort.Slice(list.Items, func(i, j int) bool {
				return list.Items[i].Status.StartTime.Before(list.Items[j].Status.StartTime)
			})
			// get latest pipelineRun
			pipelineRun = &list.Items[len(list.Items)-1]
			return true, nil
		}

		pipelineRun = &tektonv1.PipelineRun{}
		GinkgoWriter.Printf("no pipelinerun found for component %s %s", componentName, utils.GetAdditionalInfo(applicationName, namespace))
		return false, nil
	})

	return pipelineRun, err
}

// GetIntegrationPipelineRun returns the integration pipelineRun
// for a given scenario, snapshot labels.
func (i *IntegrationController) GetIntegrationPipelineRun(integrationTestScenarioName string, snapshotName string, namespace string) (*tektonv1.PipelineRun, error) {
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"pipelines.appstudio.openshift.io/type": "test",
			"test.appstudio.openshift.io/scenario":  integrationTestScenarioName,
			"appstudio.openshift.io/snapshot":       snapshotName,
		},
	}

	list := &tektonv1.PipelineRunList{}
	err := i.KubeRest().List(context.Background(), list, opts...)

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing pipelineruns in %s namespace", namespace)
	}

	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return &tektonv1.PipelineRun{}, fmt.Errorf("no pipelinerun found for integrationTestScenario %s (snapshot: %s, namespace: %s)", integrationTestScenarioName, snapshotName, namespace)
}

// WaitForIntegrationPipelineToGetStarted wait for given integration pipeline to get started.
// In case of failure, this function retries till it gets timed out.
func (i *IntegrationController) WaitForIntegrationPipelineToGetStarted(testScenarioName, snapshotName, appNamespace string) (*tektonv1.PipelineRun, error) {
	var testPipelinerun *tektonv1.PipelineRun

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
func (i *IntegrationController) WaitForIntegrationPipelineToBeFinished(testScenario *integrationv1beta2.IntegrationTestScenario, snapshot *appstudioApi.Snapshot, appNamespace string) error {
	return wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 20*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		pipelineRun, err := i.GetIntegrationPipelineRun(testScenario.Name, snapshot.Name, appNamespace)
		if err != nil {
			GinkgoWriter.Println("PipelineRun has not been created yet for test scenario %s and snapshot %s/%s", testScenario.GetName(), snapshot.GetNamespace(), snapshot.GetName())
			return false, nil
		}
		GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pipelineRun.Name, pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason())

		if !pipelineRun.IsDone() {
			return false, nil
		}

		if pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
			return true, nil
		}
		var prLogs string
		if prLogs, err = tekton.GetFailedPipelineRunLogs(i.KubeRest(), i.KubeInterface(), pipelineRun); err != nil {
			return false, fmt.Errorf("failed to get PLR logs: %+v", err)
		}
		return false, fmt.Errorf("%s", prLogs)
	})
}

func (i *IntegrationController) isScenarioInExpectedScenarios(testScenario *integrationv1beta2.IntegrationTestScenario, expectedTestScenarios []string) bool {
	for _, expectedScenario := range expectedTestScenarios {
		if expectedScenario == testScenario.Name {
			return true
		}
	}
	return false
}

// WaitForAllIntegrationPipelinesToBeFinished wait for all integration pipelines to finish.
func (i *IntegrationController) WaitForAllIntegrationPipelinesToBeFinished(testNamespace, applicationName string, snapshot *appstudioApi.Snapshot, expectedTestScenarios []string) error {
	integrationTestScenarios, err := i.GetIntegrationTestScenarios(applicationName, testNamespace)
	if err != nil {
		return fmt.Errorf("unable to get IntegrationTestScenarios for Application %s/%s. Error: %v", testNamespace, applicationName, err)
	}

	for _, testScenario := range *integrationTestScenarios {
		testScenario := testScenario
		if len(expectedTestScenarios) == 0 || i.isScenarioInExpectedScenarios(&testScenario, expectedTestScenarios) {
			GinkgoWriter.Printf("Integration test scenario %s is found\n", testScenario.Name)
			err = i.WaitForIntegrationPipelineToBeFinished(&testScenario, snapshot, testNamespace)
			if err != nil {
				return fmt.Errorf("error occurred while waiting for Integration PLR (associated with IntegrationTestScenario: %s) to get finished in %s namespace. Error: %v", testScenario.Name, testNamespace, err)
			}
		}
	}

	return nil
}

// WaitForFinalizerToGetRemovedFromAllIntegrationPipelineRuns waits for
// the given finalizer to get removed from all integration pipelinesruns
// that are related to the given application and namespace.
func (i *IntegrationController) WaitForFinalizerToGetRemovedFromAllIntegrationPipelineRuns(testNamespace, applicationName string, snapshot *appstudioApi.Snapshot, expectedTestScenarios []string) error {
	integrationTestScenarios, err := i.GetIntegrationTestScenarios(applicationName, testNamespace)
	if err != nil {
		return fmt.Errorf("unable to get IntegrationTestScenarios for Application %s/%s. Error: %v", testNamespace, applicationName, err)
	}

	for _, testScenario := range *integrationTestScenarios {
		testScenario := testScenario
		if len(expectedTestScenarios) == 0 || i.isScenarioInExpectedScenarios(&testScenario, expectedTestScenarios) {
			GinkgoWriter.Printf("Integration test scenario %s is found\n", testScenario.Name)
			err = i.WaitForFinalizerToGetRemovedFromIntegrationPipeline(&testScenario, snapshot, testNamespace)
			if err != nil {
				return fmt.Errorf("error occurred while waiting for Integration PLR (associated with IntegrationTestScenario: %s) to NOT have the finalizer. Error: %v", testScenario.Name, err)
			}
		}
	}
	return nil
}

// WaitForFinalizerToGetRemovedFromIntegrationPipeline waits for the
// given finalizer to get removed from the given integration pipelinerun
func (i *IntegrationController) WaitForFinalizerToGetRemovedFromIntegrationPipeline(testScenario *integrationv1beta2.IntegrationTestScenario, snapshot *appstudioApi.Snapshot, appNamespace string) error {
	return wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 10*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		pipelineRun, err := i.GetIntegrationPipelineRun(testScenario.Name, snapshot.Name, appNamespace)
		if err != nil {
			GinkgoWriter.Println("PipelineRun has not been created yet for test scenario %s and snapshot %s/%s", testScenario.GetName(), snapshot.GetNamespace(), snapshot.GetName())
			return false, nil
		}
		if controllerutil.ContainsFinalizer(pipelineRun, "test.appstudio.openshift.io/pipelinerun") {
			GinkgoWriter.Printf("build pipelineRun %s/%s still contains the finalizer: %s", pipelineRun.GetNamespace(), pipelineRun.GetName(), "test.appstudio.openshift.io/pipelinerun")
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
