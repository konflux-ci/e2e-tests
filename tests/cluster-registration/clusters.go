package build

import (
	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

var _ = framework.ClusterRegistrationSuiteDescribe("Cluster Registration E2E tests", func() {
	defer g.GinkgoRecover()

	// Initialize the tests controllers
	framework, err := framework.NewFramweork()
	Expect(err).NotTo(HaveOccurred())

	ns := "cluster-reg-config"

	g.Context("infrastructure is running", func() {
		g.It("verify the cluster-registration-installer-controller-manager is running", func() {
			err := framework.CommonController.WaitForPodSelector(framework.CommonController.IsPodRunning, ns, "cluster-registration-antiaffinity-selector", "cluster-registration-installer-controller", 60, 100)
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("verify the correct roles are created", func() {
			_, csaErr := framework.CommonController.GetRole("cluster-registration-installer-leader-election-role", ns)
			Expect(csaErr).NotTo(HaveOccurred())
		})
		g.It("verify the correct rolebindings are created", func() {
			_, csaErr := framework.CommonController.GetRoleBinding("cluster-registration-installer-leader-election-rolebinding", ns)
			Expect(csaErr).NotTo(HaveOccurred())
		})
		g.It("verify the correct service account is created", func() {
			_, err := framework.CommonController.GetServiceAccount("cluster-registration-installer-controller-manager", ns)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
