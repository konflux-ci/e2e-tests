package framework

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	coreV1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/avast/retry-go/v4"
	"github.com/konflux-ci/e2e-tests/pkg/clients/common"
	"github.com/konflux-ci/e2e-tests/pkg/clients/has"
	"github.com/konflux-ci/e2e-tests/pkg/clients/imagecontroller"
	"github.com/konflux-ci/e2e-tests/pkg/clients/integration"
	kubeCl "github.com/konflux-ci/e2e-tests/pkg/clients/kubernetes"
	"github.com/konflux-ci/e2e-tests/pkg/clients/release"
	"github.com/konflux-ci/e2e-tests/pkg/clients/tekton"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/sandbox"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

type ControllerHub struct {
	HasController         *has.HasController
	CommonController      *common.SuiteController
	TektonController      *tekton.TektonController
	ReleaseController     *release.ReleaseController
	IntegrationController *integration.IntegrationController
	ImageController       *imagecontroller.ImageController
}

type Framework struct {
	AsKubeAdmin          *ControllerHub
	AsKubeDeveloper      *ControllerHub
	ClusterAppDomain     string
	OpenshiftConsoleHost string
	ProxyUrl             string
	SandboxController    *sandbox.SandboxController
	UserNamespace        string
	UserName             string
	UserToken            string
}

func NewFramework(userName string, stageConfig ...utils.Options) (*Framework, error) {
	return NewFrameworkWithTimeout(userName, time.Second*60, stageConfig...)
}

// This periodically refreshes framework for Stage user because of Keycloak access token expires in 15 minutes
func refreshFrameworkStage(currentFramework *Framework, userName string, timeout time.Duration, options ...utils.Options) {
	for {
		time.Sleep(time.Minute * 10)
		fw, err := newFrameworkWithTimeout(userName, timeout, options...)
		if err != nil {
			fmt.Printf("ERROR: Failed refreshing framework for user %s: %+v\n", userName, err)
			return
		}
		*currentFramework = *fw
	}
}

