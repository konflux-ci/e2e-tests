package framework

import (
	"fmt"

	"github.com/avast/retry-go/v4"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/sandbox"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/gitops"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/integration"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/jvmbuildservice"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/spi"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

type UserControllerHub struct {
	HasController             *has.SuiteController
	CommonController          *common.SuiteController
	GitOpsController          *gitops.SuiteController
	SPIController             *spi.SuiteController
	IntegrationController     *integration.SuiteController
	TektonController          *tekton.SuiteController
	JvmbuildserviceController *jvmbuildservice.SuiteController
}

type AdminControllerHub struct {
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
	AsKubeAdmin       *AdminControllerHub
	AsKubeDeveloper   *UserControllerHub
	SandboxController *sandbox.SandboxController
	UserNamespace     string
}

func NewFramework(userName string) (*Framework, error) {
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

	asAdmin, err := initAdminControllerHub(k.AsKubeAdmin)
	if err != nil {
		return nil, fmt.Errorf("error when initializing appstudio hub controllers for admin user: %v", err)
	}

	asUser, err := initUserControllerHub(k.AsKubeDeveloper)
	if err != nil {
		return nil, fmt.Errorf("error when initializing appstudio hub controllers for sandbox user: %v", err)
	}

	return &Framework{
		AsKubeAdmin:       asAdmin,
		AsKubeDeveloper:   asUser,
		SandboxController: k.SandboxController,
		UserNamespace:     fmt.Sprintf("%s-tenant", k.UserName),
	}, nil
}

func initAdminControllerHub(cc *kubeCl.CustomClient) (*AdminControllerHub, error) {
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

	return &AdminControllerHub{
		HasController:             hasController,
		CommonController:          commonCtrl,
		SPIController:             spiController,
		TektonController:          tektonController,
		GitOpsController:          gitopsController,
		ReleaseController:         releaseController,
		IntegrationController:     integrationController,
		JvmbuildserviceController: jvmbuildserviceController,
	}, nil

}

func initUserControllerHub(cc *kubeCl.CustomClient) (*UserControllerHub, error) {
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

	// Initialize GitOps controller
	gitopsController, err := gitops.NewSuiteController(cc)
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

	// Initialize Tekton controller
	tektonController := tekton.NewSuiteController(cc)

	return &UserControllerHub{
		HasController:             hasController,
		CommonController:          commonCtrl,
		SPIController:             spiController,
		GitOpsController:          gitopsController,
		IntegrationController:     integrationController,
		JvmbuildserviceController: jvmbuildserviceController,
		TektonController:          tektonController,
	}, nil
}
