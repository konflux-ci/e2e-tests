package client

import (
	routev1 "github.com/openshift/api/route/v1"
	applicationservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	tekton "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type K8sClient struct {
	kubeClient *kubernetes.Clientset
	crClient   crclient.Client
}

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(applicationservice.AddToScheme(scheme))
	utilruntime.Must(tekton.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
}

// Kube returns the clientset for Kubernetes upstream.
func (c *K8sClient) KubeInterface() kubernetes.Interface {
	return c.kubeClient
}

// Return a rest client to perform CRUD operations on Kubernetes objects
func (c *K8sClient) KubeRest() crclient.Client {
	return c.crClient
}

// NewHASClient creates kubernetes client wrapper
func NewK8SClient() (*K8sClient, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	crClient, err := crclient.New(cfg, crclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	return &K8sClient{
		kubeClient: client,
		crClient:   crClient,
	}, nil
}
