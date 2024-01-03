package build

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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

func ValidateBuildPipelineTestResults(pipelineRun *tektonv1.PipelineRun, c crclient.Client) error {
	for _, taskName := range taskNames {
		// The inspect-image task is only required for FBC pipelines which we can infer by the component name
		prLabels := pipelineRun.GetLabels()
		componentName := prLabels["appstudio.openshift.io/component"]
		if taskName == "inspect-image" && !strings.HasPrefix(strings.ToLower(componentName), "fbc-") {
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

func fetchTaskRunResults(c crclient.Client, pr *tektonv1.PipelineRun, pipelineTaskName string) ([]tektonv1.TaskRunResult, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName != pipelineTaskName {
			continue
		}
		taskRun := &tektonv1.TaskRun{}
		taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
		if err := c.Get(context.Background(), taskRunKey, taskRun); err != nil {
			return nil, err
		}
		return taskRun.Status.Results, nil
	}
	return nil, fmt.Errorf(
		"pipelineTaskName %q not found in PipelineRun %s/%s", pipelineTaskName, pr.GetName(), pr.GetNamespace())
}

func validateTaskRunResult(trResults []tektonv1.TaskRunResult, expectedResultNames []string, taskName string) error {
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
