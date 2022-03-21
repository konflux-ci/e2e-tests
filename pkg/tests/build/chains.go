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
	ns := "tekton-chains"

	g.Context("infrastructure is running", func() {
		g.It("verify the chains controller is running", func() {
			err := commonController.WaitForPodSelector(common.IsPodRunning, ns, "app", "tekton-chains-controller", 60)
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("verify the chains certs are installed", func() {
			err := commonController.WaitForPodSelector(common.IsPodSuccessful, ns, "job-name", "chains-certs-configuration", 60)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	g.It("verify the correct secrets have been created", func() {
		_, caErr := commonController.VerifySecretExists(ns, "chains-ca-cert")
		Expect(caErr).NotTo(HaveOccurred())
	})

})
