package vcluster

import (
	"context"
	"fmt"
	"os"
	"time"

	infrastructurev1alpha1 "github.com/loft-sh/cluster-api-provider-vcluster/api/v1alpha1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	flog "github.com/redhat-appstudio/e2e-tests/pkg/apis/vcluster/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterctlclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	clusterctllog "sigs.k8s.io/cluster-api/cmd/clusterctl/log"
)

const (
	KubeconfigSecretKey = "value"
)

var (
	DefaultInfrastructureProviders = []string{"vcluster"}
)

type KubernetesVirtualizationController struct {
	// InfrastructureProvider Refers to the source of computational resources (e.g. machines, networking, etc.). Examples can be vcluster, kind etc
	InfrastructureProvider string

	// VirtualizationLogFolder folder where to write the virtual cluster logs.
	VirtualizationLogFolder string

	// Wrap of kubernetes client to connect to OC/k8s server
	KubernetesClient *kubeCl.CustomClient
}

// Create a new controller to manage VCluster. More information about ClusterCtl https://www.vcluster.com/docs/operator/cluster-api-provider
func NewVclusterController(InfrastructureProvider string, folderLogs string) (*KubernetesVirtualizationController, error) {
	kubeClient, err := kubeCl.NewAdminKubernetesClient()
	if err != nil {
		return nil, err
	}

	return &KubernetesVirtualizationController{
		InfrastructureProvider:  InfrastructureProvider,
		VirtualizationLogFolder: folderLogs,
		KubernetesClient:        kubeClient,
	}, nil
}

// Return a wrapped client to interact with Cluster APIs
func (k *KubernetesVirtualizationController) GetClusterClientWithLogger(logFileName string) (clusterctlclient.Client, *flog.LogFile) {
	log := flog.OpenLogFile(flog.OpenLogFileInput{
		LogFolder: k.VirtualizationLogFolder,
		Name:      fmt.Sprintf("%s.log", logFileName),
	})
	clusterctllog.SetLogger(log.Logger())

	c, _ := clusterctlclient.New("")

	return c, log
}

// Init initializes a management cluster by adding the requested list of providers. Init cluster will install the control planes and cert-manager in order to create our own clusters
func (k *KubernetesVirtualizationController) InitClusterManagement() ([]clusterctlclient.Components, error) {
	clusterClient, logger := k.GetClusterClientWithLogger("init-providers.log")
	defer logger.Close()

	return clusterClient.Init(clusterctlclient.InitOptions{
		InfrastructureProviders: DefaultInfrastructureProviders,
		LogUsageInstructions:    true,
		WaitProviders:           true,
	})
}

// Create Cluster API resources to manage cluster in our workload. https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/scope-and-objectives.md#what-is-cluster-api
// Objects definition can be found at: https://github.com/kubernetes-sigs/cluster-api/blob/main/api/v1alpha4/cluster_types.go
//   - clusterName: The name of our vcluster
//   - targetNamespace: The namespace where we will install the vcluster
func (k *KubernetesVirtualizationController) CreateCluster(clusterName string, targetNamespace string) (*v1alpha4.Cluster, error) {
	cluster := &v1alpha4.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: targetNamespace,
		},
		Spec: v1alpha4.ClusterSpec{
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
				Kind:       "VCluster",
				Name:       clusterName,
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
				Kind:       "VCluster",
				Name:       clusterName,
			},
		},
	}

	if err := k.KubernetesClient.KubeRest().Create(context.TODO(), cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

func (k *KubernetesVirtualizationController) CreateVCluster(clusterName string, targetNamespace string) (*infrastructurev1alpha1.VCluster, error) {
	vclusterObj := &infrastructurev1alpha1.VCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: targetNamespace,
		},
		Spec: infrastructurev1alpha1.VClusterSpec{
			HelmRelease: &infrastructurev1alpha1.VirtualClusterHelmRelease{},
		},
	}
	if err := k.KubernetesClient.KubeRest().Create(context.TODO(), vclusterObj); err != nil {
		return vclusterObj, err
	}

	err := k8swait.Poll(time.Second*20, time.Minute*2, func() (done bool, err error) {
		namespacedName := types.NamespacedName{
			Name:      clusterName,
			Namespace: targetNamespace,
		}
		err = k.KubernetesClient.KubeRest().Get(context.Background(), namespacedName, vclusterObj)

		if vclusterObj.Status.Phase == infrastructurev1alpha1.VirtualClusterDeployed {
			klog.Infof("Cluster '%s' successfully provisioned", vclusterObj.Name)

			return true, nil
		}

		klog.Infof("Waiting for cluster '%s' to be provisioned. Phase: %s", vclusterObj.Name, vclusterObj.Status.Phase)
		return false, err
	})

	if err != nil {
		return nil, err
	}

	kubeconfigBytes, err := k.GetVClusterKubeConfig(context.Background(), vclusterObj)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(fmt.Sprintf("%s/%s-kubeconfig", k.VirtualizationLogFolder, clusterName))
	if err != nil {
		return nil, err
	}

	defer f.Close()

	if _, err := f.WriteString(string(kubeconfigBytes)); err != nil {
		return nil, err
	}

	klog.Infof("Cluster '%s' available in %s", vclusterObj.Name, fmt.Sprintf("%s/%s-kubeconfig", k.VirtualizationLogFolder, clusterName))

	return nil, err
}

func (k *KubernetesVirtualizationController) GetVClusterKubeConfig(ctx context.Context, vCluster *infrastructurev1alpha1.VCluster) ([]byte, error) {
	secretName := vCluster.Name + "-kubeconfig"
	secret := &corev1.Secret{}

	if err := k.KubernetesClient.KubeRest().Get(ctx, types.NamespacedName{Namespace: vCluster.Namespace, Name: secretName}, secret); err != nil {
		return nil, err
	}

	kcBytes, ok := secret.Data[KubeconfigSecretKey]
	if !ok {
		return nil, fmt.Errorf("couldn't find kube config in vcluster secret")
	}

	return kcBytes, nil
}
