package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	_ "github.com/redhat-appstudio/e2e-tests/tests/build"
	_ "github.com/redhat-appstudio/e2e-tests/tests/cluster-registration"
	_ "github.com/redhat-appstudio/e2e-tests/tests/e2e-demos"
	_ "github.com/redhat-appstudio/e2e-tests/tests/has"
	_ "github.com/redhat-appstudio/e2e-tests/tests/release"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/log"

	"flag"

	"github.com/spf13/viper"

	"k8s.io/klog/v2"
)

type level int32

var verbosity level

// kind provides a thin abstraction layer for a KIND cluster.
type kind struct {
	Provider     *cluster.Provider
	context      string
	explicitPath string
}

func newKind(kindContext string, explicitPath string) kind {

	provider := cluster.NewProvider(cluster.ProviderWithLogger(log.NoopLogger{}))

	return kind{
		Provider:     provider,
		context:      kindContext,
		explicitPath: explicitPath,
	}
}

const (
	kindTestContext = "test1"
	testImage       = "docker.io/library/busybox:latest"
)

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(data []byte) {})

var webhookConfigPath string
var demoSuitesPath string

func init() {
	rootDir, _ := os.Getwd()

	flag.StringVar(&webhookConfigPath, "webhookConfigPath", "", "path to webhook config file")
	flag.StringVar(&demoSuitesPath, "config-suites", fmt.Sprintf(rootDir+"/tests/e2e-demos/config/default.yaml"), "path to e2e demo suites definition")
}

func TestE2E(t *testing.T) {
	klog.Info("Starting Red Hat App Studio e2e tests...")
	// Setting viper configurations in cache
	viper.Set("config-suites", demoSuitesPath)
	kind := newKind(kindTestContext, "kubeconfig")

	config := v1alpha4.Cluster{}

	if err := kind.Run(&config); err != nil {
		fmt.Println(err)
	}
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Red Hat App Studio E2E tests")
}

var _ = ginkgo.SynchronizedAfterSuite(func() {}, func() {
	//Send webhook only it the parameter configPath is not empty
	if len(webhookConfigPath) > 0 {
		klog.Info("Send webhook")
		framework.SendWebhook(webhookConfigPath)
	}
})

// Run starts a KIND cluster from a given configuration.
func (k *kind) Run(config *v1alpha4.Cluster) error {
	return k.Provider.Create(
		k.context,
		cluster.CreateWithV1Alpha4Config(config),
		cluster.CreateWithKubeconfigPath(k.explicitPath),
		cluster.CreateWithRetain(true),
	)
}
