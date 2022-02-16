package cmd

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	_ "github.com/redhat-appstudio/e2e-tests/pkg/tests/common"
	_ "github.com/redhat-appstudio/e2e-tests/pkg/tests/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	"k8s.io/klog/v2"
)

const (
	GITHUB_TOKEN_ENV = "GITHUB_TOKEN"
)

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	gomega.Expect(utils.CheckIfEnvironmentExists(GITHUB_TOKEN_ENV)).Should(gomega.BeTrue(), "%s environment variable is not set", GITHUB_TOKEN_ENV)

	return nil
}, func(data []byte) {})

func TestE2E(t *testing.T) {
	klog.Info("Starting Red Hat App Studio e2e tests...")

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Red Hat App Studio E2E tests")
}
