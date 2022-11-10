package has

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	v1 "k8s.io/api/core/v1"
)

var (
	PrivateComponentContainerImage string = fmt.Sprintf("quay.io/%s/quarkus:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
)

/*
 * Component: application-service
 * Description: Contains tests about creating an application and a quarkus component from a source devfile
 */

var _ = framework.HASSuiteDescribe("[test_id:02] private devfile source", Label("has"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())
	var oauthSecretName = ""
	var applicationName, componentName, testNamespace string

	// Initialize the application struct
	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	compDetected := appservice.ComponentDetectionDescription{}
	privateGitRepository := utils.GetEnv(constants.PRIVATE_DEVFILE_SAMPLE, PrivateQuarkusDevfileSource)

	BeforeAll(func() {
		testNamespace = utils.GetGeneratedNamespace("has-e2e")
		// Generate names for the application and component resources
		applicationName = fmt.Sprintf(RedHatAppStudioApplicationName+"-%s", util.GenerateRandomString(10))
		componentName = fmt.Sprintf(QuarkusComponentName+"-%s", util.GenerateRandomString(10))

		_, err = framework.AppstudioUser.CommonController.CreateTestNamespace(testNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)
		credentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`
		oauthSecretName = framework.AppstudioUser.SPIController.InjectManualSPIToken(testNamespace, privateGitRepository, credentials, v1.SecretTypeBasicAuth)
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
	})

	AfterAll(func() {
		err := framework.AppstudioUser.HasController.DeleteHasComponent(componentName, testNamespace, false)
		Expect(err).NotTo(HaveOccurred())

		err = framework.AppstudioUser.HasController.DeleteHasApplication(applicationName, testNamespace, false)
		Expect(err).NotTo(HaveOccurred())

		err = framework.AppstudioUser.SPIController.DeleteAllBindingTokensInASpecificNamespace(testNamespace)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

			return framework.AppstudioUser.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
		}, 1*time.Minute, 100*time.Millisecond).Should(BeFalse(), "Has controller didn't remove Red Hat AppStudio application gitops repository")

		Expect(framework.AppstudioUser.CommonController.DeleteNamespace(testNamespace)).To(Succeed())
	})

	It("Create Red Hat AppStudio Application", func() {
		createdApplication, err := framework.AppstudioUser.HasController.CreateHasApplication(applicationName, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdApplication.Spec.DisplayName).To(Equal(applicationName))
		Expect(createdApplication.Namespace).To(Equal(testNamespace))
	})

	It("Check Red Hat AppStudio Application health", func() {
		Eventually(func() string {
			application, err = framework.AppstudioUser.HasController.GetHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			return application.Status.Devfile
		}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

			return framework.AppstudioUser.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
		}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
	})

	It("Create Red Hat AppStudio ComponentDetectionQuery for Component repository", func() {
		cdq, err := framework.AppstudioUser.HasController.CreateComponentDetectionQuery(componentName, testNamespace, QuarkusDevfileSource, "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(cdq.Name).To(Equal(componentName))

	})

	It("Check Red Hat AppStudio ComponentDetectionQuery status", func() {
		// Validate that the CDQ completes successfully
		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			cdq, err = framework.AppstudioUser.HasController.GetComponentDetectionQuery(componentName, testNamespace)
			return err == nil && len(cdq.Status.ComponentDetected) > 0
		}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "ComponentDetectionQuery did not complete successfully")

		// Validate that the completed CDQ only has one detected component
		Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

		// Get the stub CDQ and validate its content
		for _, compDetected = range cdq.Status.ComponentDetected {
			Expect(compDetected.DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
			Expect(compDetected.Language).To(Equal("java"), "Detected language was not java")
			Expect(compDetected.ProjectType).To(Equal("quarkus"), "Detected framework was not quarkus")
		}
	})

	It("Create Red Hat AppStudio Quarkus component", func() {
		component, err := framework.AppstudioUser.HasController.CreateComponentFromStub(compDetected, componentName, testNamespace, oauthSecretName, applicationName)
		Expect(err).NotTo(HaveOccurred())
		Expect(component.Name).To(Equal(componentName))
	})
})
