package has

import (
	"strings"
	"time"

	"github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"
	"k8s.io/klog/v2"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	corev1 "k8s.io/api/core/v1"
)

const (
	ApplicationName        string = "pet-clinic"
	HASArgoApplicationName string = "has"
	ApplicationNamespace   string = "application-service"
	GitOpsNamespace        string = "openshift-gitops"
	QuarkusComponentName   string = "quarkus-component-e2e"
	QuarkusDevfileSource   string = "https://github.com/redhat-appstudio-qe/devfile-sample-code-with-quarkus"
)

var _ = framework.HASSuiteDescribe("Application E2E tests", func() {
	defer g.GinkgoRecover()
	hasController, err := has.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())
	commonController, err := common.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())
	application := &v1alpha1.Application{}

	g.BeforeAll(func() {
		klog.Info("Checking HAS Argo application health before starting the tests")
		Expect(commonController.WaitForArgoCDApplicationToBeReady(HASArgoApplicationName, GitOpsNamespace)).NotTo(HaveOccurred(), "HAS Argo application didn't start in 5 minutes")
	})

	g.Context("Create Application from a given devfile", func() {
		g.It("Create Red Hat AppStudio Application CR", func() {
			createdApplication, err := hasController.CreateHasApplication(ApplicationName, ApplicationNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdApplication.Spec.DisplayName).To(Equal(ApplicationName))
			Expect(createdApplication.Namespace).To(Equal(ApplicationNamespace))
		})

		g.It("Red Hat AppStudio Application lifecycle tests", func() {
			Eventually(func() string {
				application, err = hasController.GetHasApplication(ApplicationName, ApplicationNamespace)
				Expect(err).NotTo(HaveOccurred())

				return application.Status.Devfile
			}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()))
		})

		g.It("Create Quarkus component", func() {
			component, err := hasController.CreateComponent(application.Name, QuarkusComponentName, ApplicationNamespace, QuarkusDevfileSource)
			Expect(err).NotTo(HaveOccurred())
			Expect(component.Name).To(Equal(QuarkusComponentName))
		})

		g.It("Wait for component pipeline to be Succeded", func() {
			Eventually(func() corev1.ConditionStatus {
				pipelineRun, _ := hasController.GetComponentPipeline(QuarkusComponentName, ApplicationName)

				for _, condition := range pipelineRun.Status.Conditions {
					klog.Infof("PipelineRun %s reason: %s", pipelineRun.Name, condition.Reason)

					if condition.Status == corev1.ConditionTrue {
						return corev1.ConditionTrue
					}
				}
				return corev1.ConditionFalse
			}, 10*time.Minute, 10*time.Second).Should(Equal(corev1.ConditionTrue))
		})
	})
})

/*
	Right now DevFile status in HAS is a string:
	metadata:
		attributes:
			appModelRepository.url: https://github.com/redhat-appstudio-qe/pet-clinic-application-service-establish-danger
			gitOpsRepository.url: https://github.com/redhat-appstudio-qe/pet-clinic-application-service-establish-danger
		name: pet-clinic
		schemaVersion: 2.1.0
	The ObtainGitUrlFromDevfile extract from the string the git url associated with a application
*/
func ObtainGitUrlFromDevfile(devfile string) string {
	devfileLines := strings.Split(devfile, "\n")
	for _, line := range devfileLines {
		if strings.Contains(line, "appModelRepository.url") {
			gitUrl := strings.Split(line, "url: ")
			if len(gitUrl) == 2 {
				return strings.TrimSpace(gitUrl[1])
			}
		}
	}
	return ""
}
