package framework

import (
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/sandbox"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/gitops"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/integration"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/jvmbuildservice"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/o11y"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/spi"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

type ControllerHub struct {
	HasController             *has.SuiteController
	CommonController          *common.SuiteController
	TektonController          *tekton.SuiteController
	GitOpsController          *gitops.SuiteController
	SPIController             *spi.SuiteController
	ReleaseController         *release.SuiteController
	IntegrationController     *integration.SuiteController
	JvmbuildserviceController *jvmbuildservice.SuiteController
	O11yController            *o11y.SuiteController
}

type Framework struct {
	AsKubeAdmin       *ControllerHub
	AsKubeDeveloper   *ControllerHub
	SandboxController *sandbox.SandboxController
	UserNamespace     string
	UserName          string
}

func NewFramework(userName string) (*Framework, error) {
	return NewFrameworkWithTimeout(userName, time.Second*60)
}

func NewFrameworkWithTimeout(userName string, timeout time.Duration) (*Framework, error) {
	var err error
	var k *kubeCl.K8SClient

	// in some very rare cases fail to get the client for some timeout in member operator.
	// Just try several times to get the user kubeconfig
	err = retry.Do(
		func() error {
			k, err = kubeCl.NewDevSandboxProxyClient(userName)

			return err
		},
		retry.Attempts(20),
	)

	if err != nil {
		return nil, fmt.Errorf("error when initializing kubernetes clients: %v", err)
	}

	asAdmin, err := InitControllerHub(k.AsKubeAdmin)
	if err != nil {
		return nil, fmt.Errorf("error when initializing appstudio hub controllers for admin user: %v", err)
	}

	asUser, err := InitControllerHub(k.AsKubeDeveloper)
	if err != nil {
		return nil, fmt.Errorf("error when initializing appstudio hub controllers for sandbox user: %v", err)
	}

	// "pipeline" service account needs to be present in the namespace before we start with creating tekton resources
	// TODO: STONE-442 - decrease the timeout here back to 30 seconds once this issue is resolved.
	if err = utils.WaitUntil(asAdmin.CommonController.ServiceaccountPresent("pipeline", k.UserNamespace), timeout); err != nil {
		return nil, fmt.Errorf("'pipeline' service account wasn't created in %s namespace: %+v", k.UserNamespace, err)
	}

	return &Framework{
		AsKubeAdmin:       asAdmin,
		AsKubeDeveloper:   asUser,
		SandboxController: k.SandboxController,
		UserNamespace:     k.UserNamespace,
		UserName:          k.UserName,
	}, nil
}

func InitControllerHub(cc *kubeCl.CustomClient) (*ControllerHub, error) {
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
	// Initialize Tekton controller
	tektonController := tekton.NewSuiteController(cc)

	// Initialize GitOps controller
	gitopsController, err := gitops.NewSuiteController(cc)
	if err != nil {
		return nil, err
	}

	// Initialize Release Controller
	releaseController, err := release.NewSuiteController(cc)
	if err != nil {
		return nil, err
	}
	// Initialize Integration Controller
	integrationController, err := integration.NewSuiteController(cc)
	if err != nil {
		return nil, err
	}
	jvmbuildserviceController, err := jvmbuildservice.NewSuiteControler(cc)
	if err != nil {
		return nil, err
	}
	// Initialize o11y controller
	o11yController, err := o11y.NewSuiteController(cc)
	if err != nil {
		return nil, err
	}

	return &ControllerHub{
		HasController:             hasController,
		CommonController:          commonCtrl,
		SPIController:             spiController,
		TektonController:          tektonController,
		GitOpsController:          gitopsController,
		ReleaseController:         releaseController,
		IntegrationController:     integrationController,
		JvmbuildserviceController: jvmbuildserviceController,
		O11yController:            o11yController,
	}, nil

}
