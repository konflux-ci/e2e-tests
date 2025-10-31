package journey

import "fmt"
import "strings"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
import types "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/types"

import appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"
import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import utils "github.com/konflux-ci/e2e-tests/pkg/utils"
import pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

func validateSnapshotCreation(f *framework.Framework, namespace, compName string) (string, error) {
	logging.Logger.Debug("Waiting for snapshot for component %s in namespace %s to be created", compName, namespace)

	interval := time.Second * 20
	timeout := time.Minute * 5
	var snap *appstudioApi.Snapshot

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		snap, err = f.AsKubeDeveloper.IntegrationController.GetSnapshot("", "", compName, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get created Snapshot for component %s in namespace %s: %v", compName, namespace, err)
			return false, nil
		}
		return true, nil
	}, interval, timeout)

	if err != nil {
		return "", err
	}

	return snap.Name, err
}

func validateTestPipelineRunCreation(f *framework.Framework, namespace, itsName, snapName string) error {
	logging.Logger.Debug("Waiting for test pipeline run for ITS %s and snapshot %s in namespace %s to be created", itsName, snapName, namespace)

	interval := time.Second * 20
	timeout := time.Minute * 5
	var pr *pipeline.PipelineRun

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		pr, err = f.AsKubeDeveloper.IntegrationController.GetIntegrationPipelineRun(itsName, snapName, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get created test PipelineRun for integration test pipeline %s in namespace %s: %v", itsName, namespace, err)
			return false, nil
		}

		logging.Logger.Debug("Test PipelineRun %s for its %s and snap %s in namespace %s created", pr.GetName(), itsName, snapName, namespace)
		return true, nil
	}, interval, timeout)

	return err
}

func validateTestPipelineRunCondition(f *framework.Framework, namespace, itsName, snapName string) error {
	logging.Logger.Debug("Waiting for test pipeline run for ITS %s and snapshot %s in namespace %s to finish", itsName, snapName, namespace)

	interval := time.Second * 20
	timeout := time.Minute * 10
	var pr *pipeline.PipelineRun

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		pr, err = f.AsKubeDeveloper.IntegrationController.GetIntegrationPipelineRun(itsName, snapName, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get created test PipelineRun for integration test pipeline %s in namespace %s: %v", snapName, namespace, err)
			return false, nil
		}

		// Check if there are some conditions
		if len(pr.Status.Conditions) == 0 {
			logging.Logger.Debug("PipelineRun for integration test pipeline %s in namespace %s lacks status conditions", snapName, namespace)
			return false, nil
		}

		// Check right condition status
		for _, condition := range pr.Status.Conditions {
			if (strings.HasPrefix(string(condition.Type), "Error") || strings.HasSuffix(string(condition.Type), "Error")) && condition.Status == "True" {
				return false, fmt.Errorf("PipelineRun for integration test pipeline %s in namespace %s is in error state: %+v", snapName, namespace, condition)
			}
			if condition.Type == "Succeeded" && condition.Status == "True" {
				return true, nil
			}
		}

		logging.Logger.Trace("Still waiting for test pipeline run for integration test pipeline %s in namespace %s", snapName, namespace)
		return false, nil
	}, interval, timeout)

	return err
}

func HandleTest(ctx *types.PerComponentContext) error {
	if !ctx.ParentContext.ParentContext.Opts.WaitPipelines || !ctx.ParentContext.ParentContext.Opts.WaitIntegrationTestsPipelines {
		return nil
	}

	var err error
	var ok bool

	result1, err1 := logging.Measure(
		validateSnapshotCreation,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		ctx.ComponentName,
	)
	if err1 != nil {
		return logging.Logger.Fail(80, "Snapshot failed creation: %v", err1)
	}
	ctx.SnapshotName, ok = result1.(string)
	if !ok {
		return logging.Logger.Fail(81, "Snapshot name type assertion failed")
	}

	if ctx.ParentContext.ParentContext.Opts.TestScenarioGitURL == "" {
		logging.Logger.Debug("Integration Test Scenario GIT not provided, not waiting for it")
	} else {
		logging.Logger.Debug("Waiting for test pipeline run for component %s in namespace %s to be created", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)

		_, err = logging.Measure(
			validateTestPipelineRunCreation,
			ctx.Framework,
			ctx.ParentContext.ParentContext.Namespace,
			ctx.ParentContext.IntegrationTestScenarioName,
			ctx.SnapshotName,
		)
		if err != nil {
			return logging.Logger.Fail(82, "Test Pipeline Run failed creation: %v", err)
		}

		logging.Logger.Debug("Waiting for test pipeline run for component %s in namespace %s to finish", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)

		_, err = logging.Measure(
			validateTestPipelineRunCondition,
			ctx.Framework,
			ctx.ParentContext.ParentContext.Namespace,
			ctx.ParentContext.IntegrationTestScenarioName,
			ctx.SnapshotName,
		)
		if err != nil {
			return logging.Logger.Fail(83, "Test Pipeline Run failed run: %v", err)
		}
	}

	logging.Logger.Info("Integration Test Scenario for componet %s in namespace %s OK", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)

	return nil
}
