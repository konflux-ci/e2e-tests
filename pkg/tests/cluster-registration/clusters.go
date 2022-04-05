package build

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.ClusterRegistrationSuiteDescribe("Cluster Registration E2E tests", func() {
	defer g.GinkgoRecover()
	commonController, err := common.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())
	ns := "cluster-reg-config"

	g.Context("infrastructure is running", func() {
		g.It("verify the cluster-registration-installer-controller-manager is running", func() {
			err := commonController.WaitForPodSelector(commonController.IsPodRunning, ns, "cluster-registration-antiaffinity-selector", "cluster-registration-installer-controller", 60, 100)
			Expect(err).NotTo(HaveOccurred())
		})
		// g.It("verify the correct secrets have been created", func() {
		// 	_, err := commonController.GetSecret(ns, "")
		// 	Expect(err).NotTo(HaveOccurred())
		// })
		g.It("verify the correct roles are created", func() {
			_, csaErr := commonController.GetRole("cluster-registration-installer-leader-election-role", ns)
			Expect(csaErr).NotTo(HaveOccurred())
		})
		g.It("verify the correct rolebindings are created", func() {
			_, csaErr := commonController.GetRoleBinding("cluster-registration-installer-leader-election-rolebinding", ns)
			Expect(csaErr).NotTo(HaveOccurred())
		})
		g.It("verify the correct service account is created", func() {
			_, err := commonController.GetServiceAccount("cluster-registration-installer-controller-manager", ns)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
