package o11y

import "time"

const (
	O11yUserDeployments  = "o11y-e2e-deployments"
	O11yUserPipelineruns = "o11y-e2e-pipelineruns"
	O11ySA               = "pipeline"

	o11yUserSecret       string = "o11y-tests-token"
	vCPUSuccessMessage   string = "vCPU Deployment Completed"
	egressSuccessMessage string = "Image push completed"

	pipelineRunTimeout = int(time.Duration(5) * time.Minute)
	deploymentTimeout  = (1 * time.Minute)
	logScriptTimeout   = (5 * time.Minute)
)
