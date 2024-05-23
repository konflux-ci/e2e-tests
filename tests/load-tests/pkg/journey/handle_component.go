package journey

import "encoding/json"
import "fmt"
import "strings"
import "time"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import constants "github.com/redhat-appstudio/e2e-tests/pkg/constants"
import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import utils "github.com/redhat-appstudio/e2e-tests/pkg/utils"
import appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"

// Get PR URL from PaC component annotation "build.appstudio.openshift.io/status"
func getPaCPull(annotations map[string]string) (string, error) {
	var buildStatusAnn string = "build.appstudio.openshift.io/status"
	var buildStatusValue string
	var buildStatusMap map[string]interface{}

	// Get annotation we are interested in
	buildStatusValue, exists := annotations[buildStatusAnn]
	if !exists {
		return "", nil
	}

	// Parse JSON
	err := json.Unmarshal([]byte(buildStatusValue), &buildStatusMap)
	if err != nil {
		return "", fmt.Errorf("Error unmarshalling JSON:", err)
	}

	// Access the nested value using type assertion
	if pac, ok := buildStatusMap["pac"].(map[string]interface{}); ok {
		var data string
		var ok bool

		// Example: '{"pac":{"state":"enabled","merge-url":"https://github.com/rhtap-test-local/multi-platform-test-test-rhtap-1/pull/1","configuration-time":"Thu, 23 May 2024 07:06:43 UTC"},"message":"done"}'

		// Check "state" is "enabled"
		if data, ok = pac["state"].(string); ok {
			if data != "enabled" {
				return "", fmt.Errorf("Incorrect state: %s", buildStatusValue)
			}
		} else {
			return "", fmt.Errorf("Failed parsing state: %s", buildStatusValue)
		}

		// Get "merge-url"
		if data, ok = pac["merge-url"].(string); ok {
			return data, nil
		} else {
			return "", fmt.Errorf("Failed parsing state: %s", buildStatusValue)
		}
	} else {
		return "", fmt.Errorf("Failed parsing: %s", buildStatusValue)
	}
}

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

func ValidateComponent(f *framework.Framework, namespace, name string, pac bool) (string, error) {
	interval := time.Second * 20
	timeout := time.Minute * 15
	var comp *appstudioApi.Component
	var pull string

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

		// Check for right annotation
		if pac {
			pull, err = getPaCPull(comp.Annotations)
			if err != nil {
				return false, fmt.Errorf("PaC component %s in namespace %s failed on PR annotation: %v", name, namespace, err)
			}
			if pull == "" {
				logging.Logger.Debug("PaC component %s in namespace %s do not have PR yet", name, namespace)
				return false, nil
			}
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

		logging.Logger.Trace("Still waiting for condition in component %s in namespace %s", name, namespace)
		return false, nil
	}, interval, timeout)

	return "", err
}

func HandleComponent(ctx *PerComponentContext) error {
	var pullIface interface{}
	var pull string
	var err error

	stub := ctx.ParentContext.ComponentStubList[ctx.ComponentIndex]
	logging.Logger.Debug("Creating component %s in namespace %s", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)

	_, err = logging.Measure(CreateComponent, ctx.Framework, ctx.ParentContext.ParentContext.Namespace, ctx.ComponentName, ctx.ParentContext.ApplicationName, stub, ctx.ParentContext.ParentContext.Opts.PipelineSkipInitialChecks, ctx.ParentContext.ParentContext.Opts.PipelineRequestConfigurePac)
	if err != nil {
		return logging.Logger.Fail(60, "Component failed creation: %v", err)
	}

	pullIface, err = logging.Measure(ValidateComponent, ctx.Framework, ctx.ParentContext.ParentContext.Namespace, ctx.ComponentName, ctx.ParentContext.ParentContext.Opts.PipelineRequestConfigurePac)
	if err != nil {
		return logging.Logger.Fail(61, "Component failed validation: %v", err)
	}

	pull, ok := pullIface.(string)
	if !ok {
		return logging.Logger.Fail(62, "Type assertion failed on pull: %+v", pullIface)
	}
	ctx.MergeUrl = pull

	return nil
}
