package framework

import (
	"fmt"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/gitops"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/singapore"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

// Framework struct to store all controllers
type Framework struct {
	HasController       *has.SuiteController
	CommonController    *common.SuiteController
	TektonController    *tekton.SuiteController
	GitOpsController    *gitops.SuiteController
	SingaporeController *singapore.SuiteController
}

// Initialize all test controllers and return them in a Framework
func NewFramework() (*Framework, error) {

	// Initialize a common kubernetes client to be passed to the test controllers
	kubeClient, err := kubeCl.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client-go %v", err)
	}

	// Initialize Common controller
	commonCtrl, err := common.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	// Initialize Has controller
	hasController, err := has.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	// Initialize Tekton controller
	tektonController, err := tekton.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	// Initialize GitOps controller
	gitopsController, err := gitops.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	singaporeController, err := singapore.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	return &Framework{
		CommonController:    commonCtrl,
		HasController:       hasController,
		TektonController:    tektonController,
		GitOpsController:    gitopsController,
		SingaporeController: singaporeController,
	}, nil
}
