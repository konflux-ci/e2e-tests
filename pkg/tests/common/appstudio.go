package common

import (
	"time"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	commonCtrl "github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	// Pipelines names from https://github.com/redhat-appstudio/infra-deployments/tree/main/components/build/build-templates
	AppStudioClusterTaskNames    = []string{"analyze-devfile", "cleanup-build-directories", "image-exists", "appstudio-utils"}
	AppStudioComponents          = []string{"all-components-staging", "authentication", "build", "gitops"}
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
				err := commonController.WaitForArgoCDApplicationToBeReady(component, AppStudioComponentsNamespace)
				Expect(err).NotTo(HaveOccurred(), "AppStudio application "+component+" didn't start in 5 minutes")
			})
		}
	})

	g.Context("ClusterTasks:", func() {
		g.It("Check if AppStudio ClusterTasks are precreated", func() {
			err := wait.PollImmediate(100*time.Millisecond, 3*time.Minute, func() (done bool, err error) {
				for _, clusterTaskName := range AppStudioClusterTaskNames {
					_, err := commonController.GetClusterTask(clusterTaskName, PipelinesNamespace)
					if errors.IsNotFound(err) {
						return false, nil
					} else if err != nil {
						return false, err
					}
				}
				return true, nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
