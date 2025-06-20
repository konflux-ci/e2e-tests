package journey

//import "fmt"
//import "strings"
//import "time"
//
//import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
//
//import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
//import utils "github.com/konflux-ci/e2e-tests/pkg/utils"
//import pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
//
//
//// Wait for Release CR to be created
//func validateReleaseCreation(f *framework.Framework, namespace...) error {
//}
//
//
//// Wait for release pipeline run to be created
//func validateReleasePipelineRunCreation(f *framework.Framework, namespace...) error {
//}
//
//
//// Wait for release pipeline run to succeed
//func validateReleasePipelineRunCondition(f *framework.Framework, namespace...) error {
//}
//
//
//// Wait for Release CR to have a succeeding status
//func validateReleaseCondition(f *framework.Framework, namespace...) error {
//}
//
//
//func HandleReleaseRun(ctx *PerApplicationContext) error {
//	if ctx.ParentContext.Opts.ReleasePolicy == "" || !ctx.ParentContext.Opts.WaitRelease {
//		logging.Logger.Info("Skipping wait for releases because policy was not provided or waiting for releases was disabled")
//		return nil
//	}
//
//	var err error
//
//	validateReleaseCreation
//	validateReleasePipelineRunCreation
//	validateReleasePipelineRunCondition
//	validateReleaseCondition
//
//	logging.Logger.Debug("Waiting for build pipeline run for component %s in namespace %s", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)
//
//	_, err = logging.Measure(
//		validatePipelineRunCreation,
//		ctx.Framework,
//		ctx.ParentContext.ParentContext.Namespace,
//		ctx.ParentContext.ApplicationName,
//		ctx.ComponentName,
//	)
//	if err != nil {
//		return logging.Logger.Fail(70, "Build Pipeline Run failed creation: %v", err)
//	}
//
//	logging.Logger.Debug("Build pipeline run for component %s in namespace %s created", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)
//
//	_, err = logging.Measure(
//		validatePipelineRunCondition,
//		ctx.Framework,
//		ctx.ParentContext.ParentContext.Namespace,
//		ctx.ParentContext.ApplicationName,
//		ctx.ComponentName,
//	)
//	if err != nil {
//		return logging.Logger.Fail(71, "Build Pipeline Run failed run: %v", err)
//	}
//
//	logging.Logger.Debug("Build pipeline run for component %s in namespace %s succeeded", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)
//
//	_, err = logging.Measure(
//		validatePipelineRunSignature,
//		ctx.Framework,
//		ctx.ParentContext.ParentContext.Namespace,
//		ctx.ParentContext.ApplicationName,
//		ctx.ComponentName,
//	)
//	if err != nil {
//		return logging.Logger.Fail(72, "Build Pipeline Run failed signing: %v", err)
//	}
//
//	logging.Logger.Info("Build pipeline run for component %s in namespace %s OK", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)
//
//	return nil
//}
