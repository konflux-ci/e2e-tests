package journey

import "fmt"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import utils "github.com/konflux-ci/e2e-tests/pkg/utils"

func CreateApplication(f *framework.Framework, namespace string, timeout time.Duration, name string) error {
	_, err := f.AsKubeDeveloper.HasController.CreateApplicationWithTimeout(name, namespace, timeout)
	if err != nil {
		return fmt.Errorf("Unable to create the Application %s: %v", name, err)
	}
	return nil
}

func ValidateApplication(f *framework.Framework, name, namespace string) error {
	interval := time.Second * 20
	timeout := time.Minute * 15

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		_, err = f.AsKubeDeveloper.HasController.GetApplication(name, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get application %s in namespace %s: %v", name, namespace, err)
			return false, nil
		}

		return true, nil
	}, interval, timeout)

	return err
}

func HandleApplication(ctx *PerApplicationContext) error {
	var err error

	logging.Logger.Debug("Creating application %s in namespace %s", ctx.ApplicationName, ctx.ParentContext.Namespace)

	_, err = logging.Measure(CreateApplication, ctx.Framework, ctx.ParentContext.Namespace, time.Minute*60, ctx.ApplicationName)
	if err != nil {
		return logging.Logger.Fail(30, "Application failed creation: %v", err)
	}

	_, err = logging.Measure(ValidateApplication, ctx.Framework, ctx.ApplicationName, ctx.ParentContext.Namespace)
	if err != nil {
		return logging.Logger.Fail(31, "Application failed validation: %v", err)
	}

	return nil
}
