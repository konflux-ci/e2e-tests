package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/tests/e2e-demos/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	// The name of the SPIAccessTokenBinding resource that the HAS e2e tests will create
	SPIAccessTokenBindingName string = "has-private-git-repo-binding" // #nosec

	// The name of the secret to be created by the SPIAccessTokenBinding resource
	SPIAccessTokenSecretName string = "has-private-git-repo-secret" // #nosec

	// Valid container with a quarkus image to import in appstudio.Using to test a component imported from quay.io
	containerImageSource = "quay.io/redhat-appstudio-qe/test-images:7ac98d2c0ff64671baa54d4a94675601"
)

var _ = framework.E2ESuiteDescribe("magic", func() {
	defer GinkgoRecover()

	// Initialize the application struct
	application := &appservice.Application{}
	component := &appservice.Component{}

	// Initialize the tests controllers /home/flacatusu/WORKSPACE/appstudio-qe/e2e-tests/tests/e2e-demos/config
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())
	configTest, err := LoadTestGeneratorConfig()
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		// Check to see if the github token was provided
		Expect(utils.CheckIfEnvironmentExists(constants.GITHUB_TOKEN_ENV)).Should(BeTrue(), "%s environment variable is not set", constants.GITHUB_TOKEN_ENV)
		// Check if 'has-github-token' is present, unless SKIP_HAS_SECRET_CHECK env var is set
		if !utils.CheckIfEnvironmentExists(constants.SKIP_HAS_SECRET_CHECK_ENV) {
			_, err := framework.HasController.KubeInterface().CoreV1().Secrets(RedHatAppStudioApplicationNamespace).Get(context.TODO(), ApplicationServiceGHTokenSecrName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Error checking 'has-github-token' secret %s", err)
		}

		_, err := framework.CommonController.CreateTestNamespace(AppStudioE2EApplicationsNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", AppStudioE2EApplicationsNamespace, err)
	})

	for _, appTest := range configTest.Tests {
		appTest := appTest

		It("Create Application", func() {
			createdApplication, err := framework.HasController.CreateHasApplication(appTest.ApplicationName, AppStudioE2EApplicationsNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdApplication.Spec.DisplayName).To(Equal(appTest.ApplicationName))
			Expect(createdApplication.Namespace).To(Equal(AppStudioE2EApplicationsNamespace))
		})

		It("Application Health", func() {
			Eventually(func() string {
				appstudioApp, err := framework.HasController.GetHasApplication(appTest.ApplicationName, AppStudioE2EApplicationsNamespace)
				Expect(err).NotTo(HaveOccurred())
				application = appstudioApp

				return application.Status.Devfile
			}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), "Error creating gitOps repository")

			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return framework.HasController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), "Has controller didn't create gitops repository")
		})

		for _, componentTest := range appTest.Components {
			componentTest := componentTest

			var containerIMG = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

			It(fmt.Sprintf("create component %s", componentTest.Name), func() {
				if componentTest.ContainerSource != "" {
					var outputContainerImage = ""
					component, err = framework.HasController.CreateComponent(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace, "", containerImageSource, outputContainerImage, "")
					Expect(err).NotTo(HaveOccurred())
				} else {
					if componentTest.Type == "private" {
						createAndInjectTokenToSPI(framework, utils.GetEnv(constants.GITHUB_TOKEN_ENV, ""), componentTest.DevfileSample, SPIAccessTokenBindingName, AppStudioE2EApplicationsNamespace, SPIAccessTokenSecretName)
						component, err = framework.HasController.CreateComponent(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace, componentTest.DevfileSample, containerIMG, "", SPIAccessTokenSecretName)
					} else {
						component, err = framework.HasController.CreateComponentFromDevfileSource(application.Name, componentTest.Name, AppStudioE2EApplicationsNamespace, componentTest.DevfileSample, "", containerIMG, "")
					}
				}
				Expect(err).NotTo(HaveOccurred())
				Expect(component.Name).To(Equal(component.Name))
			})

			It(fmt.Sprintf("wait component %s pipeline to be finished", componentTest.Name), func() {
				if componentTest.ContainerSource != "" {
					Skip("component %s was imported from quay.io/docker.io source. Skiping pipelinerun check.")
				}
				Expect(framework.HasController.WaitForComponentPipelineToBeFinished(component.Name, application.Name, AppStudioE2EApplicationsNamespace)).NotTo(HaveOccurred(), "Failed component pipeline %v", err)
			})

			It(fmt.Sprintf("deploy component %s using gitops api", componentTest.Name), func() {
				gitOpsRepository := utils.ObtainGitOpsRepositoryUrl(application.Status.Devfile)
				gitOpsRepositoryPath := fmt.Sprintf("components/%s/base", componentTest.Name)

				_, err := framework.GitOpsController.CreateGitOpsCR(GitOpsDeploymentName, AppStudioE2EApplicationsNamespace, gitOpsRepository, gitOpsRepositoryPath, GitOpsRepositoryRevision)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					deployment, _ := framework.CommonController.GetAppDeploymentByName(componentTest.Name, AppStudioE2EApplicationsNamespace)
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
		}
	}
})

