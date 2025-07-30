package journey

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

	framework "github.com/konflux-ci/e2e-tests/pkg/framework"
)

func getDirName(baseDir, namespace, iteration string) string {
	return filepath.Join(baseDir, "collected-data", namespace, iteration) + "/"
}

func createDir(dirPath string) error {
	err := os.MkdirAll(dirPath, 0750)
	if err != nil {
		return fmt.Errorf("Failed to create directory %s: %v", dirPath, err)
	}

	return nil
}

func writeToFile(dirPath, file string, contents []byte) error {
	fileName := filepath.Join(dirPath, file)
	fileName = filepath.Clean(fileName)

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

func collectPodLogs(f *framework.Framework, dirPath, namespace, application string) error {
	podList, err := f.AsKubeAdmin.CommonController.ListPods(
		namespace,
		"appstudio.openshift.io/application",
		application,
		100,
	)
	if err != nil {
		return fmt.Errorf("Failed to list pods in namespace %s for application %s: %v", namespace, application, err)
	}

	for _, pod := range podList.Items {
		pod := pod
		podLogs := f.AsKubeAdmin.CommonController.GetPodLogs(&pod)

		for file, log := range podLogs {
			err = writeToFile(dirPath, file, log)
			if err != nil {
				return fmt.Errorf("Failed to write Pod log: %v", err)
			}
		}

		if pod.Kind == "" {
			// This is known issue:
			// https://github.com/kubernetes/client-go/issues/861#issuecomment-686766515
			pod.Kind = "Pod"
		}

		podJSON, err := json.Marshal(pod)
		if err != nil {
			return fmt.Errorf("Failed to dump Pod JSON: %v", err)
		}

		err = writeToFile(dirPath, "collected-pod-"+pod.Name+".json", podJSON)
		if err != nil {
			return fmt.Errorf("Failed to write Pod: %v", err)
		}

	}

	return nil
}

func collectPipelineRunJSONs(f *framework.Framework, dirPath, namespace, application, component, release string) error {
	prs, err := f.AsKubeDeveloper.HasController.GetComponentPipelineRunsWithType(component, application, namespace, "", "", "")
	if err != nil {
		return fmt.Errorf("Failed to list PipelineRuns %s/%s/%s: %v", namespace, application, component, err)
	}

	pr_release, err := f.AsKubeDeveloper.ReleaseController.GetPipelineRunInNamespace(namespace, release, namespace)
	if err != nil {
		logging.Logger.Warning("Failed to get Release PipelineRun %s/%s: %v", namespace, release, err)
	}

	// Make one list that contains them all
	if pr_release != nil {
		*prs = append(*prs, *pr_release)
	}

	for _, pr := range *prs {
		prJSON, err := json.Marshal(pr)
		if err != nil {
			return fmt.Errorf("Failed to dump PipelineRun JSON: %v", err)
		}

		err = writeToFile(dirPath, "collected-pipelinerun-"+pr.Name+".json", prJSON)
		if err != nil {
			return fmt.Errorf("Failed to write PipelineRun: %v", err)
		}

		for _, chr := range pr.Status.ChildReferences {
			tr, err := f.AsKubeDeveloper.TektonController.GetTaskRun(chr.Name, namespace)
			if err != nil {
				return fmt.Errorf("Failed to list TaskRuns %s/%s: %v", namespace, pr.Name, err)
			}

			if tr.Kind == "" {
				tr.Kind = tr.GetGroupVersionKind().Kind
			}
			if tr.Kind == "" {
				logging.Logger.Warning("Missing kind for TaskRun %s", tr.Name)
				tr.Kind = "TaskRun"
			}

			var trJSON []byte
			trJSON, err = json.Marshal(tr)
			if err != nil {
				return fmt.Errorf("Failed to dump TaskRun JSON: %v", err)
			}

			err = writeToFile(dirPath, "collected-taskrun-"+tr.Name+".json", trJSON)
			if err != nil {
				return fmt.Errorf("Failed to write TaskRun: %v", err)
			}
		}
	}

	return nil
}

func collectApplicationJSONs(f *framework.Framework, dirPath, namespace, application string) error {
	appJsonFileName := "collected-application-" + application + ".json"
	// Only save Application JSON if it has not already been collected (as HandlePerComponentCollection method is called for each component)
	if _, err := os.Stat(filepath.Join(dirPath, appJsonFileName)); errors.Is(err, os.ErrNotExist) {
		// Get Application JSON
		app, err := f.AsKubeDeveloper.HasController.GetApplication(application, namespace)
		if err != nil {
			return fmt.Errorf("Failed to get Application %s: %v", application, err)
		}

		appJSON, err := json.Marshal(app)
		if err != nil {
			return fmt.Errorf("Failed to dump Application JSON: %v", err)
		}

		err = writeToFile(dirPath, appJsonFileName, appJSON)
		if err != nil {
			return fmt.Errorf("Failed to write Application: %v", err)
		}
	}

	return nil
}

func collectComponentJSONs(f *framework.Framework, dirPath, namespace, component string) error {
	// Collect Component JSON
	comp, err := f.AsKubeDeveloper.HasController.GetComponent(component, namespace)
	if err != nil {
		return fmt.Errorf("Failed to get Component %s: %v", component, err)
	}

	compJSON, err := json.Marshal(comp)
	if err != nil {
		return fmt.Errorf("Failed to dump Component JSON: %v", err)
	}

	err = writeToFile(dirPath, "collected-component-"+component+".json", compJSON)
	if err != nil {
		return fmt.Errorf("Failed to write Component: %v", err)
	}

	return nil
}

func HandlePerApplicationCollection(ctx *PerApplicationContext) error {
	if ctx.ApplicationName == "" {
		logging.Logger.Debug("Application name not populated, so skipping per-application collections in %s", ctx.ParentContext.Namespace)
		return nil
	}

	var err error

	journeyCounterStr := fmt.Sprintf("%d", ctx.ParentContext.JourneyRepeatsCounter)
	dirPath := getDirName(ctx.ParentContext.Opts.OutputDir, ctx.ParentContext.Namespace, journeyCounterStr)
	err = createDir(dirPath)
	if err != nil {
		return logging.Logger.Fail(105, "Failed to create dir: %v", err)
	}

	err = collectPodLogs(ctx.Framework, dirPath, ctx.ParentContext.Namespace, ctx.ApplicationName)
	if err != nil {
		return logging.Logger.Fail(106, "Failed to collect pod logs: %v", err)
	}

	err = collectApplicationJSONs(ctx.Framework, dirPath, ctx.ParentContext.Namespace, ctx.ApplicationName)
	if err != nil {
		return logging.Logger.Fail(107, "Failed to collect application JSONs: %v", err)
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

	err = collectPipelineRunJSONs(ctx.Framework, dirPath, ctx.ParentContext.ParentContext.Namespace, ctx.ParentContext.ApplicationName, ctx.ComponentName, ctx.ReleaseName)
	if err != nil {
		return logging.Logger.Fail(102, "Failed to collect pipeline run JSONs: %v", err)
	}

	err = collectComponentJSONs(ctx.Framework, dirPath, ctx.ParentContext.ParentContext.Namespace, ctx.ComponentName)
	if err != nil {
		return logging.Logger.Fail(103, "Failed to collect component JSONs: %v", err)
	}

	return nil
}
