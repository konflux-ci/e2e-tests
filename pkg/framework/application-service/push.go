package appservice

import (
	g "github.com/onsi/ginkgo/v2"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.HASSuiteDescribe("Push E2E tests", func() {
	defer g.GinkgoRecover()

	g.Context("Crud operation:", func() {
		g.It("Create Push", func() {
			g.Skip("Skipped. No tests detected")
		})
	})
})
