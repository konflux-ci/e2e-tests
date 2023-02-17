package utils

import (
	"fmt"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// GetFailedPipelineRunLogs gets the logs of the pipelinerun failed task
func GetFailedPipelineRunLogs(c *common.SuiteController, pipelineRun *v1beta1.PipelineRun) string {
	failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
	d := utils.GetFailedPipelineRunDetails(pipelineRun)
	if d.FailedContainerName != "" {
		logs, _ := c.GetContainerLogs(d.PodName, d.FailedContainerName, pipelineRun.Namespace)
		failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
	}
	return failMessage
}
