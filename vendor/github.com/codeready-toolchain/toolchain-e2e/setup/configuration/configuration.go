package configuration

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-e2e/setup/terminal"

	quotav1 "github.com/openshift/api/quota/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OauthNS   = "openshift-authentication"
	OauthName = "oauth-openshift"

	DefaultHostNS        = "toolchain-host-operator"
	HostOperatorWorkload = "host-operator-controller-manager"

	DefaultMemberNS        = "toolchain-member-operator"
	MemberOperatorWorkload = "member-operator-controller-manager"

	CustomTemplateUsersParam  = "custom"
	DefaultTemplateUsersParam = "default"
)

var (
	HostOperatorNamespace   string
	MemberOperatorNamespace string

	DefaultRetryInterval = time.Millisecond * 200
	DefaultTimeout       = time.Minute * 5
)

// NewClient returns a new client to the cluster defined by the current context in
// the KUBECONFIG
func NewClient(term terminal.Terminal, kubeconfigPath string) (client.Client, *rest.Config, *runtime.Scheme, error) {
	// look-up the kubeconfig to use
	kubeconfigFile, err := getKubeconfigFile(kubeconfigPath)
	if err != nil {
		term.Fatalf(err, "error while locating KUBECONFIG")
	}
	term.Debugf("📔 using kubeconfig at %s", kubeconfigFile.Name())
	kubeconfig, err := newKubeConfig(kubeconfigFile)
	if err != nil {
		term.Fatalf(err, "error while loading KUBECONFIG")
	}
	s, err := NewScheme()
	if err != nil {
		term.Fatalf(err, "cannot configure scheme")
	}
	clientConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		term.Fatalf(err, "cannot create client config")
	}

	cl, err := client.New(clientConfig, client.Options{Scheme: s})
	term.Infof("API endpoint: %s", clientConfig.Host)
	return cl, clientConfig, s, err
}

// NewScheme returns the scheme configured with all the needed types
func NewScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()
	builder := append(
		runtime.SchemeBuilder{},
		toolchainv1alpha1.AddToScheme,
		quotav1.Install,
		operatorsv1alpha1.AddToScheme,
		templatev1.Install,
		routev1.Install,
		appsv1.AddToScheme,
	)
	err := builder.AddToScheme(s)
	return s, err
}

// GetKubeconfigFile returns a file reader on (by order of match):
// - the --kubeconfig CLI argument if it was provided
// - the $KUBECONFIG file it the env var was set
// - the <user_home_dir>/.kube/config file
func getKubeconfigFile(kubeconfigPath string) (*os.File, error) {
	var path string
	if kubeconfigPath != "" {
		path = kubeconfigPath
	} else if kubeconfigPath = os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
		path = kubeconfigPath
	} else {
		path = filepath.Join(homeDir(), ".kube", "config")
	}
	return os.Open(path)
}

func newKubeConfig(r io.Reader) (clientcmd.ClientConfig, error) {
	d, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return clientcmd.NewClientConfigFromBytes(d)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
