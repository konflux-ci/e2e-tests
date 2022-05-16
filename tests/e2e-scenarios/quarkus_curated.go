package e2e

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

var (
	ComponentContainerImage           string = fmt.Sprintf("quay.io/%s/test-images:%s", GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
	AppStudioE2EApplicationsNamespace string = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, "appstudio-e2e-test")
)

/*
 * Component: application-service
 * Description: Contains tests about creating an application and a quarkus component from a source devfile
 */

var _ = framework.E2ESuiteDescribe("E2E Quarkus deployment tests", func() {
	defer GinkgoRecover()

	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	// Initialize the application struct
	application := &appservice.Application{}

	BeforeAll(func() {
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
		// Check if 'has-github-token' is present, unless SKIP_HAS_SECRET_CHECK env var is set
		if !utils.CheckIfEnvironmentExists(constants.SKIP_HAS_SECRET_CHECK_ENV) {
			_, err := framework.HasController.KubeInterface().CoreV1().Secrets(RedHatAppStudioApplicationNamespace).Get(context.TODO(), ApplicationServiceGHTokenSecrName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Error checking 'has-github-token' secret %s", err)
		}

		_, err := framework.HasController.CreateTestNamespace(AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", AppStudioE2EApplicationsNamespace, err)

	})

	AfterAll(func() {
		err := framework.GitOpsController.DeleteGitOpsCR(GitOpsDeploymentName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())

		err = framework.HasController.DeleteHasComponent(QuarkusComponentName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())

		err = framework.HasController.DeleteHasApplication(RedHatAppStudioApplicationName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			gitOpsRepository := ObtainGitOpsRepositoryName(application.Status.Devfile)

			return framework.HasController.Github.CheckIfRepositoryExist(gitOpsRepository)
		}, 1*time.Minute, 100*time.Millisecond).Should(BeFalse(), "Has controller didn't remove Red Hat AppStudio application gitops repository")

	})

	It("Create Red Hat AppStudio Application", func() {
		createdApplication, err := framework.HasController.CreateHasApplication(RedHatAppStudioApplicationName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdApplication.Spec.DisplayName).To(Equal(RedHatAppStudioApplicationName))
		Expect(createdApplication.Namespace).To(Equal(AppStudioE2EApplicationsNamespace))
	})

	It("Check Red Hat AppStudio Application health", func() {
		Eventually(func() string {
			application, err = framework.HasController.GetHasApplication(RedHatAppStudioApplicationName, AppStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred())

			return application.Status.Devfile
		}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			gitOpsRepository := ObtainGitOpsRepositoryName(application.Status.Devfile)

			return framework.HasController.Github.CheckIfRepositoryExist(gitOpsRepository)
		}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
	})

	// Necessary for component pipeline
	It("Check if 'git-clone' cluster tasks exists", func() {
		Eventually(func() bool {
			return framework.CommonController.CheckIfClusterTaskExists("git-clone")
		}, 5*time.Minute, 45*time.Second).Should(BeTrue(), "'git-clone' cluster task don't exist in cluster. Component cannot be created")
	})

	It("Create Red Hat AppStudio Quarkus component", func() {
		component, err := framework.HasController.CreateComponent(application.Name, QuarkusComponentName, AppStudioE2EApplicationsNamespace, QuarkusDevfileSource, "", ComponentContainerImage)
		Expect(err).NotTo(HaveOccurred())
		Expect(component.Name).To(Equal(QuarkusComponentName))
	})

	It("Wait for component pipeline to be completed", func() {
		err := wait.PollImmediate(20*time.Second, 10*time.Minute, func() (done bool, err error) {
			pipelineRun, _ := framework.HasController.GetComponentPipeline(QuarkusComponentName, RedHatAppStudioApplicationName, AppStudioE2EApplicationsNamespace)

			for _, condition := range pipelineRun.Status.Conditions {
				klog.Infof("PipelineRun %s reason: %s", pipelineRun.Name, condition.Reason)

				if condition.Reason == "Failed" {
					return false, fmt.Errorf("Component %s pipeline failed", pipelineRun.Name)
				}

				if condition.Status == corev1.ConditionTrue {
					return true, nil
				}
			}
			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Failed component pipeline %v", err)
	})

	It("Create GitOps Deployment", func() {
		gitOpsRepository := ObtainGitOpsRepositoryUrl(application.Status.Devfile)
		gitOpsRepositoryPath := fmt.Sprintf("components/%s/base", QuarkusComponentName)

		deployment, err := framework.GitOpsController.CreateGitOpsCR(GitOpsDeploymentName, AppStudioE2EApplicationsNamespace, gitOpsRepository, gitOpsRepositoryPath, GitOpsRepositoryRevision)
		Expect(err).NotTo(HaveOccurred())
		Expect(deployment.Name).To(Equal(GitOpsDeploymentName))
	})

	It("Check GitOpsDeployment component deployment health", func() {
		Eventually(func() bool {
			deployment, _ := framework.CommonController.GetAppDeploymentByName(QuarkusComponentName, AppStudioE2EApplicationsNamespace)
			if err != nil && !errors.IsNotFound(err) {
				return false
			}
			if deployment.Status.AvailableReplicas == 1 {
				klog.Infof("Deployment %s is ready", deployment.Name)
				return true
			}

			return false
		}, 15*time.Minute, 10*time.Second).Should(BeTrue(), "Component deployment didn't become ready")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Check GitOpsDeployment image deployed is correct", func() {
		gitOpsDeployment, err := framework.CommonController.GetAppDeploymentByName(QuarkusComponentName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		depoyedImage, err := framework.GitOpsController.GetGitOpsDeployedImage(gitOpsDeployment)
		Expect(err).NotTo(HaveOccurred())
		Expect(depoyedImage).To(Equal(ComponentContainerImage))
		klog.Infof("Component deployed image: %s", depoyedImage)
	})

	It("Check GitOpsDeployment component service health", func() {
		service, err := framework.CommonController.GetServiceByName(QuarkusComponentName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(service.Name).NotTo(BeEmpty())
		klog.Infof("Service %s is ready", service.Name)
	})

	It("Check GitOpsDeployment component route health", func() {
		route, err := framework.CommonController.GetOpenshiftRoute(QuarkusComponentName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(route.Spec.Host).To(Not(BeEmpty()))
		klog.Infof("Component route host: %s", route.Spec.Host)
	})

	It("Check GitOpsDeployment backend is working porperly", func() {
		gitOpsRoute, err := framework.CommonController.GetOpenshiftRoute(QuarkusComponentName, AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred())
		err = framework.GitOpsController.CheckGitOpsEndpoint(gitOpsRoute)
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
func ObtainGitOpsRepositoryName(devfileStatus string) string {
	appDevfile, err := devfile.ParseDevfileModel(devfileStatus)
	if err != nil {
		err = fmt.Errorf("Error parsing devfile: %v", err)
	}
	// Get the devfile attributes from the parsed object
	devfileAttributes := appDevfile.GetMetadata().Attributes
	gitOpsRepository := devfileAttributes.GetString("gitOpsRepository.url", &err)
	parseUrl, err := url.Parse(gitOpsRepository)
	Expect(err).NotTo(HaveOccurred())
	repoParsed := strings.Split(parseUrl.Path, "/")

	return repoParsed[len(repoParsed)-1]
}

func ObtainGitOpsRepositoryUrl(devfileStatus string) string {
	appDevfile, err := devfile.ParseDevfileModel(devfileStatus)
	if err != nil {
		err = fmt.Errorf("Error parsing devfile: %v", err)
	}
	// Get the devfile attributes from the parsed object
	devfileAttributes := appDevfile.GetMetadata().Attributes
	gitOpsRepository := devfileAttributes.GetString("gitOpsRepository.url", &err)

	return gitOpsRepository
}

func GetQuayIOOrganization() string {
	return utils.GetEnv(constants.QUAY_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
}
