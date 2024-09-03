package integration

import (
	"context"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	integrationv1beta2 "github.com/konflux-ci/integration-service/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateIntegrationTestScenario creates beta1 version integrationTestScenario.
func (i *IntegrationController) CreateIntegrationTestScenario(itsName, applicationName, namespace, gitURL, revision, pathInRepo string, contexts []string) (*integrationv1beta2.IntegrationTestScenario, error) {
	if itsName == "" {
		itsName = "my-integration-test-" + util.GenerateRandomString(4)
	}

	integrationTestScenario := &integrationv1beta2.IntegrationTestScenario{
		ObjectMeta: metav1.ObjectMeta{
			Name:      itsName,
			Namespace: namespace,
			Labels:    constants.IntegrationTestScenarioDefaultLabels,
		},
		Spec: integrationv1beta2.IntegrationTestScenarioSpec{
			Application: applicationName,
			ResolverRef: integrationv1beta2.ResolverRef{
				Resolver: "git",
				Params: []integrationv1beta2.ResolverParameter{
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
			Contexts: []integrationv1beta2.TestContext{},
		},
	}

	if len(contexts) > 0 {
		for _, testContext := range contexts {
			integrationTestScenario.Spec.Contexts = append(integrationTestScenario.Spec.Contexts,
				integrationv1beta2.TestContext{Name: testContext, Description: testContext})
		}
	}

	err := i.KubeRest().Create(context.Background(), integrationTestScenario)
	if err != nil {
		return nil, err
	}
	return integrationTestScenario, nil
}

// Get return the status from the Application Custom Resource object.
func (i *IntegrationController) GetIntegrationTestScenarios(applicationName, namespace string) (*[]integrationv1beta2.IntegrationTestScenario, error) {
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	integrationTestScenarioList := &integrationv1beta2.IntegrationTestScenarioList{}
	err := i.KubeRest().List(context.Background(), integrationTestScenarioList, opts...)
	if err != nil {
		return nil, err
	}

	items := make([]integrationv1beta2.IntegrationTestScenario, 0)
	for _, t := range integrationTestScenarioList.Items {
		if t.Spec.Application == applicationName {
			items = append(items, t)
		}
	}
	return &items, nil
}

// DeleteIntegrationTestScenario removes given testScenario from specified namespace.
func (i *IntegrationController) DeleteIntegrationTestScenario(testScenario *integrationv1beta2.IntegrationTestScenario, namespace string) error {
	err := i.KubeRest().Delete(context.Background(), testScenario)
	return err
}
