package journey

import "fmt"
import "time"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import "github.com/redhat-appstudio/e2e-tests/pkg/utils"


func HandleUser(ctx *MainContext) error {
	var err error

	// TODO E.g. when token is incorrect, timeout does not work as expected
	if ctx.Opts.Stage {
		user := (*ctx.StageUsers)[ctx.ThreadIndex]
		ctx.Username = user.Username
		ctx.Framework, err = framework.NewFrameworkWithTimeout(
			ctx.Username,
			time.Minute * 60,
			utils.Options{
				ToolchainApiUrl: user.APIURL,
				KeycloakUrl:     user.SSOURL,
				OfflineToken:    user.Token,
			})
	} else {
		ctx.Username = fmt.Sprintf("%s-%04d", ctx.Opts.UsernamePrefix, ctx.ThreadIndex)
		ctx.Framework, err = framework.NewFrameworkWithTimeout(ctx.Username, time.Minute * 60)
	}

	if err != nil {
		return logging.Logger.Fail(10, "Unable to provision user %s: %v", ctx.Username, err)
	}

	ctx.Namespace = ctx.Framework.UserNamespace

	return nil
}

func HandleNewFrameworkForComp(ctx *PerComponentContext) error {
	var err error

	// TODO This framework generation code is duplicate to above
	if ctx.ParentContext.ParentContext.Opts.Stage {
		user := (*ctx.ParentContext.ParentContext.StageUsers)[ctx.ParentContext.ParentContext.ThreadIndex]
		ctx.Framework, err = framework.NewFrameworkWithTimeout(
			ctx.ParentContext.ParentContext.Username,
			time.Minute * 60,
			utils.Options{
				ToolchainApiUrl: user.APIURL,
				KeycloakUrl:     user.SSOURL,
				OfflineToken:    user.Token,
			})
	} else {
		ctx.Framework, err = framework.NewFrameworkWithTimeout(ctx.ParentContext.ParentContext.Username, time.Minute * 60)
	}

	if err != nil {
		return logging.Logger.Fail(11, "Unable to provision framework for user %s: %v", ctx.ParentContext.ParentContext.Username, err)
	}

	return nil
}

func HandleNewFrameworkForApp(ctx *PerApplicationContext) error {
	var err error

	// TODO This framework generation code is duplicate to above
	if ctx.ParentContext.Opts.Stage {
		user := (*ctx.ParentContext.StageUsers)[ctx.ParentContext.ThreadIndex]
		ctx.Framework, err = framework.NewFrameworkWithTimeout(
			ctx.ParentContext.Username,
			time.Minute * 60,
			utils.Options{
				ToolchainApiUrl: user.APIURL,
				KeycloakUrl:     user.SSOURL,
				OfflineToken:    user.Token,
			})
	} else {
		ctx.Framework, err = framework.NewFrameworkWithTimeout(ctx.ParentContext.Username, time.Minute * 60)
	}

	if err != nil {
		return logging.Logger.Fail(12, "Unable to provision framework for user %s: %v", ctx.ParentContext.Username, err)
	}

	return nil
}
