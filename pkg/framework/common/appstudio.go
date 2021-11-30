package common

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	g "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	commonCtrl "github.com/redhat-appstudio/e2e-tests/pkg/framework/common/controller"
)

var _ = commonDescribe("Common e2e tests", func() {
	defer g.GinkgoRecover()
	commonController, err := commonCtrl.NewCommonSuiteController()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	g.Context("AppStudio E2E common specs", func() {
		g.It("Check appstudio application health", func() {
			argo, err := commonController.GetAppStudioStatus("all-components-staging", "openshift-gitops")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(argo.Health.Status).To(gomega.Equal(health.HealthStatusHealthy))
		})
	})
})
