package o11y

import (
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
)

const (
	O11yUserDeployments  = "o11y-e2e-deployments"
	O11yUserPipelineruns = "o11y-e2e-pipelines"
	O11ySA               = constants.DefaultPipelineServiceAccount

	monitoringNamespace   string = "openshift-monitoring"
	userWorkloadNamespace string = "openshift-user-workload-monitoring"
	userWorkloadToken     string = "prometheus-user-workload-token"
	o11yUserSecret        string = "o11y-tests-token"
	vCPUSuccessMessage    string = "vCPU Deployment Completed"
	egressSuccessMessage  string = "Image push completed"

	pipelineRunTimeout = int(time.Duration(5) * time.Minute)
	deploymentTimeout  = (5 * time.Minute)
	logScriptTimeout   = (3 * time.Minute)
)
