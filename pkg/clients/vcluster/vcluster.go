package vcluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	client "github.com/redhat-appstudio/e2e-tests/pkg/clients/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	VCLUSTER_BIN = "vcluster"
)

type vclusterFactory struct {
	TargetDir  string
	KubeClient *client.CustomClient
}

type Vcluster interface {
	InitializeVCluster(clusterName string, targetNamespace string, host string) (kubeconfigPath string, err error)
}

func NewVclusterController(dir string, kube *client.CustomClient) Vcluster {
	return &vclusterFactory{
		TargetDir:  dir,
		KubeClient: kube,
	}
}

func (c *vclusterFactory) InitializeVCluster(clusterName string, targetNamespace string, host string) (kubeconfigPath string, err error) {
	var valuesFilename = fmt.Sprintf("%s/%s-values.yaml", c.TargetDir, clusterName)
	var createVclusterArgs = []string{"create", clusterName, "--namespace", targetNamespace, "--connect=false", "--expose", "-f", valuesFilename}
	kubeconfigPath = fmt.Sprintf("%s/%s-kubeconfig", c.TargetDir, clusterName)

	if err := c.WriteToFile(c.GenerateHelmValues(host), valuesFilename); err != nil {
		return "", err
	}

	if err := utils.ExecuteCommandInASpecificDirectory(VCLUSTER_BIN, createVclusterArgs, ""); err != nil {
		return "", err
	}

	if err := c.CreateKubeconfig(clusterName, targetNamespace, kubeconfigPath); err != nil {
		return "", err
	}

	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %s", err)
	}

	route, err := c.CreateRoute(clusterName, targetNamespace)
	if err != nil {
		return "", fmt.Errorf("failed to create route for vcluster: %s", err)
	}

	config.Clusters[config.CurrentContext].Server = fmt.Sprintf("https://%s", string(route.Spec.Host))

	err = clientcmd.WriteToFile(*config, kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to generate kubeconfig with openshift route: %s", err)
	}

	return kubeconfigPath, nil
}

func (vc *vclusterFactory) CreateRoute(serviceName string, namespace string) (route *routev1.Route, err error) {
	routeSpec := routev1.Route{
		ObjectMeta: v1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https"),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
	if err := vc.KubeClient.KubeRest().Create(context.Background(), &routeSpec); err != nil {
		return nil, err
	}

	return route, wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 5*time.Minute, false, func(ctx context.Context) (done bool, err error) {
		route, err = vc.KubeClient.RouteClient().RouteV1().Routes(namespace).Get(ctx, routeSpec.Name, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, condition := range route.Status.Ingress[0].Conditions {
			if condition.Type == routev1.RouteAdmitted && condition.Status == "True" {
				return true, nil
			}
		}

		return false, nil
	})
}

func (c *vclusterFactory) CreateKubeconfig(clusterName string, targetNamespace string, kubeconfigPath string) error {
	return utils.ExecuteCommandInASpecificDirectory(VCLUSTER_BIN, []string{"connect", clusterName, "--namespace", targetNamespace, "--update-current=false", "--service-account=kube-system/admin",
		"--token-expiration=10800", "--cluster-role=cluster-admin", "--insecure", "--kube-config", kubeconfigPath}, "")
}

func (c *vclusterFactory) GenerateHelmValues(host string) ValuesTemplate {
	return ValuesTemplate{
		Openshift: Openshift{
			Enable: true,
		},
		Sync: Sync{
			NetworkPolicies: NetworkPolicies{
				Enabled: true,
			},
			Services: Services{
				SyncServiceSelector: true,
			},
			Ingresses: Ingresses{
				Enabled:    true,
				PathType:   "Prefix",
				ApiVersion: "networking.k8s.io/v1",
			},
			Secrets: Secrets{
				Enabled: true,
				All:     true,
			},
		},
	}
}

// WriteToFile serializes the config to yaml and writes it out to a file.  If not present, it creates the file with the mode 0600.  If it is present
// it stomps the contents
func (c *vclusterFactory) WriteToFile(config ValuesTemplate, filename string) error {
	content, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	if err := os.WriteFile(filename, content, 0600); err != nil {
		return err
	}
	return nil
}
