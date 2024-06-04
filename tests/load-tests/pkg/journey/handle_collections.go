package journey

import "fmt"
import "os"
import "path/filepath"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"

func getDirName(baseDir, namespace, iteration string) string {
	return filepath.Join(baseDir, "collected-data", namespace, iteration) + "/"
}

func createDir(dirPath string) error {
	err := os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("Failed to create directory %s: %v", dirPath, err)
	}

	return nil
}

func writeToFile(dirPath, file string, contents []byte) error {
	fileName := filepath.Join(dirPath, file)

	fd, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("Failed to open file %s: %v", fileName, err)
	}

	_, err = fd.Write(contents)
	if err != nil {
		return fmt.Errorf("Failed to write to file %s: %v", fileName, err)
	}

	defer fd.Close()

	return nil
}

func collectPodLogs(f *framework.Framework, dirPath, namespace, component string) error {
	podList, err := f.AsKubeAdmin.CommonController.ListPods(
		namespace,
		"appstudio.openshift.io/component",
		component,
		100,
	)
	if err != nil {
		return fmt.Errorf("Failed to list pods in namespace %s for component %s: %v", namespace, component, err)
	}

	for _, pod := range podList.Items {
		podLogs := f.AsKubeAdmin.CommonController.GetPodLogs(&pod)

		for file, log := range podLogs {
			err = writeToFile(dirPath, file, log)
			if err != nil {
				return fmt.Errorf("Failed to write log: %v", err)
			}
		}
	}

	return nil
}

func HandlePerComponentCollection(ctx *PerComponentContext) error {
	if ctx.ComponentName == "" {
		logging.Logger.Debug("Component name not populated, so skipping per-component collections in %s", ctx.ParentContext.ParentContext.Namespace)
		return nil
	}

	var err error

	journeyCounterStr := fmt.Sprintf("%d", ctx.ParentContext.ParentContext.JourneyRepeatsCounter)
	dirPath := getDirName(ctx.ParentContext.ParentContext.Opts.OutputDir, ctx.ParentContext.ParentContext.Namespace, journeyCounterStr)
	err = createDir(dirPath)
	if err != nil {
		return logging.Logger.Fail(100, "Failed to create dir: %v", err)
	}

	err = collectPodLogs(ctx.Framework, dirPath, ctx.ParentContext.ParentContext.Namespace, ctx.ComponentName)
	if err != nil {
		return logging.Logger.Fail(101, "Failed to collect pod logs: %v", err)
	}

	return nil
}
