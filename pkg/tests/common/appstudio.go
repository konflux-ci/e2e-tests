package common

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	commonCtrl "github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
)

var (
	// Pipelines names from https://github.com/redhat-appstudio/infra-deployments/tree/main/components/build/build-templates
	AppStudioComponents          = []string{"all-components-staging", "authentication", "build", "gitops"}
	AppStudioComponentsNamespace = "openshift-gitops"
	PipelinesNamespace           = "build-templates"
)

var _ = framework.CommonSuiteDescribe("Red Hat App Studio common E2E", func() {
	defer GinkgoRecover()
	commonController, err := commonCtrl.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())

	Context("Argo CD", func() {
		for _, component := range AppStudioComponents {
			It(component+" status", func() {
				err := commonController.WaitForArgoCDApplicationToBeReady(component, AppStudioComponentsNamespace)
				Expect(err).NotTo(HaveOccurred(), "AppStudio application "+component+" didn't start in 5 minutes")
			})
		}
	})
})
