package build

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.ChainsSuiteDescribe("Tekton Chains E2E tests", func() {
	defer g.GinkgoRecover()
	commonController, err := common.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())
	ns := "tekton-chains"

	g.Context("infrastructure is running", func() {
		g.It("verify the chains controller is running", func() {
			err := commonController.WaitForPodSelector(commonController.IsPodRunning, ns, "app", "tekton-chains-controller", 60, 100)
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("verify the correct secrets have been created", func() {
			_, err := commonController.VerifySecretExists(ns, "chains-ca-cert")
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("verify the correct roles are created", func() {
			_, csaErr := commonController.GetRole("chains-secret-admin", ns)
			Expect(csaErr).NotTo(HaveOccurred())
			_, srErr := commonController.GetRole("secret-reader", "openshift-ingress-operator")
			Expect(srErr).NotTo(HaveOccurred())
		})
		g.It("verify the correct rolebindings are created", func() {
			_, csaErr := commonController.GetRoleBinding("chains-secret-admin", ns)
			Expect(csaErr).NotTo(HaveOccurred())
			_, csrErr := commonController.GetRoleBinding("chains-secret-reader", "openshift-ingress-operator")
			Expect(csrErr).NotTo(HaveOccurred())
		})
		g.It("verify the correct service account is created", func() {
			_, err := commonController.GetServiceAccount("chains-secrets-admin", ns)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
