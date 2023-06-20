package framework

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/avast/retry-go/v4"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
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
	ProxyUrl          string
	SandboxController *sandbox.SandboxController
	UserNamespace     string
	UserName          string
	UserToken         string
}

func NewFramework(userName string) (*Framework, error) {
	return NewFrameworkWithTimeout(userName, time.Second*60)
}

func NewFrameworkStage(userName string, toolchainApiUrl string, keycloakUrl string, offlineToken string) (*Framework, error) {
	return NewFrameworkStageWithTimeout(userName,toolchainApiUrl, keycloakUrl, offlineToken, time.Second*60)
}

func NewFrameworkWithTimeout(userName string, timeout time.Duration) (*Framework, error) {
	var err error
	var k *kubeCl.K8SClient

	// https://issues.redhat.com/browse/CRT-1670
	if len(userName) > 20 {
		GinkgoWriter.Printf("WARNING: username %q is longer than 20 characters - the tenant namespace prefix will be shortened to %s\n", userName, userName[:20])
	}

	// in some very rare cases fail to get the client for some timeout in member operator.
	// Just try several times to get the user kubeconfig

	err = retry.Do(
		func() error {
			if k, err = kubeCl.NewDevSandboxProxyClient(userName); err != nil {
				GinkgoWriter.Printf("error when creating dev sandbox proxy client: %+v\n", err)
			}
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

	if err = utils.WaitUntil(asAdmin.CommonController.ServiceaccountPresent(constants.DefaultPipelineServiceAccount, k.UserNamespace), timeout); err != nil {
		return nil, fmt.Errorf("'%s' service account wasn't created in %s namespace: %+v", constants.DefaultPipelineServiceAccount, k.UserNamespace, err)
	}

	if err = asAdmin.CommonController.AddRegistryAuthSecretToSA("QUAY_TOKEN", k.UserNamespace); err != nil {
		GinkgoWriter.Println(fmt.Sprintf("Failed to add registry auth secret to service account: %v\n", err))
	}

	return &Framework{
		AsKubeAdmin:       asAdmin,
		AsKubeDeveloper:   asUser,
		ProxyUrl:          k.ProxyUrl,
		SandboxController: k.SandboxController,
		UserNamespace:     k.UserNamespace,
		UserName:          k.UserName,
		UserToken:         k.UserToken,
	}, nil
}

func NewFrameworkStageWithTimeout(userName string, toolchainApiUrl string, keycloakUrl string, offlineToken string, timeout time.Duration) (*Framework, error) {
	var err error
	var k *kubeCl.K8SClient

	if len(userName) > 20 {
		GinkgoWriter.Printf("WARNING: username %q is longer than 20 characters - the tenant namespace prefix will be shortened to %s\n", userName, userName[:20])
	}

	// in some very rare cases fail to get the client for some timeout in member operator.
	// Just try several times to get the user kubeconfig

	err = retry.Do(
		func() error {
			if k, err = kubeCl.NewDevSandboxProxyStageClient(userName, toolchainApiUrl, keycloakUrl, offlineToken); err != nil {
				GinkgoWriter.Printf("error when creating dev sandbox proxy client for stage: %+v\n", err)
			}
			return err
		},
		retry.Attempts(20),
	)

	if err != nil {
		return nil, fmt.Errorf("error when initializing kubernetes clients: %v", err)
	}

	asUser, err := InitControllerHub(k.AsKubeDeveloper)
	if err != nil {
		return nil, fmt.Errorf("error when initializing appstudio hub controllers for sandbox user: %v", err)
	}

	if err = utils.WaitUntil(asUser.CommonController.ServiceaccountPresent(constants.DefaultPipelineServiceAccount, k.UserNamespace), timeout); err != nil {
		return nil, fmt.Errorf("'%s' service account wasn't created in %s namespace: %+v", constants.DefaultPipelineServiceAccount, k.UserNamespace, err)
	}

	return &Framework{
		AsKubeAdmin: nil,
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
