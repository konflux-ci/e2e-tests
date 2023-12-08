package stage

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"math/rand"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	// Environment Name
	EnvironmentName string = "development"
	Timeout time.Duration = 5*time.Minute
	expectedReplicas int32 = 1
	HealthEndpoint string = "/"
)


var _ = framework.RhtapStageSuiteDescribe("Stage E2E tests", g.Label("stage"), func (){
	defer g.GinkgoRecover()
	
	var err error
	source := rand.NewSource(time.Now().UnixNano())
	randomNumber := rand.New(source).Intn(9000) + 1000
	ApplicationName := fmt.Sprintf("verify-stage-app-%d", randomNumber)
	componentRepoUrl := "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	ComponentDetectionQueryName := fmt.Sprintf("%s-cdq", ApplicationName)
	app := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	fwk := &framework.Framework{}
	compDetected := appservice.ComponentDetectionDescription{}
	componentObj := &appservice.Component{}
	g.AfterEach(framework.ReportFailure(&fwk))

	var username, token, ssourl, apiurl string

	g.BeforeAll(func() {
		// Initialize the tests controllers
		token = utils.GetEnv("STAGEUSER_TOKEN", "")
		ssourl = utils.GetEnv("STAGE_SSOURL","")
		apiurl = utils.GetEnv("STAGE_APIURL", "")
		username = utils.GetEnv("STAGE_USERNAME","")
		if(token == "" && ssourl == "" && apiurl == "" && username == ""){
			g.Fail("Failed: Please set the required Stage Variables for user")
		}

		fwk, err = framework.NewFrameworkWithTimeout(username, Timeout, utils.Options{
			ToolchainApiUrl: apiurl,
			KeycloakUrl: ssourl,
			OfflineToken: token,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	g.AfterAll(func ()  {
		err := fwk.AsKubeDeveloper.HasController.DeleteAllApplicationsInASpecificNamespace(fwk.UserNamespace, 5*time.Minute)
		if err != nil {
			g.GinkgoWriter.Println("Error while deleting resources for user, got error: %v\n", err)
		}
		Expect(err).NotTo(HaveOccurred())
		err = fwk.AsKubeDeveloper.HasController.DeleteAllComponentDetectionQueriesInASpecificNamespace(fwk.UserNamespace, 5*time.Minute)
		if err != nil {
			g.GinkgoWriter.Println("while deleting component detection queries for user, got error: %v\n", err)
		}
		Expect(err).NotTo(HaveOccurred())
	})

	g.Context("Run a Simple Stage test", func() {
		g.It("Create a Application", func(){

			
			app, err := fwk.AsKubeDeveloper.HasController.CreateApplication(ApplicationName, fwk.UserNamespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(app.Spec.DisplayName).To(Equal(ApplicationName))
			Expect(app.Namespace).To(Equal(fwk.UserNamespace))


		})
		g.It("check app exists and is healthy", func(){
			Eventually(func() string {
				appstudioApp, err := fwk.AsKubeDeveloper.HasController.GetApplication(ApplicationName, fwk.UserNamespace)
				Expect(err).NotTo(HaveOccurred())
				app = appstudioApp

				return app.Status.Devfile
			}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), fmt.Sprintf("timed out waiting for gitOps repository to be created for the %s application in %s namespace", ApplicationName, fwk.UserNamespace))
		})
		g.It("Create CDQ without error", func(){
			cdq, err = fwk.AsKubeDeveloper.HasController.CreateComponentDetectionQueryWithTimeout(ComponentDetectionQueryName, fwk.UserNamespace, componentRepoUrl, "", "", "", false, Timeout)
			Expect(err).NotTo(HaveOccurred())
			
		})
		g.It("Validate CDQ object information", func() {
			// Validate that the CDQ completes successfully
			Eventually(func() (appservice.ComponentDetectionMap, error) {
				// application info should be stored even after deleting the application in application variable
				cdqStatus, err := fwk.AsKubeDeveloper.HasController.GetComponentDetectionQuery(cdq.Name, fwk.UserNamespace)
				if err != nil {
					return nil, err
				}
				return cdqStatus.Status.ComponentDetected, nil
				// Validate that the completed CDQ only has one detected component
			}, 1*time.Minute, 1*time.Second).Should(HaveLen(1), fmt.Sprintf("ComponentDetectionQuery %s/%s does not have the expected amount of components", fwk.UserNamespace, cdq.Name))

			// Get the stub CDQ and validate its content
			for _, compDetected = range cdq.Status.ComponentDetected {
				Expect(compDetected.DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
			}
		})
		g.It("Create an RHTAP Component", func(){
			
			componentObj, err = fwk.AsKubeDeveloper.HasController.CreateComponent(compDetected.ComponentStub, fwk.UserNamespace, "", "", app.Name, true, map[string]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(componentObj.Name).To(Equal(compDetected.ComponentStub.ComponentName))
		})
		g.It("Get Component and Wait for Component pipeline to finish", func() {
			componentObj, err = fwk.AsKubeDeveloper.HasController.GetComponent(componentObj.GetName(), fwk.UserNamespace)
			Expect(err).ShouldNot(HaveOccurred(), "failed to get component: %v", err)
			Expect(fwk.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(componentObj, "",
				fwk.AsKubeDeveloper.TektonController, &has.RetryOptions{Retries: 2, Always: true})).To(Succeed())
		})
		g.It(fmt.Sprintf("deploys component %s successfully using gitops", componentObj.Name), func() {
			var deployment *appsv1.Deployment
			
			Eventually(func() error {
				deployment, err = fwk.AsKubeDeveloper.CommonController.GetDeployment(componentObj.Name, fwk.UserNamespace)
				if err != nil {
					return err
				}
				if deployment.Status.AvailableReplicas != expectedReplicas {
					return fmt.Errorf("the deployment %s/%s does not have the expected amount of replicas (expected: %d, got: %d)", deployment.GetNamespace(), deployment.GetName(), expectedReplicas, deployment.Status.AvailableReplicas)
				}
				return nil
			}, 25*time.Minute, 10*time.Second).Should(Succeed(), fmt.Sprintf("timed out waiting for deployment of a component %s/%s to become ready", componentObj.GetNamespace(), componentObj.GetName()))
			
		})
		g.It(fmt.Sprintf("checks if component %s route(s) exist and health endpoint (if defined) is reachable", componentObj.Name), func() {
			
			Eventually(func() error {
				gitOpsRoute, err := fwk.AsKubeDeveloper.CommonController.GetOpenshiftRouteByComponentName(componentObj.Name, fwk.UserNamespace)
				Expect(err).NotTo(HaveOccurred())
				
					err = fwk.AsKubeDeveloper.CommonController.RouteEndpointIsAccessible(gitOpsRoute, HealthEndpoint)
					if err != nil {
						g.GinkgoWriter.Printf("Failed to request component endpoint: %+v\n retrying...\n", err)
						return err
					}
				
				return nil
			}, 5*time.Minute, 10*time.Second).Should(Succeed())
			
		})
	})
})
