package tekton

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
)

const quayBaseUrl = "https://quay.io/api/v1"

type KubeController struct {
	Commonctrl common.SuiteController
	Tektonctrl SuiteController
	Namespace  string
}

// Create the struct for kubernetes clients
type SuiteController struct {
	*kubeCl.CustomClient
}

// Create controller for Tekton Task/Pipeline CRUD operations
func NewSuiteController(kube *kubeCl.CustomClient) *SuiteController {
	return &SuiteController{kube}
}
