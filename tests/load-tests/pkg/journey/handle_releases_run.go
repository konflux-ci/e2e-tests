package journey

import "fmt"
import "strings"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import utils "github.com/konflux-ci/e2e-tests/pkg/utils"


// Wait for Release CR to be created
func validateReleaseCreation(f *framework.Framework, namespace, snapshotName string) (string, error) {
	logging.Logger.Debug("Waiting for release for snapshot %s in namespace %s to be created", snapshotName, namespace)

	var releaseName string

	interval := time.Second * 10
	timeout := time.Minute * 5

	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		release, err := f.AsKubeDeveloper.ReleaseController.GetRelease("", snapshotName, namespace)
		if err != nil {
			fmt.Printf("Can not get release for snapshot %s in namespace %s: %v\n", snapshotName, namespace, err)
			return false, nil
		}

		releaseName = release.Name

		return true, nil
	}, interval, timeout)

	return releaseName, err
}


// Wait for release pipeline run to be created
func validateReleasePipelineRunCreation(f *framework.Framework, namespace, releaseName string) error {
	logging.Logger.Debug("Waiting for release pipeline for release %s in namespace %s to be created", releaseName, namespace)

	interval := time.Second * 10
	timeout := time.Minute * 5

	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		_, err = f.AsKubeDeveloper.ReleaseController.GetPipelineRunInNamespace(namespace, releaseName, namespace)
		if err != nil {
			fmt.Printf("Pipelinerun for release %s in namespace %s not created yet: %v\n", releaseName, namespace, err)
			return false, nil
		}

		return true, nil
	}, interval, timeout)

	return err
}


// Wait for release pipeline run to succeed
func validateReleasePipelineRunCondition(f *framework.Framework, namespace, releaseName string) error {
	logging.Logger.Debug("Waiting for release pipeline for release %s in namespace %s to finish", releaseName, namespace)

	interval := time.Second * 10
	timeout := time.Minute * 10

	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		pipelineRun, err := f.AsKubeDeveloper.ReleaseController.GetPipelineRunInNamespace(namespace, releaseName, namespace)
		if err != nil {
			fmt.Printf("PipelineRun for release %s in namespace %s not created yet: %v\n", releaseName, namespace, err)
			return false, nil
		}

		// Check if there are some conditions
		if len(pipelineRun.Status.Conditions) == 0 {
			fmt.Printf("PipelineRun %s in namespace %s lacks status conditions\n", pipelineRun.GetName(), pipelineRun.GetNamespace())
			return false, nil
		}

		// Check right condition status
		for _, condition := range pipelineRun.Status.Conditions {
			if (strings.HasPrefix(string(condition.Type), "Error") || strings.HasSuffix(string(condition.Type), "Error")) && condition.Status == "True" {
				return false, fmt.Errorf("PipelineRun %s in namespace %s is in error state: %+v", pipelineRun.GetName(), pipelineRun.GetNamespace(), condition)
			}
			if condition.Type == "Succeeded" && condition.Status == "False" {
				return false, fmt.Errorf("PipelineRun %s in namespace %s failed: %+v", pipelineRun.GetName(), pipelineRun.GetNamespace(), condition)
			}
			if condition.Type == "Succeeded" && condition.Status == "True" {
				return true, nil
			}
		}

		return false, nil
	}, interval, timeout)

	return err
}


// Wait for Release CR to have a succeeding status
func validateReleaseCondition(f *framework.Framework, namespace, releaseName string) error {
	logging.Logger.Debug("Waiting for release %s in namespace %s to finish", releaseName, namespace)

	interval := time.Second * 10
	timeout := time.Minute * 5

	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		release, err := f.AsKubeDeveloper.ReleaseController.GetRelease(releaseName, "", namespace)
		if err != nil {
			fmt.Printf("Can not get release %s in namespace %s: %v\n", releaseName, namespace, err)
			return false, nil
		}

		// Check if there are some conditions
		if len(release.Status.Conditions) == 0 {
			fmt.Printf("Release %s in namespace %s lacks status conditions\n", releaseName, namespace)
			return false, nil
		}

		// Check right condition status
		for _, condition := range release.Status.Conditions {
			if condition.Type == "Released" && condition.Status == "False" {
				return false, fmt.Errorf("Release %s in namespace %s failed: %+v", releaseName, namespace, condition)
			}
			if condition.Type == "Released" && condition.Status == "True" {
				return true, nil
			}
		}

		return false, nil
	}, interval, timeout)

	return err
}


func HandleReleaseRun(ctx *PerComponentContext) error {
	if ctx.ParentContext.ParentContext.Opts.ReleasePolicy == "" || !ctx.ParentContext.ParentContext.Opts.WaitRelease {
		logging.Logger.Info("Skipping waiting for releases because policy was not provided or waiting was disabled")
		return nil
	}

	var releaseName string
	var iface interface{}
	var ok bool
	var err error

	iface, err = logging.Measure(
		validateReleaseCreation,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		ctx.SnapshotName,
	)
	if err != nil {
		return logging.Logger.Fail(90, "Release failed creation: %v", err)
	}

	releaseName, ok = iface.(string)
	if !ok {
		return logging.Logger.Fail(91, "Type assertion failed on release name: %+v", iface)
	}

	_, err = logging.Measure(
		validateReleasePipelineRunCreation,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		releaseName,
	)
	if err != nil {
		return logging.Logger.Fail(92, "Release pipeline run failed creation: %v", err)
	}

	_, err = logging.Measure(
		validateReleasePipelineRunCondition,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		releaseName,
	)
	if err != nil {
		return logging.Logger.Fail(93, "Release pipeline run failed: %v", err)
	}

	_, err = logging.Measure(
		validateReleaseCondition,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		releaseName,
	)
	if err != nil {
		return logging.Logger.Fail(94, "Release failed: %v", err)
	}

	logging.Logger.Info("Release %s in namespace %s succeeded", releaseName, ctx.ParentContext.ParentContext.Namespace)

	return nil
}
