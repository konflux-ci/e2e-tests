package build

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/types"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var taskNames = []string{"clair-scan", "clamav-scan", "deprecated-base-image-check", "inspect-image", "sbom-json-check"}

type TestOutput struct {
	Result    string `json:"result"`
	Timestamp string `json:"timestamp"`
	Note      string `json:"note"`
	Namespace string `json:"namespace"`
	Successes int    `json:"successes"`
	Failures  int    `json:"failures"`
	Warnings  int    `json:"warnings"`
}

type ClairScanResult struct {
	Vulnerabilities Vulnerabilities `json:"vulnerabilities"`
}

type Vulnerabilities struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

type PipelineBuildInfo struct {
	runtime  string
	strategy string
}

func GetPipelineBuildInfo(pr *pipeline.PipelineRun) PipelineBuildInfo {
	labels := pr.GetLabels()
	runtime := labels["pipelines.openshift.io/runtime"]
	strategy := labels["pipelines.openshift.io/strategy"]
	return PipelineBuildInfo{
		runtime:  runtime,
		strategy: strategy,
	}
}

func IsDockerBuild(pr *pipeline.PipelineRun) bool {
	info := GetPipelineBuildInfo(pr)
	return info.runtime == "generic" && info.strategy == "docker"
}

func IsFBCBuild(pr *pipeline.PipelineRun) bool {
	info := GetPipelineBuildInfo(pr)
	return info.runtime == "fbc" && info.strategy == "fbc"
}

func ValidateBuildPipelineTestResults(pipelineRun *pipeline.PipelineRun, c crclient.Client) error {
	for _, taskName := range taskNames {
		// The inspect-image task is only required for FBC pipelines which we can infer by the component name
		isFBCBuild := IsFBCBuild(pipelineRun)

		if !isFBCBuild && taskName == "inspect-image" {
			continue
		}
		if isFBCBuild && (taskName == "clair-scan" || taskName == "clamav-scan") {
			continue
		}
		results, err := fetchTaskRunResults(c, pipelineRun, taskName)
		if err != nil {
			return err
		}

		resultsToValidate := []string{constants.TektonTaskTestOutputName}

		switch taskName {
		case "clair-scan":
			resultsToValidate = append(resultsToValidate, "CLAIR_SCAN_RESULT")
		case "deprecated-image-check":
			resultsToValidate = append(resultsToValidate, "PYXIS_HTTP_CODE")
		case "inspect-image":
			resultsToValidate = append(resultsToValidate, "BASE_IMAGE", "BASE_IMAGE_REPOSITORY")
		}

		if err := validateTaskRunResult(results, resultsToValidate, taskName); err != nil {
			return err
		}

	}
	return nil
}

func fetchTaskRunResults(c crclient.Client, pr *pipeline.PipelineRun, pipelineTaskName string) ([]pipeline.TaskRunResult, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName != pipelineTaskName {
			continue
		}
		taskRun := &pipeline.TaskRun{}
		taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
		if err := c.Get(context.Background(), taskRunKey, taskRun); err != nil {
			return nil, err
		}
		return taskRun.Status.TaskRunStatusFields.Results, nil
	}
	return nil, fmt.Errorf(
		"pipelineTaskName %q not found in PipelineRun %s/%s", pipelineTaskName, pr.GetName(), pr.GetNamespace())
}

func validateTaskRunResult(trResults []pipeline.TaskRunResult, expectedResultNames []string, taskName string) error {
	for _, rn := range expectedResultNames {
		found := false
		for _, r := range trResults {
			if rn == r.Name {
				found = true
				switch r.Name {
				case constants.TektonTaskTestOutputName:
					var testOutput = &TestOutput{}
					err := json.Unmarshal([]byte(r.Value.StringVal), &testOutput)
					if err != nil {
						return fmt.Errorf("cannot parse %q result: %+v", constants.TektonTaskTestOutputName, err)
					}
					// If the test result isn't SUCCESS, the overall outcome is a failure
					if taskName == "sbom-json-check" {
						if testOutput.Result == "FAILURE" {
							return fmt.Errorf("expected Result for Task name %q to be SUCCESS: %+v", taskName, testOutput)
						}
					}
				case "CLAIR_SCAN_RESULT":
					var testOutput = &ClairScanResult{}
					err := json.Unmarshal([]byte(r.Value.StringVal), &testOutput)
					if err != nil {
						return fmt.Errorf("cannot parse CLAIR_SCAN_RESULT result: %+v", err)
					}
				case "PYXIS_HTTP_CODE", "BASE_IMAGE", "BASE_IMAGE_REPOSITORY":
					if len(r.Value.StringVal) < 1 {
						return fmt.Errorf("value of %q result is empty", r.Name)
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("expected result name %q not found in Task %q result", rn, taskName)
		}
	}
	return nil
}
