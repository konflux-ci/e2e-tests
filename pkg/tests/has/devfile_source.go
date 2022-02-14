package has

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"
	"k8s.io/klog/v2"

	. "github.com/onsi/ginkgo/v2"
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

var ComponentContainerImage string = fmt.Sprintf("quay.io/redhat-appstudio-qe/quarkus:%s", strings.Replace(uuid.New().String(), "-", "", -1))

var _ = framework.HASSuiteDescribe("devfile source", func() {
	defer GinkgoRecover()
	hasController, err := has.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())
	commonController, err := common.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())
	application := &v1alpha1.Application{}

	BeforeAll(func() {
		Expect(commonController.WaitForArgoCDApplicationToBeReady(HASArgoApplicationName, GitOpsNamespace)).NotTo(HaveOccurred(), "HAS Argo application didn't start in 5 minutes")
		klog.Info("HAS Argo CD application is ready")
	})

	It("Create Red Hat AppStudio Application", func() {
		createdApplication, err := hasController.CreateHasApplication(ApplicationName, ApplicationNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdApplication.Spec.DisplayName).To(Equal(ApplicationName))
		Expect(createdApplication.Namespace).To(Equal(ApplicationNamespace))
	})

	It("Check Red Hat AppStudio Application health", func() {
		Eventually(func() string {
			application, err = hasController.GetHasApplication(ApplicationName, ApplicationNamespace)
			Expect(err).NotTo(HaveOccurred())

			return application.Status.Devfile
		}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

		gitOpsRepository := ObtainGitUrlFromDevfile(application.Status.Devfile)
		Expect(gitOpsRepository).NotTo(BeEmpty())

		// Extract repository name from the github url
		parseUrl, err := url.Parse(gitOpsRepository)
		Expect(err).NotTo(HaveOccurred())
		repoParsed := strings.Split(parseUrl.Path, "/")
		Expect(repoParsed[len(repoParsed)-1]).To(Not(BeEmpty()))

		exists := hasController.Github.CheckIfRepositoryExist(repoParsed[len(repoParsed)-1])
		Expect(exists).To(BeTrue())

		klog.Infof("GitOpsRepository generated: %s", gitOpsRepository)
	})

	It("Create Red Hat AppStudio Quarkus component", func() {
		component, err := hasController.CreateComponent(application.Name, QuarkusComponentName, ApplicationNamespace, QuarkusDevfileSource, ComponentContainerImage)
		Expect(err).NotTo(HaveOccurred())
		Expect(component.Name).To(Equal(QuarkusComponentName))
	})

	It("Wait for component pipeline to be completed", func() {
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

	It("Check component deployment health", func() {
		deployment, err := hasController.GetComponentDeployment(QuarkusComponentName, ApplicationNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(deployment.Status.AvailableReplicas).To(Equal(int32(1)))
		klog.Infof("Deployment %s is ready", deployment.Name)
	})

	It("Check component service health", func() {
		service, err := hasController.GetComponentService(QuarkusComponentName, ApplicationNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(service.Name).NotTo(BeEmpty())
		klog.Infof("Service %s is ready", service.Name)
	})

	It("Verify component route health", func() {
		route, err := hasController.GetComponentRoute(QuarkusComponentName, ApplicationNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(route.Spec.Host).To(Not(BeEmpty()))
		klog.Infof("Component route host: %s", route.Spec.Host)
	})

	It("Remove Red Hat AppStudio component", func() {
		err := hasController.DeleteHasComponent(QuarkusComponentName, ApplicationNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Delete Red Hat AppStudio Application CR", func() {
		err := hasController.DeleteHasApplication(ApplicationName, ApplicationNamespace)
		Expect(err).NotTo(HaveOccurred())
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
func ObtainGitUrlFromDevfile(devfileStatus string) string {
	appDevfile, err := devfile.ParseDevfileModel(devfileStatus)
	if err != nil {
		err = fmt.Errorf("Error parsing devfile: %v", err)
	}
	// Get the devfile attributes from the parsed object
	devfileAttributes := appDevfile.GetMetadata().Attributes
	return devfileAttributes.GetString("gitOpsRepository.url", &err)
}
