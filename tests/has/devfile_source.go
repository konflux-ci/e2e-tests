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
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	DEFAULT_USER_PRIVATE_REPOS = "has-e2e-private"
)

var (
	ComponentContainerImage string = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
)

/*
 * Component: application-service
 * Description: Contains tests about creating an application and a quarkus component from a source devfile
 */

var _ = framework.HASSuiteDescribe("[test_id:01] DEVHAS-62 devfile source", Label("has"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var applicationName, componentName string

	// Initialize the application struct
	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	compDetected := appservice.ComponentDetectionDescription{}

	var testNamespace string

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(DEFAULT_USER_PRIVATE_REPOS)
		Expect(err).NotTo(HaveOccurred())
		testNamespace = fw.UserNamespace
		Expect(testNamespace).NotTo(BeEmpty())
		applicationName = fmt.Sprintf(RedHatAppStudioApplicationName+"-%s", util.GenerateRandomString(10))
		componentName = fmt.Sprintf(QuarkusComponentName+"-%s", util.GenerateRandomString(10))
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			_, err = fw.AsKubeDeveloper.HasController.GetComponentDetectionQuery(componentName, testNamespace)
			if err != nil {
				err = fw.AsKubeDeveloper.HasController.DeleteHasComponentDetectionQuery(componentName, testNamespace)
				Expect(err).NotTo(HaveOccurred())
			}
			err = fw.AsKubeDeveloper.HasController.DeleteHasApplication(applicationName, testNamespace, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())

			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 100*time.Millisecond).Should(BeFalse(), "Has controller didn't remove Red Hat AppStudio application gitops repository")
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
		_, err := fw.AsKubeDeveloper.HasController.CreateComponentDetectionQuery(componentName, testNamespace, QuarkusDevfileSource, "", "", "", false)
		Expect(err).NotTo(HaveOccurred())
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
		outputContainerImg := fmt.Sprintf("quay.io/%s/test-images:%s-%s", utils.GetQuayIOOrganization(), fw.UserName, strings.Replace(uuid.New().String(), "-", "", -1))
		_, err := fw.AsKubeDeveloper.HasController.CreateComponentFromStub(compDetected, testNamespace, outputContainerImg, "", applicationName)
		Expect(err).NotTo(HaveOccurred())
	})

	It("gitops Repository should not be deleted when component gets deleted", func() {
		comp2Detected := appservice.ComponentDetectionDescription{}
		outputContainerImg := fmt.Sprintf("quay.io/%s/test-images:%s-%s", utils.GetQuayIOOrganization(), fw.UserName, strings.Replace(uuid.New().String(), "-", "", -1))

		for _, comp2Detected = range cdq.Status.ComponentDetected {
			comp2Detected.ComponentStub.ComponentName = "java-quarkus2"
		}
		component2, err := fw.AsKubeDeveloper.HasController.CreateComponentFromStub(comp2Detected, testNamespace, outputContainerImg, "", applicationName)
		Expect(err).NotTo(HaveOccurred())

		err = fw.AsKubeDeveloper.HasController.DeleteHasComponent(component2.Name, testNamespace, false)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

			return fw.AsKubeDeveloper.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
		}, 1*time.Minute, 100*time.Millisecond).Should(BeTrue(), "Gitops repository deleted after component was deleted")
	})

	It("checks a Component gets deleted when its application is deleted", func() {
		err = fw.AsKubeDeveloper.HasController.DeleteHasApplication(applicationName, testNamespace, false)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			_, err := fw.AsKubeDeveloper.HasController.GetHasComponent(componentName, testNamespace)
			if err != nil && errors.IsNotFound(err) {
				return true
			}

			return false
		}, 10*time.Minute, 10*time.Second).Should(BeTrue(), "Component didn't get get deleted with its Application")
		Expect(err).NotTo(HaveOccurred())
	})
})
