package journey

import "fmt"
import "strings"
import "time"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import utils "github.com/redhat-appstudio/e2e-tests/pkg/utils"
import appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"

func CreateComponentDetectionQuery(f *framework.Framework, namespace string, timeout time.Duration, name, repoUrl, repoRevision string) error {
	_, err := f.AsKubeDeveloper.HasController.CreateComponentDetectionQueryWithTimeout(name, namespace, repoUrl, repoRevision, "", "", false, timeout)
	if err != nil {
		return fmt.Errorf("Unable to create the Component Detection Query %s: %v", name, err)
	}
	return nil
}

func ValidateComponentDetectionQuery(f *framework.Framework, namespace, name string) (*appstudioApi.ComponentDetectionQuery, error) {
	interval := time.Second * 20
	timeout := time.Minute * 15
	var cdq *appstudioApi.ComponentDetectionQuery

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		cdq, err = f.AsKubeDeveloper.HasController.GetComponentDetectionQuery(name, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get created Component Detection Query %s in namespace %s: %v", name, namespace, err)
			return false, nil
		}

		// Check if there are some conditions
		if len(cdq.Status.Conditions) == 0 {
			logging.Logger.Debug("Component Detection Query %s in namespace %s lacks status conditions", name, namespace)
			return false, nil
		}

		// Check right condition status
		for _, condition := range cdq.Status.Conditions {
			if (strings.HasPrefix(condition.Type, "Error") || strings.HasSuffix(condition.Type, "Error")) && condition.Status == "True" {
				return false, fmt.Errorf("Component Detection Query %s in namespace %s is in error state: %+v", name, namespace, condition)
			}
			if condition.Type == "Completed" && condition.Status == "True" {
				return true, nil
			}
		}

		// Check if CDQ detected 1 or more component stubs
		if len(cdq.Status.ComponentDetected) == 0 {
			logging.Logger.Debug("Component Detection Query %s in namespace %s did not detected any component stubs", name, namespace)
			return false, nil
		}

		logging.Logger.Trace("Still waiting for condition in component detection query %s in namespace %s", name, namespace)
		return false, nil
	}, interval, timeout)

	return cdq, err
}

func ExtractComponentStubs(cdq *appstudioApi.ComponentDetectionQuery, count int) []appstudioApi.ComponentDetectionDescription {
	// Get all the detected components
	compStubs := make([]appstudioApi.ComponentDetectionDescription, 0, count)
	for _, stub := range cdq.Status.ComponentDetected {
		compStubs = append(compStubs, stub)
	}

	// If we want more components than detected, use last one multiple times
	for i := len(compStubs); i < count; i++ {
		compStubs = append(compStubs, compStubs[len(cdq.Status.ComponentDetected)-1])
	}

	return compStubs
}

func HandleComponentDetectionQuery(ctx *PerApplicationContext) error {
	var err error

	name := fmt.Sprintf("%s-cdq", ctx.ApplicationName)
	logging.Logger.Debug("Creating component detection query %s in namespace %s", name, ctx.ParentContext.Namespace)

	_, err = logging.Measure(CreateComponentDetectionQuery, ctx.Framework, ctx.ParentContext.Namespace, time.Minute*60, name, ctx.ParentContext.Opts.ComponentRepoUrl, ctx.ParentContext.ComponentRepoRevision)
	if err != nil {
		return logging.Logger.Fail(50, "Component Detection Query failed creation: %v", err)
	}

	var cdq *appstudioApi.ComponentDetectionQuery
	var ok bool
	result1, err1 := logging.Measure(ValidateComponentDetectionQuery, ctx.Framework, ctx.ParentContext.Namespace, name)
	if err1 != nil {
		return logging.Logger.Fail(51, "Component Detection Query failed validation: %v", err1)
	}
	cdq, ok = result1.(*appstudioApi.ComponentDetectionQuery)
	if !ok {
		return logging.Logger.Fail(52, "Component Detection Query type assertion failed")
	}

	ctx.ComponentDetectionQueryName = name
	ctx.ComponentStubList = ExtractComponentStubs(cdq, ctx.ParentContext.Opts.ComponentsCount)

	return nil
}
