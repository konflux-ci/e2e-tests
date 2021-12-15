package cmd

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	_ "github.com/redhat-appstudio/e2e-tests/pkg/tests/common"
	_ "github.com/redhat-appstudio/e2e-tests/pkg/tests/has"
)

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(data []byte) {})

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Red Hat App Studio E2E tests")
}
