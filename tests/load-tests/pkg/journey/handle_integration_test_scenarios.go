package journey

import (
	"context"
	"fmt"
	"strings"
	"time"

	logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

	framework "github.com/konflux-ci/e2e-tests/pkg/framework"

	util "github.com/devfile/library/v2/pkg/util"

	utils "github.com/konflux-ci/e2e-tests/pkg/utils"

	integrationApi "github.com/konflux-ci/integration-service/api/v1beta2"

	types "k8s.io/apimachinery/pkg/types"
)

func createIntegrationTestScenario(f *framework.Framework, namespace, name, appName, scenarioGitURL, scenarioRevision, scenarioPathInRepo string) error {
	_, err := f.AsKubeDeveloper.IntegrationController.CreateIntegrationTestScenario(name, appName, namespace, scenarioGitURL, scenarioRevision, scenarioPathInRepo, []string{})
	if err != nil {
		return fmt.Errorf("Unable to create the Integration Test Scenario %s: %v", name, err)
	}
	return nil
}

func validateIntegrationTestScenario(f *framework.Framework, namespace, name, appName string) error {
	interval := time.Second * 20
	timeout := time.Minute * 15
	var its integrationApi.IntegrationTestScenario

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		err = f.AsKubeDeveloper.IntegrationController.KubeRest().Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, &its)
		if err != nil {
			logging.Logger.Debug("Unable to get created integration test scenario %s for application %s in namespace %s: %v", name, appName, namespace, err)
			return false, nil
		}

		// Check if there are some conditions
		if len(its.Status.Conditions) == 0 {
			logging.Logger.Debug("Integration test scenario %s for application %s in namespace %s lacks status conditions", name, appName, namespace)
			return false, nil
		}

		// Check right condition status
		for _, condition := range its.Status.Conditions {
			if (strings.HasPrefix(condition.Type, "Error") || strings.HasSuffix(condition.Type, "Error")) && condition.Status == "True" {
				return false, fmt.Errorf("Integration test scenario %s for application %s in namespace %s is in error state: %+v", name, appName, namespace, condition)
			}
			if condition.Type == "IntegrationTestScenarioValid" && condition.Status == "True" {
				return true, nil
			}
		}

		logging.Logger.Debug("Unknown error when validating application %s in namespace %s", name, namespace)
		return false, nil
	}, interval, timeout)

	return err

}

func HandleIntegrationTestScenario(ctx *PerApplicationContext) error {
	var err error

	name := fmt.Sprintf("%s-its-%s", ctx.ParentContext.Username, util.GenerateRandomString(5))
	logging.Logger.Debug("Creating integration test scenario %s for application %s in namespace %s", name, ctx.ApplicationName, ctx.ParentContext.Namespace)

	_, err = logging.Measure(
		createIntegrationTestScenario,
		ctx.Framework,
		ctx.ParentContext.Namespace,
		name,
		ctx.ApplicationName,
		ctx.ParentContext.Opts.TestScenarioGitURL,
		ctx.ParentContext.Opts.TestScenarioRevision,
		ctx.ParentContext.Opts.TestScenarioPathInRepo,
	)
	if err != nil {
		return logging.Logger.Fail(40, "Integration test scenario failed creation: %v", err)
	}

	_, err = logging.Measure(
		validateIntegrationTestScenario,
		ctx.Framework,
		ctx.ParentContext.Namespace,
		name,
		ctx.ApplicationName,
	)
	if err != nil {
		return logging.Logger.Fail(41, "Integration test scenario failed validation: %v", err)
	}

	ctx.IntegrationTestScenarioName = name

	return nil
}
