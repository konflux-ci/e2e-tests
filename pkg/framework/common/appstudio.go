package common

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	commonCtrl "github.com/redhat-appstudio/e2e-tests/pkg/framework/common/controller"
)

var _ = framework.CommonSuiteDescribe("Common e2e tests", func() {
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
