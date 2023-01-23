package has

import (
	"context"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		_, err = framework.CommonController.CreateTestNamespace(testNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)

		credentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`
		oauthSecretName = framework.SPIController.InjectManualSPIToken(testNamespace, privateGitRepository, credentials, v1.SecretTypeBasicAuth)

		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
		// Check if 'has-github-token' is present, unless SKIP_HAS_SECRET_CHECK env var is set
		if !utils.CheckIfEnvironmentExists(constants.SKIP_HAS_SECRET_CHECK_ENV) {
			_, err := framework.HasController.KubeInterface().CoreV1().Secrets(RedHatAppStudioApplicationNamespace).Get(context.TODO(), ApplicationServiceGHTokenSecrName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Error checking 'has-github-token' secret %s", err)
		}

	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			err := framework.HasController.DeleteHasComponent(componentName, testNamespace, false)
			Expect(err).NotTo(HaveOccurred())

			err = framework.HasController.DeleteHasApplication(applicationName, testNamespace, false)
			Expect(err).NotTo(HaveOccurred())

			err = framework.SPIController.DeleteAllBindingTokensInASpecificNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return framework.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 100*time.Millisecond).Should(BeFalse(), "Has controller didn't remove Red Hat AppStudio application gitops repository")

			Expect(framework.CommonController.DeleteNamespace(testNamespace)).To(Succeed())
		}
	})

	It("creates Red Hat AppStudio Application", func() {
		createdApplication, err := framework.HasController.CreateHasApplication(applicationName, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(createdApplication.Spec.DisplayName).To(Equal(applicationName))
		Expect(createdApplication.Namespace).To(Equal(testNamespace))
	})

	It("checks Red Hat AppStudio Application health", func() {
		Eventually(func() string {
			application, err = framework.HasController.GetHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			return application.Status.Devfile
		}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

			return framework.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
		}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
	})

	// Necessary for component pipeline
	It("checks if 'git-clone' cluster tasks exists", func() {
		Eventually(func() bool {
			return framework.CommonController.CheckIfClusterTaskExists("git-clone")
		}, 5*time.Minute, 45*time.Second).Should(BeTrue(), "'git-clone' cluster task don't exist in cluster. Component cannot be created")
	})

	It("creates Red Hat AppStudio ComponentDetectionQuery for Component repository", func() {
		cdq, err := framework.HasController.CreateComponentDetectionQuery(componentName, testNamespace, QuarkusDevfileSource, "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(cdq.Name).To(Equal(componentName))

	})

	It("checks Red Hat AppStudio ComponentDetectionQuery status", func() {
		// Validate that the CDQ completes successfully
		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			cdq, err = framework.HasController.GetComponentDetectionQuery(componentName, testNamespace)
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
		component, err := framework.HasController.CreateComponentFromStub(compDetected, componentName, testNamespace, oauthSecretName, applicationName)
		Expect(err).NotTo(HaveOccurred())
		Expect(component.Name).To(Equal(componentName))
	})
})
