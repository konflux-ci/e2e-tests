package build

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.ChainsSuiteDescribe("Application E2E tests", func() {
	defer g.GinkgoRecover()
	commonController, err := common.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())

	g.Context("Crud operation:", func() {
		g.It("Verify the chains controller is running", func() {
			err := commonController.WaitForPodToBeReady("app", "tekton-chains-controller", "tekton-chains")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
