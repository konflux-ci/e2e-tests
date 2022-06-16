package cmd

import (
	"testing"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	_ "github.com/redhat-appstudio/e2e-tests/tests/build"
	_ "github.com/redhat-appstudio/e2e-tests/tests/cluster-registration"
	_ "github.com/redhat-appstudio/e2e-tests/tests/e2e-demos"
	_ "github.com/redhat-appstudio/e2e-tests/tests/has"
	_ "github.com/redhat-appstudio/e2e-tests/tests/release"

	"flag"

	"k8s.io/klog/v2"
)

const ()

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(data []byte) {})

var webhookConfigPath string

func init() {
	flag.StringVar(&webhookConfigPath, "webhookConfigPath", "", "path to webhook config file")
}

func TestE2E(t *testing.T) {
	klog.Info("Starting Red Hat App Studio e2e tests...")

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
