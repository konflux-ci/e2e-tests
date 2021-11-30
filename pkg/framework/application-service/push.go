package appservice

import (
	"fmt"

	"github.com/onsi/ginkgo"
)

var _ = HASDescribe("Push API e2e tests", func() {
	ginkgo.It("Check if HAS push has created", func() {
		fmt.Println("Test Component")
	})
})
