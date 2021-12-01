package appservice

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	hController "github.com/redhat-appstudio/e2e-tests/pkg/framework/application-service/controller"
)

var _ = framework.HASSuiteDescribe("Application e2e tests", func() {
	defer g.GinkgoRecover()
	hasController, err := hController.NewHasSuiteController()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	g.Context("HAS Application tests", func() {
		g.It("Create HAS Application", func() {
			_, err := hasController.CreateHasApplication("test-app", "application-service")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			argo, err := hasController.GetArgoApplicationStatus("all-components-staging", "openshift-gitops")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(argo.Health.Status).To(gomega.Equal(health.HealthStatusHealthy))
		})
		g.It("Check if HAS Application has created", func() {
			status, err := hasController.GetHasApplicationStatus("test-app", "application-service")
			for _, status := range status.Conditions {
				gomega.Expect(string(status.Status)).To(gomega.Equal("True"))
			}

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
