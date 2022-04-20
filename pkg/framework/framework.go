package framework

import (
	"fmt"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

// Framework struct to store all controllers
type Framework struct {
	HasController    *has.SuiteController
	CommonController *common.SuiteController
	TektonController *tekton.SuiteController
}

// Initialize all test controllers and return them in a Framework
func NewFramweork() (*Framework, error) {

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

	return &Framework{
		CommonController: commonCtrl,
		HasController:    hasController,
		TektonController: tektonController,
	}, nil
}
