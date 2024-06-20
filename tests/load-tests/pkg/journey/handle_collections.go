package journey

import "fmt"
import "os"
import "path/filepath"
import "encoding/json"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"

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
		pod := pod
		podLogs := f.AsKubeAdmin.CommonController.GetPodLogs(&pod)

		for file, log := range podLogs {
			err = writeToFile(dirPath, file, log)
			if err != nil {
				return fmt.Errorf("Failed to write Pod log: %v", err)
			}
		}

		if pod.Kind == "" {
			logging.Logger.Warning("Missing kind for Pod %s", pod.Name)
			pod.Kind = "Pod"
		}

		podJSON, err := json.Marshal(pod)
		if err != nil {
			return fmt.Errorf("Failed to dump Pod JSON: %v", err)
		}

		err = writeToFile(dirPath, "collected-pod-" + pod.Name + ".json", podJSON)
		if err != nil {
			return fmt.Errorf("Failed to write Pod: %v", err)
		}

	}

	return nil
}

func collectPipelineRunJSONs(f *framework.Framework, dirPath, namespace, application, component string) error {
	prs, err := f.AsKubeDeveloper.HasController.GetComponentPipelineRunsWithType(component, application, namespace, "", "")
	if err != nil {
		return fmt.Errorf("Failed to list PipelineRuns %s/%s/%s: %v", namespace, application, component, err)
	}

	for _, pr := range *prs {
		prJSON, err := json.Marshal(pr)
		if err != nil {
			return fmt.Errorf("Failed to dump PipelineRun JSON: %v", err)
		}

		err = writeToFile(dirPath, "collected-pipelinerun-" + pr.Name + ".json", prJSON)
		if err != nil {
			return fmt.Errorf("Failed to write PipelineRun: %v", err)
		}

		for _, chr := range pr.Status.ChildReferences {
			tr, err := f.AsKubeDeveloper.TektonController.GetTaskRun(chr.Name, namespace)
			if err != nil {
				return fmt.Errorf("Failed to list TaskRuns %s/%s: %v", namespace, pr.Name, err)
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

			err = writeToFile(dirPath, "collected-taskrun-" + tr.Name + ".json", trJSON)
			if err != nil {
				return fmt.Errorf("Failed to write TaskRun: %v", err)
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

	err = collectPipelineRunJSONs(ctx.Framework, dirPath, ctx.ParentContext.ParentContext.Namespace, ctx.ParentContext.ApplicationName, ctx.ComponentName)
	if err != nil {
		return logging.Logger.Fail(102, "Failed to collect pipeline run JSONs: %v", err)
	}

	return nil
}
