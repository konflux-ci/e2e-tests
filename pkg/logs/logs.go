package logs

import (
	"errors"
	"fmt"
	"os"

	. "github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	. "github.com/onsi/ginkgo/v2"
)

func StoreTestLogs(testNamespace, jobName string, componentPipelineRun *v1beta1.PipelineRun, cs *common.SuiteController, t *tekton.SuiteController) error {
	wd, _ := os.Getwd()
	artifactDir := GetEnv("ARTIFACT_DIR", fmt.Sprintf("%s/tmp", wd))
	testLogsDir := fmt.Sprintf("%s/%s", artifactDir, testNamespace)

	if err := os.MkdirAll(testLogsDir, os.ModePerm); err != nil {
		return err
	}

	var errPods, errPipelineRuns error

	if errPods := cs.StorePodLogs(testNamespace, jobName, testLogsDir); errPods != nil {
		GinkgoWriter.Printf("Failed to store pod logs: %v", errPods)
	}

	if componentPipelineRun != nil {
		if errPipelineRuns := t.StorePipelineRuns(componentPipelineRun, testLogsDir, testNamespace); errPipelineRuns != nil {
			GinkgoWriter.Printf("Failed to store pipelineRun logs: %v", errPipelineRuns)
		}
	}

	return errors.Join(errPods, errPipelineRuns)
}
