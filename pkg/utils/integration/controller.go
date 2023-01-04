package integration

import (
	"context"
	"fmt"
	"time"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	integrationv1alpha1 "github.com/redhat-appstudio/integration-service/api/v1alpha1"
	releasev1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func (h *SuiteController) GetApplicationSnapshot(applicationName, namespace, componentName string) (*appstudioApi.Snapshot, error) {
	applicationSnapshots := &appstudioApi.SnapshotList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	err := h.KubeRest().List(context.TODO(), applicationSnapshots, opts...)
	if err != nil {
		return nil, err
	}
	for _, applicationSnapshot := range applicationSnapshots.Items {
		if applicationSnapshot.Spec.Application == applicationName &&
			applicationSnapshot.Labels["test.appstudio.openshift.io/component"] == componentName {
			return &applicationSnapshot, nil
		}
	}

	return nil, nil
}

func (h *SuiteController) GetReleasesWithApplicationSnapshot(applicationSnapshot *appstudioApi.Snapshot) (*[]releasev1alpha1.Release, error) {
	releases := &releasev1alpha1.ReleaseList{}
	opts := []client.ListOption{
		client.InNamespace(applicationSnapshot.Namespace),
		client.MatchingFields{"spec.applicationSnapshot": applicationSnapshot.Name},
	}

	err := h.KubeRest().List(context.TODO(), releases, opts...)
	if err != nil {
		return nil, err
	}

	for _, release := range releases.Items {
		klog.Infof("Release %s is found", release.Name)
	}

	return &releases.Items, nil
}

// Get return the status from the Application Custom Resource object
func (h *SuiteController) GetIntegrationTestScenarios(applicationName, namespace string) (*[]integrationv1alpha1.IntegrationTestScenario, error) {
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	integrationTestScenarioList := &integrationv1alpha1.IntegrationTestScenarioList{}
	err := h.KubeRest().List(context.TODO(), integrationTestScenarioList, opts...)
	if err != nil {
		return nil, err
	}
	return &integrationTestScenarioList.Items, nil
}

func (h *SuiteController) CreateApplicationSnapshot(applicationName, namespace, componentName string) (*appstudioApi.Snapshot, error) {
	sample_image := "quay.io/redhat-appstudio/sample-image"

	hasSnapshot := &appstudioApi.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snapshot-sample",
			Namespace: namespace,
			Labels: map[string]string{
				"test.appstudio.openshift.io/type":      "component",
				"test.appstudio.openshift.io/component": componentName,
				"pipelinesascode.tekton.dev/event-type": "push",
			},
		},
		Spec: appstudioApi.SnapshotSpec{
			Application: applicationName,
			Components: []appstudioApi.SnapshotComponent{
				{
					Name:           componentName,
					ContainerImage: sample_image,
				},
			},
		},
	}
	err := h.KubeRest().Create(context.TODO(), hasSnapshot)
	if err != nil {
		return nil, err
	}
	return hasSnapshot, err
}

func (h *SuiteController) CreateIntegrationPipelineRun(applicationSnapshotName, namespace, componentName string) (*tektonv1beta1.PipelineRun, error) {
	testpipelineRun := &tektonv1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "component-pipelinerun" + "-",
			Namespace:    namespace,
			Labels: map[string]string{
				"pipelinesascode.tekton.dev/event-type":  "push",
				"build.appstudio.openshift.io/component": componentName,
				"pipelines.appstudio.openshift.io/type":  "test",
				"test.appstudio.openshift.io/snapshot":   applicationSnapshotName,
				"test.appstudio.openshift.io/scenario":   "example-pass",
			},
		},
		Spec: tektonv1beta1.PipelineRunSpec{
			PipelineRef: &tektonv1beta1.PipelineRef{
				Name:   "component-pipeline-pass",
				Bundle: "quay.io/kpavic/test-bundle:component-pipeline-pass",
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
	err := h.KubeRest().Create(context.TODO(), testpipelineRun)
	if err != nil {
		return nil, err
	}
	return testpipelineRun, err
}

func (h *SuiteController) CreateIntegrationTestScenario(applicationName, namespace, bundleURL, pipelineName string) (*integrationv1alpha1.IntegrationTestScenario, error) {
	integrationTestScenario := &integrationv1alpha1.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-pass",
			Namespace: namespace,
			Labels: map[string]string{
				"test.appstudio.openshift.io/optional": "false",
			},
		},
		Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
			Application: applicationName,
			Bundle:      "quay.io/kpavic/test-bundle:component-pipeline-pass",
			Pipeline:    "component-pipeline-pass",
			Environment: integrationv1alpha1.TestEnvironment{
				Name:   "envname",
				Type:   "POC",
				Params: []string{},
			},
		},
	}

	err := h.KubeRest().Create(context.TODO(), integrationTestScenario)
	if err != nil {
		return nil, err
	}
	return integrationTestScenario, nil
}

func (h *SuiteController) WaitForIntegrationPipelineToBeFinished(testScenario *integrationv1alpha1.IntegrationTestScenario, applicationSnapshot *appstudioApi.Snapshot, applicationName string, appNamespace string) error {
	return wait.PollImmediate(20*time.Second, 100*time.Minute, func() (done bool, err error) {
		pipelineRun, _ := h.GetIntegrationPipelineRun(testScenario.Name, applicationSnapshot.Name, appNamespace)

		for _, condition := range pipelineRun.Status.Conditions {
			klog.Infof("PipelineRun %s reason: %s", pipelineRun.Name, condition.Reason)

			if condition.Reason == "Failed" {
				return false, fmt.Errorf("component %s pipeline failed", pipelineRun.Name)
			}

			if condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

// GetComponentPipeline returns the pipeline for a given component labels
func (h *SuiteController) GetIntegrationPipelineRun(integrationTestScenarioName string, applicationSnapshotName string, namespace string) (*tektonv1beta1.PipelineRun, error) {

	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"pipelines.appstudio.openshift.io/type": "test",
			"test.appstudio.openshift.io/snapshot":  applicationSnapshotName,
			"test.appstudio.openshift.io/scenario":  integrationTestScenarioName,
		},
	}

	list := &tektonv1beta1.PipelineRunList{}
	err := h.KubeRest().List(context.TODO(), list, opts...)

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing pipelineruns in %s namespace", namespace)
	}

	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return &tektonv1beta1.PipelineRun{}, fmt.Errorf("no pipelinerun found for integrationTestScenario %s", integrationTestScenarioName)
}
