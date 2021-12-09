package appservice

import (
	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	hController "github.com/redhat-appstudio/e2e-tests/pkg/framework/application-service/controller"
)

const (
	HASApplicationName      = "test-app"
	HASApplicationNamespace = "application-service"
)

var _ = framework.HASSuiteDescribe("Application E2E tests", func() {
	defer g.GinkgoRecover()
	hasController, err := hController.NewHasSuiteController()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	g.Context("Crud operation:", func() {
		g.It("Create application", func() {
			_, err := hasController.CreateHasApplication(HASApplicationName, HASApplicationNamespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		g.It("Get application", func() {
			status, err := hasController.GetHasApplicationStatus(HASApplicationName, HASApplicationNamespace)
			for _, status := range status.Conditions {
				gomega.Expect(string(status.Status)).To(gomega.Equal("True"))
			}
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		g.It("Delete application", func() {
			err := hasController.DeleteHasApplication(HASApplicationName, HASApplicationNamespace)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
