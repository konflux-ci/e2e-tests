package framework

import (
	"fmt"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"
)

//struct holding all controllers
type Framework struct {
	HasController    *has.SuiteController
	CommonController *common.SuiteController
}

//initiate all controllers
func NewControllersInterface() (*Framework, error) {
	kubeClient, err := kubeCl.NewK8SClient()
	if err != nil {
		return nil, fmt.Errorf("error creating client-go %v", err)
	}

	commonCtrl, err := common.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	hasController, err := has.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	return &Framework{
		CommonController: commonCtrl,
		HasController:    hasController,
	}, nil
}
