package journey

import "fmt"
import "strings"
import "time"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import constants "github.com/redhat-appstudio/e2e-tests/pkg/constants"
import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import utils "github.com/redhat-appstudio/e2e-tests/pkg/utils"
import appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"


func CreateComponent(f *framework.Framework, namespace, name, appName string, stub appstudioApi.ComponentDetectionDescription, skipInitialChecks, requestConfigurePac bool) error {
	// Prepare annotations to add
	var annotationsMap map[string]string
	if requestConfigurePac {
		annotationsMap = constants.ComponentPaCRequestAnnotation
	} else {
		annotationsMap = map[string]string{}
	}

	stub.ComponentStub.ComponentName = name
	_, err := f.AsKubeDeveloper.HasController.CreateComponent(stub.ComponentStub, namespace, "", "", appName, skipInitialChecks, annotationsMap)
	if err != nil {
		return fmt.Errorf("Unable to create the Component %s: %v", name, err)
	}
	return nil
}

func ValidateComponent(f *framework.Framework, namespace, name string) error {
	interval := time.Second * 20
	timeout := time.Minute * 15
	var comp *appstudioApi.Component

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		comp, err = f.AsKubeDeveloper.HasController.GetComponent(name, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get created Component %s in namespace %s: %v", name, namespace, err)
			return false, nil
		}

		// Check if there are some conditions
		if len(comp.Status.Conditions) == 0 {
			logging.Logger.Debug("Component %s in namespace %s lacks status conditions", name, namespace)
			return false, nil
		}

		// Check right condition status
		for _, condition := range comp.Status.Conditions {
			if (strings.HasPrefix(condition.Type, "Error") || strings.HasSuffix(condition.Type, "Error")) && condition.Status == "True" {
				return false, fmt.Errorf("Component Detection Query %s in namespace %s is in error state: %+v", name, namespace, condition)
			}
			if condition.Type == "Created" && condition.Status == "True" {
				return true, nil
			}
		}

		logging.Logger.Debug("Still waiting for condition in component %s in namespace %s", name, namespace)
		return false, nil
	}, interval, timeout)

	return err
}

func HandleComponent(ctx *PerComponentContext) error {
	var err error

	name := fmt.Sprintf("%s-comp-%d", ctx.ParentContext.ApplicationName, ctx.ComponentIndex)
	stub := ctx.ParentContext.ComponentStubList[ctx.ComponentIndex]
	logging.Logger.Debug("Creating component %s in namespace %s", name, ctx.ParentContext.ParentContext.Namespace)

	_, err = logging.Measure(CreateComponent, ctx.Framework, ctx.ParentContext.ParentContext.Namespace, name, ctx.ParentContext.ApplicationName, stub, ctx.ParentContext.ParentContext.Opts.PipelineSkipInitialChecks, ctx.ParentContext.ParentContext.Opts.PipelineRequestConfigurePac)
	if err != nil {
		return logging.Logger.Fail(60, "Component failed creation: %v", err)
	}

	_, err = logging.Measure(ValidateComponent, ctx.Framework, ctx.ParentContext.ParentContext.Namespace, name)
	if err != nil {
		return logging.Logger.Fail(61, "Component failed validation: %v", err)
	}

	ctx.ComponentName = name

	return nil
}
