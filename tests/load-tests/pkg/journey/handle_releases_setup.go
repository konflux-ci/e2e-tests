package journey

import "fmt"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
import types "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/types"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import meta "k8s.io/apimachinery/pkg/api/meta"
import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
import releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
import tektonutils "github.com/konflux-ci/release-service/tekton/utils"
import utils "github.com/konflux-ci/e2e-tests/pkg/utils"

// Create ReleasePlan CR
func createReleasePlan(f *framework.Framework, namespace, appName string) (string, error) {
	name := appName + "-rp"
	logging.Logger.Debug("Creating release plan %s in namespace %s", name, namespace)

	_, err := f.AsKubeDeveloper.ReleaseController.CreateReleasePlan(name, namespace, appName, namespace, "true", nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("Unable to create the ReleasePlan %s in %s: %v", name, namespace, err)
	}

	return name, nil
}


// Create ReleasePlanAdmission CR
// Assumes enterprise contract policy and service account with required permissions is already there
func createReleasePlanAdmission(f *framework.Framework, namespace, appName, policyName, releasePipelineSAName, releasePipelineUrl, releasePipelineRevision, releasePipelinePath string) (string, error) {
	name := appName + "-rpa"
	logging.Logger.Debug("Creating release plan admission %s in namespace %s with policy %s and pipeline SA %s", name, namespace, policyName, releasePipelineSAName)

	pipeline := &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasePipelineUrl},
			{Name: "revision", Value: releasePipelineRevision},
			{Name: "pathInRepo", Value: releasePipelinePath},
		},
	}
	// CreateReleasePlanAdmission(name, namespace, environment, origin, policy, serviceAccountName string, applications []string, autoRelease bool, pipelineRef *tektonutils.PipelineRef, data *runtime.RawExtension)
	_, err := f.AsKubeDeveloper.ReleaseController.CreateReleasePlanAdmission(name, namespace, "", namespace, policyName, releasePipelineSAName, []string{appName}, true, pipeline, nil)
	if err != nil {
		return "", fmt.Errorf("Unable to create the ReleasePlanAdmission %s in %s: %v", name, namespace, err)
	}

	return name, nil
}


// Wait for ReleasePlan CR to be created and to have status "Matched"
func validateReleasePlan(f *framework.Framework, namespace, name string) error {
	logging.Logger.Debug("Validating release plan %s in namespace %s", name, namespace)

	interval := time.Second * 10
	timeout := time.Minute * 5

	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		releasePlan, err := f.AsKubeDeveloper.ReleaseController.GetReleasePlan(name, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get ReleasePlan %s in %s: %v\n", name, namespace, err)
			return false, nil
		}

		condition := meta.FindStatusCondition(releasePlan.Status.Conditions, releaseApi.MatchedConditionType.String())
		if condition == nil {
			logging.Logger.Debug("MatchedConditon of %s is still not set\n", releasePlan.Name)
			return false, nil
		}
		// it may need a period of time for the ReleasePlanCR to be reconciled
		if condition.Status == metav1.ConditionFalse {
			logging.Logger.Debug("MatchedConditon of %s has not reconciled yet\n", releasePlan.Name)
			return false, nil
		}
		if condition.Status != metav1.ConditionTrue {
			logging.Logger.Debug("MatchedConditon of %s is not true yet\n", releasePlan.Name)
			return false, nil
		}
		if condition.Reason == releaseApi.MatchedReason.String() {
			return true, nil
		}

		return false, fmt.Errorf("MatchedConditon of %s incorrect: %v", releasePlan.Name, condition)
	}, interval, timeout)

	return err
}


// Wait for ReleasePlanAdmission CR to be created and to have status "Matched"
func validateReleasePlanAdmission(f *framework.Framework, namespace, name string) error {
	logging.Logger.Debug("Validating release plan admission %s in namespace %s", name, namespace)

	interval := time.Second * 10
	timeout := time.Minute * 5

	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		releasePlanAdmission, err := f.AsKubeDeveloper.ReleaseController.GetReleasePlanAdmission(name, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get ReleasePlanAdmission %s in %s: %v\n", name, namespace, err)
			return false, nil
		}

		condition := meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, releaseApi.MatchedConditionType.String())
		if condition == nil {
			logging.Logger.Debug("MatchedConditon of %s is still not set\n", releasePlanAdmission.Name)
			return false, nil
		}
		// it may need a period of time for the ReleasePlanCR to be reconciled
		if condition.Status == metav1.ConditionFalse {
			logging.Logger.Debug("MatchedConditon of %s has not reconciled yet\n", releasePlanAdmission.Name)
			return false, nil
		}
		if condition.Status != metav1.ConditionTrue {
			logging.Logger.Debug("MatchedConditon of %s is not true yet\n", releasePlanAdmission.Name)
			return false, nil
		}
		if condition.Reason == releaseApi.MatchedReason.String() {
			return true, nil
		}

		return false, fmt.Errorf("MatchedConditon of %s incorrect: %v", releasePlanAdmission.Name, condition)
	}, interval, timeout)

	return err
}


func HandleReleaseSetup(ctx *types.PerApplicationContext) error {
	if ctx.ParentContext.Opts.ReleasePolicy == "" {
		logging.Logger.Info("Skipping setting up releases because policy was not provided")
		return nil
	}

	var releasePlanName string
	var releasePlanAdmissionName string
	var iface interface{}
	var ok bool
	var err error

	iface, err = logging.Measure(
		ctx,
		createReleasePlan,
		ctx.Framework,
		ctx.ParentContext.Namespace,
		ctx.ApplicationName,
	)
	if err != nil {
		return logging.Logger.Fail(91, "Release Plan failed creation: %v", err)
	}

	releasePlanName, ok = iface.(string)
	if !ok {
		return logging.Logger.Fail(92, "Type assertion failed on release plan name: %+v", iface)
	}

	iface, err = logging.Measure(
		ctx,
		createReleasePlanAdmission,
		ctx.Framework,
		ctx.ParentContext.Namespace,
		ctx.ApplicationName,
		ctx.ParentContext.Opts.ReleasePolicy,
		ctx.ParentContext.Opts.ReleasePipelineServiceAccount,
		ctx.ParentContext.Opts.ReleasePipelineUrl,
		ctx.ParentContext.Opts.ReleasePipelineRevision,
		ctx.ParentContext.Opts.ReleasePipelinePath,
	)
	if err != nil {
		return logging.Logger.Fail(93, "Release Plan Admission failed creation: %v", err)
	}

	releasePlanAdmissionName, ok = iface.(string)
	if !ok {
		return logging.Logger.Fail(94, "Type assertion failed on release plan admission name: %+v", iface)
	}

	iface, err = logging.Measure(
		ctx,
		validateReleasePlan,
		ctx.Framework,
		ctx.ParentContext.Namespace,
		releasePlanName,
	)
	if err != nil {
		return logging.Logger.Fail(95, "Release Plan failed validation: %v", err)
	}

	iface, err = logging.Measure(
		ctx,
		validateReleasePlanAdmission,
		ctx.Framework,
		ctx.ParentContext.Namespace,
		releasePlanAdmissionName,
	)
	if err != nil {
		return logging.Logger.Fail(96, "Release Plan Admission failed validation: %v", err)
	}


	logging.Logger.Info("Configured release %s & %s for application %s in namespace %s", releasePlanName, releasePlanAdmissionName, ctx.ApplicationName, ctx.ParentContext.Namespace)

	return nil
}
