package appservice

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	g "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	hasClient "github.com/redhat-appstudio/e2e-tests/pkg/framework/application-service/client"
)

var _ = HASDescribe("Application e2e tests", func() {
	defer g.GinkgoRecover()
	hasClient, err := hasClient.NewHASClient()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	g.It("Create HAS Application", func() {
		//	_, err := hasClient.CreateHasApplication("application-test", "application-service")
		//	gomega.Expect(err).NotTo(gomega.HaveOccurred())
		argo, err := hasClient.GetArgoApplicationStatus("all-components-staging", "openshift-gitops")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(argo.Health.Status).To(gomega.Equal(health.HealthStatusHealthy))
	})
	g.It("Check if HAS Application has created", func() {
		/*defer g.GinkgoRecover()
		status, err := hasClient.GetHasApplicationStatus("application-sample", "application-service")
		for _, status := range status.Conditions {
			gomega.Expect(string(status.Status)).To(gomega.Equal("True"))
		}

		gomega.Expect(err).NotTo(gomega.HaveOccurred())*/
	})
})