func newFrameworkWithTimeout(userName string, timeout time.Duration, options ...utils.Options) (*Framework, error) {
	var err error
	var k *kubeCl.K8SClient
	var clusterAppDomain, openshiftConsoleHost string
	var option utils.Options
	var asUser *ControllerHub

	if userName == "" {
		return nil, fmt.Errorf("userName cannot be empty when initializing a new framework instance")
	}
	isStage, isSA, err := utils.CheckOptions(options)
	if err != nil {
		return nil, err
	}
	if len(options) == 1 {
		option = options[0]
	} else {
		option = utils.Options{}
	}

	var asAdmin *ControllerHub
	if isStage {
		// in some very rare cases fail to get the client for some timeout in member operator.
		// Just try several times to get the user kubeconfig
		err = retry.Do(
			func() error {
				if k, err = kubeCl.NewDevSandboxProxyClient(userName, isSA, option); err != nil {
					GinkgoWriter.Printf("error when creating dev sandbox proxy client: %+v\n", err)
				}
				return err
			},
			retry.Attempts(20),
		)

		if err != nil {
			return nil, fmt.Errorf("error when initializing kubernetes clients: %v", err)
		}
		asUser, err = InitControllerHub(k.AsKubeDeveloper)
		if err != nil {
			return nil, fmt.Errorf("error when initializing appstudio hub controllers for sandbox user: %v", err)
		}
		asAdmin = asUser

	} else {
		client, err := kubeCl.NewAdminKubernetesClient()
		if err != nil {
			return nil, err
		}

		asAdmin, err = InitControllerHub(client)
		if err != nil {
			return nil, fmt.Errorf("error when initializing appstudio hub controllers for admin user: %v", err)
		}
		asUser = asAdmin

		nsName := os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV)
		if nsName == "" {
			nsName = userName

			_, err := asAdmin.CommonController.CreateTestNamespace(userName)
			if err != nil {
				return nil, fmt.Errorf("failed to create test namespace %s: %+v", nsName, err)
			}

			// creating this empty configMap change is temporary, when we move to SA per component fully, it will be removed
			cmName := "use-new-sa"
			cmNamespace := "build-service"
			_, err = asAdmin.CommonController.GetConfigMap(cmName, cmNamespace)
			if err != nil {
				// if not found, create new one
				if k8sErrors.IsNotFound(err) {
					newConfigMap := &coreV1.ConfigMap{
						ObjectMeta: v1.ObjectMeta{
							Name: cmName,
						},
					}
					_, err := asAdmin.CommonController.CreateConfigMap(newConfigMap, cmNamespace)
					if err != nil && !k8sErrors.IsAlreadyExists(err) {
						return nil, fmt.Errorf("failed to create %s configMap with error: %v", cmName, err)
					}
				} else {
					return nil, fmt.Errorf("failed to get config map with error: %v", err)
				}
			}

			if err = utils.WaitUntil(asAdmin.CommonController.ServiceAccountPresent(constants.DefaultPipelineServiceAccount, nsName), timeout); err != nil {
				return nil, fmt.Errorf("'%s' service account wasn't created in %s namespace: %+v", constants.DefaultPipelineServiceAccount, nsName, err)
			}
		}

		if os.Getenv("TEST_ENVIRONMENT") == "upstream" {
			// Get cluster domain (IP address) from kubeconfig
			kubeconfig, err := config.GetConfig()
			if err != nil {
				return nil, fmt.Errorf("error when getting kubeconfig: %+v", err)
			}

			parsedURL, err := url.Parse(kubeconfig.Host)
			if err != nil {
				return nil, fmt.Errorf("failed to parse kubeconfig host URL: %+v", err)
			}
			clusterAppDomain = parsedURL.Hostname()
			openshiftConsoleHost = clusterAppDomain

		} else {
			r, err := asAdmin.CommonController.CustomClient.RouteClient().RouteV1().Routes("openshift-console").Get(context.Background(), "console", v1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("cannot get openshift console route in order to determine cluster app domain: %+v", err)
			}
			openshiftConsoleHost = r.Spec.Host
			clusterAppDomain = strings.Join(strings.Split(openshiftConsoleHost, ".")[1:], ".")
		}

		proxyAuthInfo := &sandbox.SandboxUserAuthInfo{}

		k = &kubeCl.K8SClient{
			AsKubeAdmin:     client,
			AsKubeDeveloper: client,
			ProxyUrl:        proxyAuthInfo.ProxyUrl,
			UserName:        userName,
			UserNamespace:   nsName,
			UserToken:       proxyAuthInfo.UserToken,
		}

	}

	return &Framework{
		AsKubeAdmin:          asAdmin,
		AsKubeDeveloper:      asUser,
		ClusterAppDomain:     clusterAppDomain,
		OpenshiftConsoleHost: openshiftConsoleHost,
		ProxyUrl:             k.ProxyUrl,
		SandboxController:    k.SandboxController,
		UserNamespace:        k.UserNamespace,
		UserName:             k.UserName,
		UserToken:            k.UserToken,
	}, nil
}

func NewFrameworkWithTimeout(userName string, timeout time.Duration, options ...utils.Options) (*Framework, error) {
	isStage, isSA, err := utils.CheckOptions(options)
	if err != nil {
		return nil, err
	}

	if isStage && !isSA {
		options[0].ToolchainApiUrl = fmt.Sprintf("%s/workspaces/%s", options[0].ToolchainApiUrl, userName)
	}

	fw, err := newFrameworkWithTimeout(userName, timeout, options...)

	if isStage && !isSA {
		go refreshFrameworkStage(fw, userName, timeout, options...)
	}

	return fw, err
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

	// Initialize Tekton controller
	tektonController := tekton.NewSuiteController(cc)

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

	// Initialize Image Controller
	imageController, err := imagecontroller.NewSuiteController(cc)
	if err != nil {
		return nil, err
	}

	return &ControllerHub{
		HasController:         hasController,
		CommonController:      commonCtrl,
		TektonController:      tektonController,
		ReleaseController:     releaseController,
		IntegrationController: integrationController,
		ImageController:       imageController,
	}, nil
}
