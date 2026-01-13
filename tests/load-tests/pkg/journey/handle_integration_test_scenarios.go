package journey

import (
	"fmt"
	"time"

	logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
	types "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/types"

	framework "github.com/konflux-ci/e2e-tests/pkg/framework"

	utils "github.com/konflux-ci/e2e-tests/pkg/utils"
)

func createIntegrationTestScenario(f *framework.Framework, namespace, appName, scenarioGitURL, scenarioRevision, scenarioPathInRepo string) (string, error) {
	interval := time.Second * 10
	timeout := time.Minute * 1

	name := fmt.Sprintf("%s-its", appName)
	logging.Logger.Debug("Creating integration test scenario %s for application %s in namespace %s", name, appName, namespace)

	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		_, err = f.AsKubeDeveloper.IntegrationController.CreateIntegrationTestScenario(name, appName, namespace, scenarioGitURL, scenarioRevision, scenarioPathInRepo, "", []string{})
		if err != nil {
			logging.Logger.Debug("Failed to create the Integration Test Scenario %s in namespace %s: %v", name, namespace, err)
			return false, nil
		}

		return true, nil
	}, interval, timeout)
	if err != nil {
		return "", fmt.Errorf("Unable to create the Integration Test Scenario %s in namespace %s: %v", name, namespace, err)
	}

	return name, nil
}

func HandleIntegrationTestScenario(ctx *types.PerApplicationContext) error {
	if ctx.IntegrationTestScenarioName != "" {
		logging.Logger.Debug("Skipping integration test scenario creation because reusing integration test scenario %s in namespace %s", ctx.IntegrationTestScenarioName, ctx.ParentContext.Namespace)
		return nil
	}

	if ctx.ParentContext.Opts.TestScenarioGitURL == "" {
		logging.Logger.Debug("Skipping integration test scenario creation because GIT was not provided")
		return nil
	}

	var iface interface{}
	var err error
	var ok bool

	iface, err = logging.Measure(
		ctx,
		createIntegrationTestScenario,
		ctx.Framework,
		ctx.ParentContext.Namespace,
		ctx.ApplicationName,
		ctx.ParentContext.Opts.TestScenarioGitURL,
		ctx.ParentContext.Opts.TestScenarioRevision,
		ctx.ParentContext.Opts.TestScenarioPathInRepo,
	)
	if err != nil {
		return logging.Logger.Fail(40, "Integration test scenario failed creation: %v", err)
	}

	ctx.IntegrationTestScenarioName, ok = iface.(string)
	if !ok {
		return logging.Logger.Fail(41, "Type assertion failed on integration test scenario name: %+v", iface)
	}

	return nil
}
