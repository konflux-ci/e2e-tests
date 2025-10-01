package journey

import "fmt"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
import types "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/types"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import utils "github.com/konflux-ci/e2e-tests/pkg/utils"

import util "github.com/devfile/library/v2/pkg/util"

func createApplication(f *framework.Framework, namespace string, runPrefix string) (string, error) {
	name := fmt.Sprintf("%s-app-%s", runPrefix, util.GenerateRandomString(5))
	_, err := f.AsKubeDeveloper.HasController.CreateApplicationWithTimeout(name, namespace, time.Minute*60)
	if err != nil {
		return "", fmt.Errorf("Unable to create the Application %s: %v", name, err)
	}
	return name, nil
}

func validateApplication(f *framework.Framework, name, namespace string) error {
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

func HandleApplication(ctx *types.PerApplicationContext) error {
	if ctx.ApplicationName != "" {
		logging.Logger.Debug("Skipping application creation because reusing application %s in namespace %s", ctx.ApplicationName, ctx.ParentContext.Namespace)
		return nil
	}

	var iface interface{}
	var err error
	var ok bool

	logging.Logger.Debug("Creating application %s in namespace %s", ctx.ApplicationName, ctx.ParentContext.Namespace)

	iface, err = logging.Measure(
		ctx,
		createApplication,
		ctx.Framework,
		ctx.ParentContext.Namespace,
		ctx.ParentContext.Opts.RunPrefix,
	)
	if err != nil {
		return logging.Logger.Fail(30, "Application failed creation: %v", err)
	}

	ctx.ApplicationName, ok = iface.(string)
	if !ok {
		return logging.Logger.Fail(31, "Type assertion failed on application name: %+v", iface)
	}

	_, err = logging.Measure(
		ctx,
		validateApplication,
		ctx.Framework,
		ctx.ApplicationName,
		ctx.ParentContext.Namespace,
	)
	if err != nil {
		return logging.Logger.Fail(32, "Application failed validation: %v", err)
	}

	return nil
}
