package journey

import "fmt"
import "strings"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import utils "github.com/konflux-ci/e2e-tests/pkg/utils"
import pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

func validatePipelineRunCreation(f *framework.Framework, namespace, appName, compName string) error {
	interval := time.Second * 20
	timeout := time.Minute * 30

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		_, err = f.AsKubeDeveloper.HasController.GetComponentPipelineRunWithType(compName, appName, namespace, "build", "")
		if err != nil {
			logging.Logger.Debug("Unable to get created PipelineRun for component %s in namespace %s: %v", compName, namespace, err)
			return false, nil
		}
		return true, nil
	}, interval, timeout)

	return err
}

func validatePipelineRunCondition(f *framework.Framework, namespace, appName, compName string) error {
	interval := time.Second * 20
	timeout := time.Minute * 60
	var pr *pipeline.PipelineRun

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		pr, err = f.AsKubeDeveloper.HasController.GetComponentPipelineRunWithType(compName, appName, namespace, "build", "")
		if err != nil {
			logging.Logger.Debug("Unable to get created PipelineRun for component %s in namespace %s: %v", compName, namespace, err)
			return false, nil
		}

		// Check if there are some conditions
		if len(pr.Status.Conditions) == 0 {
			logging.Logger.Debug("PipelineRun for component %s in namespace %s lacks status conditions", compName, namespace)
			return false, nil
		}

		// Check right condition status
		for _, condition := range pr.Status.Conditions {
			if (strings.HasPrefix(string(condition.Type), "Error") || strings.HasSuffix(string(condition.Type), "Error")) && condition.Status == "True" {
				return false, fmt.Errorf("PipelineRun for component %s in namespace %s is in error state: %+v", compName, namespace, condition)
			}
			if condition.Type == "Succeeded" && condition.Status == "True" {
				return true, nil
			}
		}

		logging.Logger.Trace("Still waiting for pipeline run condition for component %s in namespace %s", compName, namespace)
		return false, nil
	}, interval, timeout)

	return err
}

func validatePipelineRunSignature(f *framework.Framework, namespace, appName, compName string) error {
	interval := time.Second * 20
	timeout := time.Minute * 60
	var pr *pipeline.PipelineRun

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		pr, err = f.AsKubeDeveloper.HasController.GetComponentPipelineRunWithType(compName, appName, namespace, "build", "")
		if err != nil {
			logging.Logger.Debug("Unable to get created PipelineRun for component %s in namespace %s: %v", compName, namespace, err)
			return false, nil
		}

		// Check if there are some annotations
		if len(pr.Annotations) == 0 {
			logging.Logger.Debug("PipelineRun for component %s in namespace %s lacks metadata annotations", compName, namespace)
			return false, nil
		}

		// Check for right annotation
		if _, exists := pr.Annotations["chains.tekton.dev/signed"]; exists {
			if pr.Annotations["chains.tekton.dev/signed"] == "true" {
				return true, nil
			} else {
				logging.Logger.Debug("PipelineRun for component %s in namespace %s still not signed", compName, namespace)
				return false, nil
			}
		} else {
			logging.Logger.Debug("PipelineRun for component %s in namespace %s do not have 'chains.tekton.dev/signed' annotation", compName, namespace)
			return false, nil
		}
	}, interval, timeout)

	return err
}

func HandlePipelineRun(ctx *PerComponentContext) error {
	if !ctx.ParentContext.ParentContext.Opts.WaitPipelines {
		return nil
	}

	var err error

	logging.Logger.Debug("Creating build pipeline run for component %s in namespace %s", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)

	_, err = logging.Measure(
		validatePipelineRunCreation,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		ctx.ParentContext.ApplicationName,
		ctx.ComponentName,
	)
	if err != nil {
		return logging.Logger.Fail(70, "Build Pipeline Run failed creation: %v", err)
	}

	_, err = logging.Measure(
		validatePipelineRunCondition,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		ctx.ParentContext.ApplicationName,
		ctx.ComponentName,
	)
	if err != nil {
		return logging.Logger.Fail(71, "Build Pipeline Run failed run: %v", err)
	}

	_, err = logging.Measure(
		validatePipelineRunSignature,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		ctx.ParentContext.ApplicationName,
		ctx.ComponentName,
	)
	if err != nil {
		return logging.Logger.Fail(72, "Build Pipeline Run failed signing: %v", err)
	}

	return nil
}
