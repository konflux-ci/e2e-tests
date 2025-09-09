package journey

import "fmt"
import "time"
import "strings"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
import loadtestutils "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/loadtestutils"

import "github.com/konflux-ci/e2e-tests/pkg/framework"
import "github.com/konflux-ci/e2e-tests/pkg/utils"

// Returns framework, namespace (and error)
func provisionFramework(stageUsers []loadtestutils.User, threadIndex int, username string, isStage bool) (*framework.Framework, string, error) {
	var f *framework.Framework
	var err error

	if isStage {
		user := stageUsers[threadIndex]
		f, err = framework.NewFrameworkWithTimeout(
			username,
			time.Minute*60,
			utils.Options{
				ApiUrl: user.APIURL,
				Token:  user.Token,
			})
	} else {
		f, err = framework.NewFrameworkWithTimeout(username, time.Minute*60)
	}

	if err != nil {
		return nil, "", err
	}

	return f, f.UserNamespace, nil
}

func HandleUser(ctx *MainContext) error {
	var err error

	if ctx.Opts.Stage {
		ctx.Username = strings.TrimSuffix((*ctx.StageUsers)[ctx.ThreadIndex].Namespace, "-tenant")
	} else {
		ctx.Username = fmt.Sprintf("%s-%04d", ctx.Opts.RunPrefix, ctx.ThreadIndex)
	}

	ctx.Framework, ctx.Namespace, err = provisionFramework(
		*ctx.StageUsers,
		ctx.ThreadIndex,
		ctx.Username,
		ctx.Opts.Stage,
	)
	if err != nil {
		return logging.Logger.Fail(10, "Unable to provision user %s: %v", ctx.Username, err)
	}

	return nil
}

func HandleNewFrameworkForApp(ctx *PerApplicationContext) error {
	var err error

	ctx.Framework, _, err = provisionFramework(
		*ctx.ParentContext.StageUsers,
		ctx.ParentContext.ThreadIndex,
		ctx.ParentContext.Username,
		ctx.ParentContext.Opts.Stage,
	)
	if err != nil {
		return logging.Logger.Fail(11, "Unable to provision framework for user %s: %v", ctx.ParentContext.Username, err)
	}

	return nil
}

func HandleNewFrameworkForComp(ctx *PerComponentContext) error {
	var err error

	ctx.Framework, _, err = provisionFramework(
		*ctx.ParentContext.ParentContext.StageUsers,
		ctx.ParentContext.ParentContext.ThreadIndex,
		ctx.ParentContext.ParentContext.Username,
		ctx.ParentContext.ParentContext.Opts.Stage,
	)
	if err != nil {
		return logging.Logger.Fail(12, "Unable to provision framework for user %s: %v", ctx.ParentContext.ParentContext.Username, err)
	}

	return nil
}
