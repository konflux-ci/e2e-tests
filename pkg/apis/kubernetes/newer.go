package client

import (
	"fmt"
	"os"

	jvmbuildserviceclientset "github.com/redhat-appstudio/jvm-build-service/pkg/client/clientset/versioned"
	pipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	AsAppStudioAdmin *CustomClient
	AsAppStudioUser  *CustomClient
}

func NewKubernetesClient() (*K8SClient, error) {
	adminKubeconfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	adminClusterClient, err := createCustomClient(*adminKubeconfig)
	if err != nil {
		return nil, err
	}

	userKubeconfigPath := os.Getenv("USER_KUBE_CONFIG_PATH")
	if userKubeconfigPath == "" {
		return nil, fmt.Errorf("'USER_KUBE_CONFIG_PATH' env var needs to be exported and has to point to a workload cluster")
	}

	userCfg, err := clientcmd.BuildConfigFromFlags("", userKubeconfigPath)
	if err != nil {
		return nil, err
	}
	userClusterClient, err := createCustomClient(*userCfg)
	if err != nil {
		return nil, err
	}

	return &K8SClient{
		AsAppStudioAdmin: adminClusterClient,
		AsAppStudioUser:  userClusterClient,
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
