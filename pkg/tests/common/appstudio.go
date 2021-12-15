package common

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	commonCtrl "github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
)

var (
	// Pipelines names from https://github.com/redhat-appstudio/infra-deployments/tree/main/components/build/build-templates
	AppStudioPipelinesNames      = []string{"analyze-devfile", "appstudio-utils"}
	AppStudioComponents          = []string{"all-components-staging", "authentication", "build", "gitops", "has"}
	AppStudioComponentsNamespace = "openshift-gitops"
	PipelinesNamespace           = "build-templates"
)

var _ = framework.CommonSuiteDescribe("Red Hat App Studio common E2E", func() {
	defer g.GinkgoRecover()
	commonController, err := commonCtrl.NewSuiteController()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	g.Context("Argo CD", func() {
		for _, component := range AppStudioComponents {
			g.It(component+" status", func() {
				componentStatus, err := commonController.GetAppStudioComponentStatus(component, AppStudioComponentsNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(componentStatus.Health.Status).To(gomega.Equal(health.HealthStatusHealthy))

			})
		}
	})

	g.Context("ClusterTasks:", func() {
		for _, pipelineName := range AppStudioPipelinesNames {
			g.It("Check if "+pipelineName+" clustertask is pre-created", func() {
				p, err := commonController.GetClusterTask(pipelineName, PipelinesNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(p.Name).To(gomega.Equal(pipelineName))
			})
		}
	})
})
