package logs

import (
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

	if err := cs.StorePodLogs(testNamespace, jobName, testLogsDir); err != nil {
		GinkgoWriter.Printf("Failed to store pod logs: %v", err)
	}

	if componentPipelineRun != nil {
		if err := t.StorePipelineRuns(componentPipelineRun, testLogsDir, testNamespace); err != nil {
			GinkgoWriter.Printf("Failed to store pipelineRun logs: %v", err)
		}
	}

	return nil
}
