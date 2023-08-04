package integration

import (
	"context"

	"github.com/devfile/library/v2/pkg/util"
	integrationv1alpha1 "github.com/redhat-appstudio/integration-service/api/v1alpha1"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateIntegrationTestScenario creates new integrationTestScenario.
func (i *IntegrationController) CreateIntegrationTestScenario(applicationName, namespace, bundleURL, pipelineName string) (*integrationv1alpha1.IntegrationTestScenario, error) {
	integrationTestScenario := &integrationv1alpha1.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-pass-" + util.GenerateRandomString(4),
			Namespace: namespace,
			Labels: map[string]string{
				"test.appstudio.openshift.io/optional": "false",
			},
		},
		Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
			Application: applicationName,
			Bundle:      bundleURL,
			Pipeline:    pipelineName,
		},
	}

	err := i.KubeRest().Create(context.TODO(), integrationTestScenario)
	if err != nil {
		return nil, err
	}
	return integrationTestScenario, nil
}

// CreateIntegrationTestScenarioWithEnvironment creates new integrationTestScenario with given Environment.
func (i *IntegrationController) CreateIntegrationTestScenarioWithEnvironment(applicationName, namespace, bundleURL, pipelineName, environmentName string) (*integrationv1alpha1.IntegrationTestScenario, error) {
	integrationTestScenario := &integrationv1alpha1.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-pass-" + util.GenerateRandomString(4),
			Namespace: namespace,
			Labels: map[string]string{
				"test.appstudio.openshift.io/optional": "false",
			},
		},
		Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
			Application: applicationName,
			Bundle:      bundleURL,
			Pipeline:    pipelineName,
			Environment: integrationv1alpha1.TestEnvironment{
				Name: environmentName,
				Type: "POC",
			},
		},
	}

	err := i.KubeRest().Create(context.TODO(), integrationTestScenario)
	if err != nil {
		return nil, err
	}
	return integrationTestScenario, nil
}

// CreateIntegrationTestScenario_beta1 creates new beta1 version integrationTestScenario.
func (i *IntegrationController) CreateIntegrationTestScenario_beta1(applicationName, namespace, gitURL, revision, pathInRepo string) (*integrationv1beta1.IntegrationTestScenario, error) {
	integrationTestScenario := &integrationv1beta1.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-resolver-pass-" + util.GenerateRandomString(4),
			Namespace: namespace,
			Labels: map[string]string{
				"test.appstudio.openshift.io/optional": "false",
			},
		},
		Spec: integrationv1beta1.IntegrationTestScenarioSpec{
			Application: applicationName,
			ResolverRef: integrationv1beta1.ResolverRef{
				Resolver: "git",
				Params: []integrationv1beta1.ResolverParameter{
					{
						Name:  "url",
						Value: gitURL,
					},
					{
						Name:  "revision",
						Value: revision,
					},
					{
						Name:  "pathInRepo",
						Value: pathInRepo,
					},
				},
			},
		},
	}

	err := i.KubeRest().Create(context.TODO(), integrationTestScenario)
	if err != nil {
		return nil, err
	}
	return integrationTestScenario, nil
}

// Get return the status from the Application Custom Resource object.
func (i *IntegrationController) GetIntegrationTestScenarios(applicationName, namespace string) (*[]integrationv1beta1.IntegrationTestScenario, error) {
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	integrationTestScenarioList := &integrationv1beta1.IntegrationTestScenarioList{}
	err := i.KubeRest().List(context.TODO(), integrationTestScenarioList, opts...)
	if err != nil {
		return nil, err
	}

	items := make([]integrationv1beta1.IntegrationTestScenario, 0)
	for _, t := range integrationTestScenarioList.Items {
		if t.Spec.Application == applicationName {
			items = append(items, t)
		}
	}
	return &items, nil
}

// DeleteIntegrationTestScenario removes given testScenario from specified namespace.
func (i *IntegrationController) DeleteIntegrationTestScenario(testScenario *integrationv1beta1.IntegrationTestScenario, namespace string) error {
	err := i.KubeRest().Delete(context.TODO(), testScenario)
	return err
}
