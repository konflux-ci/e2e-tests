package has

import (
	"fmt"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	v1 "k8s.io/api/core/v1"
)

const (
	DEFAULT_USER_PUBLIC_REPOS = "has-e2e-public"
)

var (
	PrivateComponentContainerImage string = fmt.Sprintf("quay.io/%s/quarkus:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

	// Secret Name created by spi to interact with github
	SPIGithubSecretName string = "e2e-github-secret"
)

/*
 * Component: application-service
 * Description: Contains tests about creating an application and a quarkus component from a source devfile
 */

var _ = framework.HASSuiteDescribe("[test_id:02] private devfile source", Label("has"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var oauthSecretName = ""
	var applicationName, componentName string

	// Initialize the application struct
	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	compDetected := appservice.ComponentDetectionDescription{}
	privateGitRepository := utils.GetEnv(constants.PRIVATE_DEVFILE_SAMPLE, PrivateQuarkusDevfileSource)

	var testNamespace string

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(DEFAULT_USER_PUBLIC_REPOS)
		Expect(err).NotTo(HaveOccurred())
		testNamespace = fw.UserNamespace
		Expect(testNamespace).NotTo(BeEmpty())
		// Generate names for the application and component resources
		applicationName = fmt.Sprintf(RedHatAppStudioApplicationName+"-%s", util.GenerateRandomString(10))
		componentName = fmt.Sprintf(QuarkusComponentName+"-%s", util.GenerateRandomString(10))

		credentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`
		oauthSecretName = fw.AsKubeDeveloper.SPIController.InjectManualSPIToken(testNamespace, privateGitRepository, credentials, v1.SecretTypeBasicAuth, SPIGithubSecretName)

		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			err := fw.AsKubeDeveloper.HasController.DeleteHasComponent(componentName, testNamespace, false)
			Expect(err).NotTo(HaveOccurred())

			err = fw.AsKubeDeveloper.HasController.DeleteHasApplication(applicationName, testNamespace, false)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 100*time.Millisecond).Should(BeFalse(), "Has controller didn't remove Red Hat AppStudio application gitops repository")
			Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
		}
	})

	It("creates Red Hat AppStudio Application", func() {
		createdApplication, err := fw.AsKubeDeveloper.HasController.CreateHasApplication(applicationName, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdApplication.Spec.DisplayName).To(Equal(applicationName))
		Expect(createdApplication.Namespace).To(Equal(testNamespace))
	})

	It("checks Red Hat AppStudio Application health", func() {
		Eventually(func() string {
			application, err = fw.AsKubeDeveloper.HasController.GetHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			return application.Status.Devfile
		}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

			return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
		}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
	})

	It("creates Red Hat AppStudio ComponentDetectionQuery for Component repository", func() {
		cdq, err := fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(componentName, testNamespace, QuarkusDevfileSource, "", "", "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(cdq.Name).To(Equal(componentName))
	})

	It("checks Red Hat AppStudio ComponentDetectionQuery status", func() {
		// Validate that the CDQ completes successfully
		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			cdq, err = fw.AsKubeDeveloper.HasController.GetComponentDetectionQuery(componentName, testNamespace)
			return err == nil && len(cdq.Status.ComponentDetected) > 0
		}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "ComponentDetectionQuery did not complete successfully")

		// Validate that the completed CDQ only has one detected component
		Expect(len(cdq.Status.ComponentDetected)).To(Equal(1), "Expected length of the detected Components was not 1")

		// Get the stub CDQ and validate its content
		for _, compDetected = range cdq.Status.ComponentDetected {
			Expect(compDetected.DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
			Expect(compDetected.Language).To(Equal("Java"), "Detected language was not java")
			Expect(compDetected.ProjectType).To(Equal("Quarkus"), "Detected framework was not quarkus")
		}
	})

	It("creates Red Hat AppStudio Quarkus component", func() {
		_, err := fw.AsKubeDeveloper.HasController.CreateComponentFromStub(compDetected, componentName, testNamespace, oauthSecretName, applicationName, "")
		Expect(err).NotTo(HaveOccurred())
	})
})
