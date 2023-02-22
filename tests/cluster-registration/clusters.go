package build

import (
	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

const (
	DEFAULT_USER_CLUSTER_REG = "cluster-reg-ns"
)

var _ = framework.ClusterRegistrationSuiteDescribe("Cluster Registration E2E tests", g.Label("cluster-registration"), func() {
	defer g.GinkgoRecover()
	var fwk *framework.Framework
	var err error

	g.BeforeAll(func() {
		// Initialize the tests controllers
		fwk, err = framework.NewFramework(DEFAULT_USER_CLUSTER_REG)
		Expect(err).NotTo(HaveOccurred())
	})

	g.AfterAll(func() {
		if !g.CurrentSpecReport().Failed() {
			Expect(fwk.SandboxController.DeleteUserSignup(fwk.UserName)).NotTo(BeFalse())
		}
	})

	g.Context("infrastructure is running", func() {
		g.It("verifies if the cluster-registration-installer-controller-manager is running", func() {
			err := fwk.AsKubeAdmin.CommonController.WaitForPodSelector(fwk.AsKubeAdmin.CommonController.IsPodRunning, constants.CLUSTER_REG_NS, "cluster-registration-antiaffinity-selector", "cluster-registration-installer-controller", 60, 100)
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("verifies if the correct roles are created", func() {
			_, csaErr := fwk.AsKubeAdmin.CommonController.GetRole("cluster-registration-installer-leader-election-role", constants.CLUSTER_REG_NS)
			Expect(csaErr).NotTo(HaveOccurred())
		})
		g.It("verifies if the correct rolebindings are created", func() {
			_, csaErr := fwk.AsKubeAdmin.CommonController.GetRoleBinding("cluster-registration-installer-leader-election-rolebinding", constants.CLUSTER_REG_NS)
			Expect(csaErr).NotTo(HaveOccurred())
		})
		g.It("verifies if the correct service account is created", func() {
			_, err := fwk.AsKubeAdmin.CommonController.GetServiceAccount("cluster-registration-installer-controller-manager", constants.CLUSTER_REG_NS)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
