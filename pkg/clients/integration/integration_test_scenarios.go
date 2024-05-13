package integration

import (
	"context"
	"fmt"

	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	integrationv1beta1 "github.com/konflux-ci/integration-service/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateIntegrationTestScenarioWithEnvironment will create an IntegrationTestScenario with a
// user-supplied environment embedded in its Spec.Environment
func (i *IntegrationController) CreateIntegrationTestScenarioWithEnvironment(applicationName, namespace, gitURL, revision, pathInRepo string, environment *appservice.Environment) (*integrationv1beta1.IntegrationTestScenario, error) {
	integrationTestScenario := &integrationv1beta1.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-pass-with-env-" + util.GenerateRandomString(4),
			Namespace: namespace,
			Labels:    constants.IntegrationTestScenarioDefaultLabels,
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
			Environment: integrationv1beta1.TestEnvironment{
				Name: environment.Name,
				Type: environment.Spec.Type,
				Configuration: &appservice.EnvironmentConfiguration{
					Env: []appservice.EnvVarPair{
						{
							Name:  environment.Spec.Configuration.Env[0].Name,
							Value: environment.Spec.Configuration.Env[0].Value,
						},
					},
				},
			},
		},
	}

	err := i.KubeRest().Create(context.Background(), integrationTestScenario)
	if err != nil {
		return nil, fmt.Errorf("error occurred when creating the IntegrationTestScenario: %+v", err)
	}
	return integrationTestScenario, nil
}

// CreateIntegrationTestScenario creates beta1 version integrationTestScenario.
func (i *IntegrationController) CreateIntegrationTestScenario(itsName, applicationName, namespace, gitURL, revision, pathInRepo string) (*integrationv1beta1.IntegrationTestScenario, error) {
	if itsName == "" {
		itsName = "my-integration-test-" + util.GenerateRandomString(4)
	}

	integrationTestScenario := &integrationv1beta1.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      itsName,
			Namespace: namespace,
			Labels:    constants.IntegrationTestScenarioDefaultLabels,
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

	err := i.KubeRest().Create(context.Background(), integrationTestScenario)
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
	err := i.KubeRest().List(context.Background(), integrationTestScenarioList, opts...)
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
	err := i.KubeRest().Delete(context.Background(), testScenario)
	return err
}
