package cmd

import (
	"fmt"
	"os"
	"testing"

	testutils "github.com/kudobuilder/kuttl/pkg/test/utils"
	"github.com/onsi/ginkgo/v2"
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

// kindLogger lets KIND log to the kuttl logger.
// KIND log level N corresponds to kuttl log level N+1, such that
// using the default 0 kuttl log level produces no KIND output.
type kindLogger struct {
	l testutils.Logger
}

func (k kindLogger) V(level log.Level) log.InfoLogger {
	if int(level) >= int(verbosity) {
		return &nopLogger{}
	}
	return k
}

func (k kindLogger) Warn(message string) {
	k.l.Log(message)
}

func (k kindLogger) Warnf(format string, args ...interface{}) {
	k.l.Logf(format, args...)
}

func (k kindLogger) Error(message string) {
	k.l.Log(message)
}

func (k kindLogger) Errorf(format string, args ...interface{}) {
	k.l.Logf(format, args...)
}

func (k kindLogger) Info(message string) {
	k.l.Log(message)
}

func (k kindLogger) Infof(format string, args ...interface{}) {
	k.l.Logf(format, args...)
}

func (k kindLogger) Enabled() bool {
	return true
}

type nopLogger struct{}

func (n *nopLogger) Enabled() bool {
	return false
}

func (n *nopLogger) Info(message string) {}

func (n *nopLogger) Infof(format string, args ...interface{}) {}

func newKind(kindContext string, explicitPath string, logger testutils.Logger) kind {

	provider := cluster.NewProvider(cluster.ProviderWithLogger(&kindLogger{logger}))

	return kind{
		Provider:     provider,
		context:      kindContext,
		explicitPath: explicitPath,
	}
}

const (
	kindTestContext = "test"
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
	kind := newKind(kindTestContext, "kubeconfig", testutils.NewTestLogger(t, ""))

	config := v1alpha4.Cluster{}

	if err := kind.Run(&config); err != nil {
		fmt.Println(err)
	}

	//gomega.RegisterFailHandler(ginkgo.Fail)
	//ginkgo.RunSpecs(t, "Red Hat App Studio E2E tests")
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

// IsRunning checks if a KIND cluster is already running for the current context.
func (k *kind) IsRunning() bool {
	contexts, err := k.Provider.List()
	if err != nil {
		panic(err)
	}

	for _, context := range contexts {
		if context == k.context {
			return true
		}
	}

	return false
}
