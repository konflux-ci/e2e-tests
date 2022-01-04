package has

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

const (
	ApplicationName      = "test-app"
	ApplicationNamespace = "application-service"
)

var _ = framework.HASSuiteDescribe("Application E2E tests", func() {

	defer g.GinkgoRecover()
	hasController, err := has.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())

	g.Context("Crud operation:", func() {
		g.It("Create application", func() {
			_, err := hasController.CreateHasApplication(ApplicationName, ApplicationNamespace)
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("Get application", func() {
			status, err := hasController.GetHasApplicationStatus(ApplicationName, ApplicationNamespace)
			for _, status := range status.Conditions {
				Expect(string(status.Status)).To(Equal("True"))
			}
			Expect(err).NotTo(HaveOccurred())
		})
		g.It("Delete application", func() {
			err := hasController.DeleteHasApplication(ApplicationName, ApplicationNamespace)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
