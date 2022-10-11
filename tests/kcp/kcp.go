package kcp

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = Describe("jcp-pene", func() {
	//defer GinkgoRecover()
	// Initialize the tests controllers
	fw, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())
	defer GinkgoRecover()

	It("aaaa", func() {
		_, err = fw.CommonController.CreateTestNamespace("polla-gorda")
		fmt.Println(err)
	})
})
