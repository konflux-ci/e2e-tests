package journey

import (
	"fmt"
	"time"

	logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

	framework "github.com/konflux-ci/e2e-tests/pkg/framework"
)

func purgeStage(f *framework.Framework, namespace string) error {
	var err error

	err = f.AsKubeDeveloper.HasController.DeleteAllApplicationsInASpecificNamespace(namespace, time.Minute*5)
	if err != nil {
		return fmt.Errorf("Error when deleting applications in namespace %s: %v", namespace, err)
	}

	err = f.AsKubeDeveloper.HasController.DeleteAllComponentsInASpecificNamespace(namespace, time.Minute*5)
	if err != nil {
		return fmt.Errorf("Error when deleting components in namespace %s: %v", namespace, err)
	}

	err = f.AsKubeDeveloper.HasController.DeleteAllImageRepositoriesInASpecificNamespace(namespace, time.Minute*5)
	if err != nil {
		return fmt.Errorf("Error when deleting image repositories in namespace %s: %v", namespace, err)
	}

	err = f.AsKubeDeveloper.TektonController.DeleteAllPipelineRunsInASpecificNamespace(namespace)
	if err != nil {
		return fmt.Errorf("Error when deleting pipeline runs in namespace %s: %v", namespace, err)
	}

	err = f.AsKubeDeveloper.CommonController.DeleteSecretsByLabel(namespace, "build.appstudio.redhat.com/multi-platform-secret", "true")
	if err != nil {
		return fmt.Errorf("Error when deleting MPC secrets in namespace %s: %v", namespace, err)
	}

	logging.Logger.Debug("Finished purging namespace %s", namespace)
	return nil
}

func purgeCi(f *framework.Framework, username string) error {
	err := f.AsKubeAdmin.CommonController.DeleteNamespace(f.UserNamespace)
	if err != nil {
		return fmt.Errorf("error when deleting namespace %s for user %s: %v", f.UserNamespace, username, err)
	}

	logging.Logger.Debug("Finished purging namespace %s for user %s", f.UserNamespace, username)
	return nil
}

func Purge() error {
	if !PerUserContexts[0].Opts.Purge {
		return nil
	}

	errCounter := 0

	for _, ctx := range PerUserContexts {
		if ctx.Opts.Stage {
			err := purgeStage(ctx.Framework, ctx.Namespace)
			if err != nil {
				logging.Logger.Error("Error when purging Stage: %v", err)
				errCounter++
			}
		} else {
			err := purgeCi(ctx.Framework, ctx.Username)
			if err != nil {
				logging.Logger.Error("Error when purging CI: %v", err)
				errCounter++
			}
		}
	}

	if errCounter > 0 {
		return fmt.Errorf("Hit %d errors when purging resources", errCounter)
	} else {
		logging.Logger.Info("No errors when purging resources")
		return nil
	}
}