func LoadTestGeneratorConfig() (config.WorkflowSpec, error) {
	c := config.WorkflowSpec{}
	// Open config file
	file, err := os.Open(filepath.Clean("/home/flacatusu/WORKSPACE/appstudio-qe/e2e-tests/tests/e2e-demos/config/default.yaml"))
	if err != nil {
		return c, err
	}

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	return c, nil
}

// createAndInjectTokenToSPI creates the specified SPIAccessTokenBinding resource and injects the specified token into it.
func createAndInjectTokenToSPI(framework *framework.Framework, token, repoURL, spiAccessTokenBindingName, namespace, secretName string) {
	// Get the token for the current openshift user
	tokenBytes, err := exec.Command("oc", "whoami", "--show-token").Output()
	Expect(err).NotTo(HaveOccurred())
	bearerToken := strings.TrimSuffix(string(tokenBytes), "\n")

	// Create the SPI Access Token Binding resource and upload the token for the private repository
	_, err = framework.SPIController.CreateSPIAccessTokenBinding(spiAccessTokenBindingName, namespace, repoURL, secretName)
	Expect(err).NotTo(HaveOccurred())

	// Wait for the resource to be in the "AwaitingTokenData" phase
	var spiAccessTokenBinding *v1beta1.SPIAccessTokenBinding
	Eventually(func() bool {
		// application info should be stored even after deleting the application in application variable
		spiAccessTokenBinding, err = framework.SPIController.GetSPIAccessTokenBinding(spiAccessTokenBindingName, namespace)
		if err != nil {
			return false
		}
		return spiAccessTokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseInjected || (spiAccessTokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData && spiAccessTokenBinding.Status.OAuthUrl != "")
	}, 1*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI controller didn't set SPIAccessTokenBinding to AwaitingTokenData/Injected")

	if spiAccessTokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData {
		// If the phase is AwaitingTokenData then manually inject the git token
		// Get the oauth url and linkedAccessTokenName from the spiaccesstokenbinding resource
		oauthURL := spiAccessTokenBinding.Status.OAuthUrl
		parsedOAuthURL, err := url.Parse(oauthURL)
		Expect(err).NotTo(HaveOccurred())
		oauthHost := parsedOAuthURL.Host
		linkedAccessTokenName := spiAccessTokenBinding.Status.LinkedAccessTokenName

		// Before injecting the token, validate that the linkedaccesstoken resource exists, otherwise injecting will return a 404 error code
		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			_, err := framework.SPIController.GetSPIAccessToken(linkedAccessTokenName, namespace)
			return err == nil
		}, 1*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI controller didn't create the SPIAccessToken")

		// Now that the spiaccesstokenbinding is in the AwaitingTokenData phase, inject the GitHub token
		var bearer = "Bearer " + string(bearerToken)
		var jsonStr = []byte(`{"access_token":"` + token + `"}`)
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		req, err := http.NewRequest("POST", "https://"+oauthHost+"/token/"+namespace+"/"+linkedAccessTokenName, bytes.NewBuffer(jsonStr))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Add("Authorization", bearer)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).Should(Equal(204))
		defer resp.Body.Close()

		// Check to see if the token was successfully injected
		Eventually(func() bool {
			// application info should be stored even after deleting the application in application variable
			spiAccessTokenBinding, err = framework.SPIController.GetSPIAccessTokenBinding(spiAccessTokenBindingName, namespace)
			return err == nil && spiAccessTokenBinding.Status.Phase == v1beta1.SPIAccessTokenBindingPhaseInjected
		}, 1*time.Minute, 100*time.Millisecond).Should(BeTrue(), "SPI controller didn't set SPIAccessTokenBinding to Injected")
	}
}
