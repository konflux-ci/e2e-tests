package framework

import (
	"fmt"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/gitops"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/integration"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/jvmbuildservice"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/spi"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

// Framework struct to store all controllers
type ControllerHub struct {
	HasController             *has.SuiteController
	CommonController          *common.SuiteController
	TektonController          *tekton.SuiteController
	GitOpsController          *gitops.SuiteController
	SPIController             *spi.SuiteController
	ReleaseController         *release.SuiteController
	IntegrationController     *integration.SuiteController
	JvmbuildserviceController *jvmbuildservice.SuiteController
}

type Framework struct {
	Appstudio     *ControllerHub
	Hacbs         *ControllerHub
	AppstudioUser *ControllerHub
	HacbsUser     *ControllerHub
	Cluster       *ControllerHub
}

// Initialize all test controllers and return them in a Framework
func NewFramework() (*Framework, error) {

	// Initialize a common kubernetes client to be passed to the test controllers
	k, err := kubeCl.NewK8SClients()
	if err != nil {
		return nil, fmt.Errorf("error when initializing kubernetes clients: %+v", err)
	}

	a, err := initControllerHub(k.AppstudioClient)
	if err != nil {
		return nil, err
	}

	h, err := initControllerHub(k.HacbsClient)
	if err != nil {
		return nil, err
	}

	au, err := initControllerHub(k.AppstudioUserClient)
	if err != nil {
		return nil, err
	}

	hu, err := initControllerHub(k.HacbsUserClient)
	if err != nil {
		return nil, err
	}

	cl, err := initControllerHub(k.ClusterClient)
	if err != nil {
		return nil, err
	}

	return &Framework{
		Appstudio:     a,
		Hacbs:         h,
		AppstudioUser: au,
		HacbsUser:     hu,
		Cluster:       cl,
	}, nil
}

func initControllerHub(cc *kubeCl.CustomClient) (*ControllerHub, error) {
	// Initialize Common controller
	commonCtrl, err := common.NewSuiteController(cc)
	if err != nil {
		return nil, err
	}

	// Initialize Has controller
	hasController, err := has.NewSuiteController(cc)
	if err != nil {
		return nil, err
	}

	spiController, err := spi.NewSuiteController(cc)
	if err != nil {
		return nil, err
	}

	// TODO: Once all controllers are working on KCP activate all the clients.
	// Initialize Tekton controller
	/*tektonController, err := tekton.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}*/

	/*// Initialize GitOps controller
	gitopsController, err := gitops.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	// Initialize SPI controller
	spiController, err := spi.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	// Initialize Release Controller
	releaseController, err := release.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	// Initialize Integration Controller
	integrationController, err := integration.NewSuiteController(kubeClient)
	if err != nil {
		return nil, err
	}

	jvmbuildserviceController, err := jvmbuildservice.NewSuiteControler(kubeClient)
	if err != nil {
		return nil, err
	}*/

	return &ControllerHub{
		HasController:    hasController,
		CommonController: commonCtrl,
		SPIController:    spiController,
		// TODO: Once all controllers are working on KCP activate all the clients.
		//TektonController: tektonController,
		//GitOpsController:  gitopsController,
		//ReleaseController: releaseController,
		//IntegrationController: integrationController,
		//JvmbuildserviceController: jvmbuildserviceController,
	}, nil
}
