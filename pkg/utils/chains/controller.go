package chains

import (
	"fmt"

	"github.com/redhat-appstudio/e2e-tests/pkg/client"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// Create the struct for kubernetes clients
type SuiteController struct {
	*client.K8sClient
}

// Create controller for Application/Component crud operations
func NewSuiteController() (*SuiteController, error) {
	client, err := client.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client-go %v", err)
	}
	return &SuiteController{
		client,
	}, nil
}

func VerifyTaskSignature(taskRun *v1beta1.TaskRun) bool {
	return taskRun.Annotations["chains.tekton.dev/signed"] == "true"
}

func (s *SuiteController) VerifyAttestation(taskRun *v1beta1.TaskRun) error {
	return nil

}

func (s *SuiteController) VerifyImageSignature(taskRun *v1beta1.TaskRun) error {
	return nil

}
