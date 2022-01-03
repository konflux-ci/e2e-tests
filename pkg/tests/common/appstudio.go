package common

import (
	"fmt"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	commonCtrl "github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	// Pipelines names from https://github.com/redhat-appstudio/infra-deployments/tree/main/components/build/build-templates
	AppStudioClusterTaskNames    = []string{"analyze-devfile", "appstudio-utils"}
	AppStudioComponents          = []string{"all-components-staging", "authentication", "build", "gitops", "has"}
	AppStudioComponentsNamespace = "openshift-gitops"
	PipelinesNamespace           = "build-templates"
)

var _ = framework.CommonSuiteDescribe("Red Hat App Studio common E2E", func() {
	defer g.GinkgoRecover()
	commonController, err := commonCtrl.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())

	g.Context("Argo CD", func() {
		for _, component := range AppStudioComponents {
			g.It(component+" status", func() {
				componentStatus, err := commonController.GetAppStudioComponentStatus(component, AppStudioComponentsNamespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(componentStatus.Health.Status).To(Equal(health.HealthStatusHealthy))

			})
		}
	})
	defer g.GinkgoRecover()

	g.Context("ClusterTasks:", func() {
		g.It("Check if AppStudio ClusterTasks are precreated", func() {
			err := wait.PollImmediate(30*time.Second, 10*time.Minute, func() (done bool, err error) {
				for _, clusterTaskName := range AppStudioClusterTaskNames {
					clusterTask, err := commonController.GetClusterTask(clusterTaskName, PipelinesNamespace)
					if err != nil {
						return false, nil
					}
					if clusterTaskName != clusterTask.Name {
						return false, fmt.Errorf("Cluster task have an unexpected name")
					}
				}
				return true, nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
