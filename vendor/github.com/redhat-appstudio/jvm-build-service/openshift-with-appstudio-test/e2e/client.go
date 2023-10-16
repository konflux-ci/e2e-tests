package e2e

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	projectset "github.com/openshift/client-go/project/clientset/versioned"
	quotaset "github.com/openshift/client-go/quota/clientset/versioned"
	jvmclientset "github.com/redhat-appstudio/jvm-build-service/pkg/client/clientset/versioned"
	pipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeConfig         *rest.Config
	kubeClient         *kubeset.Clientset
	projectClient      *projectset.Clientset
	tektonClient       *pipelineclientset.Clientset
	jvmClient          *jvmclientset.Clientset
	apiextensionClient *apiextensionsclient.Clientset
	qutoaClient        *quotaset.Clientset
)

func getConfig() (*rest.Config, error) {
	// If an env variable is specified with the config locaiton, use that
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	}
	// If no explicit location, try the in-cluster config
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags(
			"", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not locate a kubeconfig")
}

func setupClients(t *testing.T) {
	var err error
	if kubeConfig == nil {
		kubeConfig, err = getConfig()
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}

	if kubeClient == nil {
		kubeClient, err = kubeset.NewForConfig(kubeConfig)
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}

	if tektonClient == nil {
		tektonClient, err = pipelineclientset.NewForConfig(kubeConfig)
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}

	if jvmClient == nil {
		jvmClient, err = jvmclientset.NewForConfig(kubeConfig)
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}

	if projectClient == nil {
		projectClient, err = projectset.NewForConfig(kubeConfig)
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}

	if apiextensionClient == nil {
		apiextensionClient, err = apiextensionsclient.NewForConfig(kubeConfig)
		if err != nil {
			t.Fatalf("%#v", err)
		}
	}

	if qutoaClient == nil {
		qutoaClient = quotaset.NewForConfigOrDie(kubeConfig)
	}
}
