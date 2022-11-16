package client

import (
	"fmt"
	"os"

	routev1 "github.com/openshift/api/route/v1"
	applicationservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	managedgitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend/apis/managed-gitops/v1alpha1"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	ws "github.com/kcp-dev/kcp/pkg/apis/tenancy/v1beta1"
	integrationservice "github.com/redhat-appstudio/integration-service/api/v1alpha1"
	jvmbuildservice "github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	jvmbuildserviceclientset "github.com/redhat-appstudio/jvm-build-service/pkg/client/clientset/versioned"
	release "github.com/redhat-appstudio/release-service/api/v1alpha1"
	spi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	tekton "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
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
type K8sClients struct {
	ClusterClient       *CustomClient
	AppstudioClient     *CustomClient
	HacbsClient         *CustomClient
	AppstudioUserClient *CustomClient
	HacbsUserClient     *CustomClient
}

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(applicationservice.AddToScheme(scheme))
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
	utilruntime.Must(ws.AddToScheme(scheme))
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

// NewK8SClients creates kubernetes client wrapper
func NewK8SClients() (*K8sClients, error) {
	// Init (workload) cluster client
	clusterKubeconfigPath := os.Getenv("CLUSTER_KUBECONFIG")
	if clusterKubeconfigPath == "" {
		return nil, fmt.Errorf("'CLUSTER_KUBECONFIG' env var needs to be exported and has to point to a workload cluster")
	}
	cfgCluster, err := clientcmd.BuildConfigFromFlags("", clusterKubeconfigPath)
	if err != nil {
		return nil, err
	}
	clusterClient, err := createCustomClient(*cfgCluster)
	if err != nil {
		return nil, err
	}

	// Init clients for appstudio, redhat-appstudio, hacbs, redhat-hacbs workspaces
	wsID := os.Getenv("WORKSPACE_ID")
	if wsID != "" {
		wsID = "-" + wsID
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	kcpHome := cfg.Host

	// :appstudio ws client
	cfg.Host = kcpHome + ":appstudio" + wsID
	auc, err := createCustomClient(*cfg)
	if err != nil {
		return nil, err
	}

	// :hacbs ws client
	cfg.Host = kcpHome + ":hacbs" + wsID
	huc, err := createCustomClient(*cfg)
	if err != nil {
		return nil, err
	}

	// :redhat-appstudio ws client
	cfg.Host = kcpHome + ":redhat-appstudio" + wsID
	ac, err := createCustomClient(*cfg)
	if err != nil {
		return nil, err
	}

	// :redhat-hacbs ws client
	cfg.Host = kcpHome + ":redhat-hacbs" + wsID
	hc, err := createCustomClient(*cfg)
	if err != nil {
		return nil, err
	}

	return &K8sClients{
		ClusterClient:       clusterClient,
		AppstudioClient:     ac,
		HacbsClient:         hc,
		AppstudioUserClient: auc,
		HacbsUserClient:     huc,
	}, nil
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
