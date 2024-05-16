package journey

import "fmt"
import "strings"
import "time"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import util "github.com/devfile/library/v2/pkg/util"
import utils "github.com/redhat-appstudio/e2e-tests/pkg/utils"
import appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"

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
	var app *appstudioApi.Application

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		app, err = f.AsKubeDeveloper.HasController.GetApplication(name, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get application %s in namespace %s: %v", name, namespace, err)
			return false, nil
		}

		// Check if there are some conditions
		if len(app.Status.Conditions) == 0 {
			logging.Logger.Debug("Application %s in namespace %s lacks status conditions", name, namespace)
			return false, nil
		}

		// Check right condition status
		for _, condition := range app.Status.Conditions {
			if (strings.HasPrefix(condition.Type, "Error") || strings.HasSuffix(condition.Type, "Error")) && condition.Status == "True" {
				logging.Logger.Debug("Application %s in namespace %s is in error state: %+v", name, namespace, condition)
			}
			if condition.Type == "Created" && condition.Status == "True" {
				return true, nil
			}
		}

		logging.Logger.Trace("Still waiting for condition in application %s in namespace %s", name, namespace)
		return false, nil
	}, interval, timeout)

	return err
}

func HandleApplication(ctx *PerApplicationContext) error {
	var err error

	name := fmt.Sprintf("%s-app-%s", ctx.ParentContext.Username, util.GenerateRandomString(5))
	logging.Logger.Debug("Creating application %s in namespace %s", name, ctx.ParentContext.Namespace)

	_, err = logging.Measure(CreateApplication, ctx.Framework, ctx.ParentContext.Namespace, time.Minute*60, name)
	if err != nil {
		return logging.Logger.Fail(30, "Application failed creation: %v", err)
	}

	_, err = logging.Measure(ValidateApplication, ctx.Framework, name, ctx.ParentContext.Namespace)
	if err != nil {
		return logging.Logger.Fail(31, "Application failed validation: %v", err)
	}

	ctx.ApplicationName = name

	return nil
}
