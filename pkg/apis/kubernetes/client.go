package client

import (
	"fmt"
	"io/ioutil"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	ocpOauth "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/sandbox"
	integrationservice "github.com/redhat-appstudio/integration-service/api/v1alpha1"
	jvmbuildservice "github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	jvmbuildserviceclientset "github.com/redhat-appstudio/jvm-build-service/pkg/client/clientset/versioned"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend-shared/apis/managed-gitops/v1alpha1"
	managedgitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend/apis/managed-gitops/v1alpha1"
	release "github.com/redhat-appstudio/release-service/api/v1alpha1"
	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	tekton "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type CustomClient struct {
	kubeClient            *kubernetes.Clientset
	crClient              crclient.Client
	pipelineClient        pipelineclientset.Interface
	dynamicClient         dynamic.Interface
	jvmbuildserviceClient jvmbuildserviceclientset.Interface
}

type K8SClient struct {
	AsKubeAdmin       *CustomClient
	AsKubeDeveloper   *CustomClient
	SandboxController *sandbox.SandboxController
	UserName          string
}

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appstudioApi.AddToScheme(scheme))
	utilruntime.Must(ocpOauth.AddToScheme(scheme))
	utilruntime.Must(tekton.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(managedgitopsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(spi.AddToScheme(scheme))
	utilruntime.Must(toolchainv1alpha1.AddToScheme(scheme))
	utilruntime.Must(release.AddToScheme(scheme))
	utilruntime.Must(gitopsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(integrationservice.AddToScheme(scheme))
	utilruntime.Must(jvmbuildservice.AddToScheme(scheme))
	utilruntime.Must(ecp.AddToScheme(scheme))
	utilruntime.Must(buildservice.AddToScheme(scheme))
	utilruntime.Must(userv1.AddToScheme(scheme))
}

// Kube returns the clientset for Kubernetes upstream.
func (c *CustomClient) KubeInterface() kubernetes.Interface {
	return c.kubeClient
}

// Return a rest client to perform CRUD operations on Kubernetes objects
func (c *CustomClient) KubeRest() crclient.Client {
	return c.crClient
}

func (c *CustomClient) PipelineClient() pipelineclientset.Interface {
	return c.pipelineClient
}

func (c *CustomClient) JvmbuildserviceClient() jvmbuildserviceclientset.Interface {
	return c.jvmbuildserviceClient
}

// Returns a DynamicClient interface.
// Note: other client interfaces are likely preferred, except in rare cases.
func (c *CustomClient) DynamicClient() dynamic.Interface {
	return c.dynamicClient
}

func NewDevSandboxProxyClient(userName string) (*K8SClient, error) {
	asAdminClient, err := NewAdminKubernetesClient()
	if err != nil {
		return nil, err
	}

	sandboxController, err := sandbox.NewDevSandboxController(asAdminClient.KubeInterface(), asAdminClient.KubeRest())
	if err != nil {
		return nil, err
	}

	userAuthInfo, err := sandboxController.ReconcileUserCreation(userName)
	if err != nil {
		return nil, err
	}

	cfgBytes, err := ioutil.ReadFile(userAuthInfo.KubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user kubeconfig %v", err)
	}

	userCfg, err := clientcmd.RESTConfigFromKubeConfig(cfgBytes)
	if err != nil {
		return nil, err
	}

	sandboxProxyClient, err := createCustomClient(*userCfg)
	if err != nil {
		return nil, err
	}

	return &K8SClient{
		AsKubeAdmin:       asAdminClient,
		AsKubeDeveloper:   sandboxProxyClient,
		UserName:          userAuthInfo.UserName,
		SandboxController: sandboxController,
	}, nil
}

func NewAdminKubernetesClient() (*CustomClient, error) {
	adminKubeconfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	return createCustomClient(*adminKubeconfig)
}

func createCustomClient(cfg rest.Config) (*CustomClient, error) {
	client, err := kubernetes.NewForConfig(&cfg)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(&cfg)
	if err != nil {
		return nil, err
	}

	crClient, err := crclient.New(&cfg, crclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	pipelineClient, err := pipelineclientset.NewForConfig(&cfg)
	if err != nil {
		return nil, err
	}

	jvmbuildserviceClient, err := jvmbuildserviceclientset.NewForConfig(&cfg)
	if err != nil {
		return nil, err
	}
	return &CustomClient{
		kubeClient:            client,
		crClient:              crClient,
		pipelineClient:        pipelineClient,
		dynamicClient:         dynamicClient,
		jvmbuildserviceClient: jvmbuildserviceClient,
	}, nil
}
