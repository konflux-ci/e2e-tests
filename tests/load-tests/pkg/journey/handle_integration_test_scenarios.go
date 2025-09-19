package journey

import (
	"fmt"
	"time"

	logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

	framework "github.com/konflux-ci/e2e-tests/pkg/framework"

	utils "github.com/konflux-ci/e2e-tests/pkg/utils"
)

func createIntegrationTestScenario(f *framework.Framework, namespace, name, appName, scenarioGitURL, scenarioRevision, scenarioPathInRepo string) error {
	interval := time.Second * 10
	timeout := time.Minute * 1

	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		_, err = f.AsKubeDeveloper.IntegrationController.CreateIntegrationTestScenario(name, appName, namespace, scenarioGitURL, scenarioRevision, scenarioPathInRepo, "", []string{})
		if err != nil {
			logging.Logger.Debug("Failed to create the Integration Test Scenario %s in namespace %s: %v", name, namespace, err)
			return false, nil
		}

		return true, nil
	}, interval, timeout)
	if err != nil {
		return fmt.Errorf("Unable to create the Integration Test Scenario %s in namespace %s: %v", name, namespace, err)
	}

	return nil
}

func HandleIntegrationTestScenario(ctx *PerApplicationContext) error {
	if ctx.ParentContext.Opts.TestScenarioGitURL == "" {
		logging.Logger.Debug("Integration Test Scenario GIT not provided, not creating it")
		return nil
	}

	var err error

	name := fmt.Sprintf("%s-its", ctx.ApplicationName)
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

	ctx.IntegrationTestScenarioName = name

	return nil
}
