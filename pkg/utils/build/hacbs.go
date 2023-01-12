package build

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func FetchTaskRunResult(pr *v1beta1.PipelineRun, pipelineTaskName string, result string) (string, error) {
	for _, tr := range pr.Status.TaskRuns {
		if tr.PipelineTaskName != pipelineTaskName {
			continue
		}
		for _, trResult := range tr.Status.TaskRunResults {
			if trResult.Name == result {
				return strings.TrimSuffix(trResult.Value.StringVal, "\n"), nil
			}
		}
	}
	return "", fmt.Errorf(
		"result %q not found in TaskRuns of PipelineRun %s/%s for pipeline task name %s", result, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name, pipelineTaskName)
}

func FetchImageTaskRunResult(pr *v1beta1.PipelineRun, pipelineTaskName string, result string) (string, error) {
	for _, tr := range pr.Status.TaskRuns {
		if tr.PipelineTaskName != pipelineTaskName {
			continue
		}
		for _, trResult := range tr.Status.TaskRunResults {

			if trResult.Name == "BASE_IMAGE_REPOSITORY" || trResult.Name == result {
				return trResult.Value.StringVal, nil
			}
		}
	}
	return "", fmt.Errorf(
		"result %q not found in TaskRuns of PipelineRun %s/%s", result, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

func ValidateImageTaskRunResults(taskname string, result string) bool {
	var re = regexp.MustCompile(`devfile/python`)
	if taskname == "sanity-inspect-image" {
		if !(re.MatchString(result)) {
			Fail(fmt.Sprintf("Expected Result for Taskrun '%s', failed with '%s'", taskname, result))
		}
	}
	return true
}

func ValidateTaskRunResults(taskname string, result string) bool {
	var testOutput map[string]interface{}
	err := json.Unmarshal([]byte(result), &testOutput)
	if err != nil {
		Fail(fmt.Sprintf("Taskrun '%s' has failed with '%s'", taskname, err))
	}
	// conftest-clair taskruns are expected to FAIL
	if taskname == "conftest-clair" {
		if testOutput["result"] == "FAILURE" {
			// Fail(fmt.Sprintf("Expected Result for Taskrun '%s' is SUCCESS, failed with '%s'", taskname, testOutput["failures"]))
			return true
		}
	}
	// sanity-label-check taskruns are expected to FAIL
	if taskname == "sanity-label-check" {
		if testOutput["result"] == "FAILURE" {
			// Fail(fmt.Sprintf("Expected Result for Taskrun '%s' is SUCCESS, failed with '%s'", taskname, testOutput["failures"]))
			return true
		}
	}
	// If the test result isn't SUCCESS, the overall outcome is a failure
	if taskname == "sbom-json-check" {
		if testOutput["result"] == "FAILURE" {
			Fail(fmt.Sprintf("Expected Result for Taskrun '%s' is SUCCESS, but '%d' test failed", taskname, testOutput["failures"]))
		}
		return true
	}
	return false
}
