package appservice

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.HASSuiteDescribe("Push API e2e tests", func() {
	ginkgo.It("Check if HAS push has created", func() {
		fmt.Println("Test Me")
	})
})
