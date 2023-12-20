package pipelines

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releasecommon "github.com/redhat-appstudio/e2e-tests/tests/release"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	fbcApplicationName              = "fbc-pipelines-aplication"
	fbcHotfixAppName                = "fbc-hotfix-aplication"
	fbcComponentName                = "fbc-pipelines-component"
	fbcHotfixCompName               = "fbc-hotfix-component"
	fbcReleasePlanName              = "fbc-pipelines-releaseplan"
	fbcHotfixRPName                 = "fbc-hotfix-releaseplan"
	fbcReleasePlanAdmissionName     = "fbc-pipelines-releaseplanadmission"
	fbcHotfixRPAName                = "fbc-hotfix-releaseplanadmission"
	fbcEnterpriseContractPolicyName = "fbc-pipelines-policy"
	fbcHotfixECPolicyName           = "fbc-hotfix-policy"
	fbcServiceAccountName           = "release-service-account"
	fbcSourceGitURL                 = "https://github.com/redhat-appstudio-qe/fbc-sample-repo"
	targetPort                      = 50051
	relSvcCatalogPathInRepo         = "pipelines/fbc-release/fbc-release.yaml"
	ecPolicyLibPath                 = "github.com/enterprise-contract/ec-policies//policy/lib"
	ecPolicyReleasePath             = "github.com/enterprise-contract/ec-policies//policy/release"
	ecPolicyDataBundle              = "oci::quay.io/redhat-appstudio-tekton-catalog/data-acceptable-bundles:latest"
	ecPolicyDataPath                = "github.com/release-engineering/rhtap-ec-policy//data"
)

var _ = framework.ReleasePipelinesSuiteDescribe("FBC e2e-tests", Label("release-pipelines", "fbc-tests"), func() {
	defer GinkgoRecover()

	var devWorkspace = os.Getenv(constants.RELEASE_DEV_WORKSPACE_ENV)
	var managedWorkspace = os.Getenv(constants.RELEASE_MANAGED_WORKSPACE_ENV)
	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var issueId = "bz12345"

	AfterEach(framework.ReportFailure(&devFw))

	stageOptions := utils.Options{
		ToolchainApiUrl: os.Getenv(constants.TOOLCHAIN_API_URL_ENV),
		KeycloakUrl:     os.Getenv(constants.KEYLOAK_URL_ENV),
		OfflineToken:    os.Getenv(constants.OFFLINE_TOKEN_ENV),
	}

	Describe("with FBC happy path", Label("fbcHappyPath"), func() {
		var component *appservice.Component
		BeforeAll(func() {

			devFw, err = framework.NewFrameworkWithTimeout(
				devWorkspace,
				time.Minute*60,
				stageOptions,
			)
			Expect(err).NotTo(HaveOccurred())

			managedFw, err = framework.NewFrameworkWithTimeout(
				managedWorkspace,
				time.Minute*60,
				stageOptions,
			)
			Expect(err).NotTo(HaveOccurred())
			managedNamespace = managedFw.UserNamespace

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcReleasePlanName, devNamespace, fbcApplicationName, managedNamespace, "true")
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcEnterpriseContractPolicyName, relSvcCatalogPathInRepo, "false", "")
			component = releasecommon.CreateComponentByCDQ(*devFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponentName, fbcSourceGitURL)
			createFBCEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcApplicationName, devNamespace, false)).NotTo(HaveOccurred())
				Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
				Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
			}
		})

		var _ = Describe("Post-release verification", func() {
			It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
				assertBuildPipelineRunCreated(*devFw, devNamespace, managedNamespace, fbcApplicationName, component)
			})

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(*devFw, *managedFw, devNamespace, managedNamespace, fbcApplicationName, component)
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(*devFw, devNamespace, managedNamespace, fbcApplicationName, component)
			})
		})
	})

	Describe("with FBC hotfix process", Label("fbcHotfix"), func() {
		var component *appservice.Component

		BeforeAll(func() {
			devFw, err = framework.NewFrameworkWithTimeout(
				devWorkspace,
				time.Minute*60,
				stageOptions,
			)
			Expect(err).NotTo(HaveOccurred())

			managedFw, err = framework.NewFrameworkWithTimeout(
				managedWorkspace,
				time.Minute*60,
				stageOptions,
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcHotfixAppName, devNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcHotfixRPName, devNamespace, fbcHotfixAppName, managedNamespace, "true")
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcHotfixRPAName, *managedFw, devNamespace, managedNamespace,fbcHotfixAppName, fbcHotfixECPolicyName, relSvcCatalogPathInRepo, "true", issueId)
			component = releasecommon.CreateComponentByCDQ(*devFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcHotfixCompName, fbcSourceGitURL)
			createFBCEnterpriseContractPolicy(fbcHotfixECPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcHotfixAppName, devNamespace, false)).NotTo(HaveOccurred())
				Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcHotfixECPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
				Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcHotfixRPAName, managedNamespace, false)).NotTo(HaveOccurred())
			}
		})

		var _ = Describe("FBC hotfix post-release verification", func() {
			It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
				assertBuildPipelineRunCreated(*devFw, devNamespace, managedNamespace, fbcHotfixAppName, component)
			})

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(*devFw, *managedFw, devNamespace, managedNamespace, fbcHotfixAppName, component)
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(*devFw, devNamespace, managedNamespace, fbcHotfixAppName, component)
			})
		})
	})
})

func assertBuildPipelineRunCreated(devFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, component *appservice.Component) {
	Expect(devFw.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(component, "", devFw.AsKubeDeveloper.TektonController, &has.RetryOptions{Retries: 3, Always: true})).To(Succeed())
	_, err := devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(component.Name, fbcAppName, devNamespace, "")
	Expect(err).ShouldNot(HaveOccurred())

}

func assertReleasePipelineRunSucceeded(devFw, managedFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, component *appservice.Component) {
	buildPr, err := devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(component.Name, fbcAppName, devNamespace, "")
	Expect(err).ShouldNot(HaveOccurred())
	Eventually(func() error {
		snapshot, err := devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPr.Name, "", devNamespace)
		if err != nil {
			return fmt.Errorf("snapshot %s in namespace %s has not been found yet", snapshot.Name, devNamespace)
		}
		releaseCR, err := devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
		if err != nil {
			return fmt.Errorf("release %s in namespace %s has not been found yet", releaseCR.Name, managedNamespace)
		}
		Expect(err).ShouldNot(HaveOccurred())

		releasePr, err := managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
		Expect(err).ShouldNot(HaveOccurred())
		if !releasePr.IsDone() {
			return fmt.Errorf("release pipelinerun %s in namespace %s did not finish yet", releasePr.Name, releasePr.Namespace)
		}
		GinkgoWriter.Println("Release PR: ", releasePr.Name)
		Expect(tekton.HasPipelineRunSucceeded(releasePr)).To(BeTrue(), fmt.Sprintf("release pipelinerun %s/%s did not succeed", releasePr.GetNamespace(), releasePr.GetName()))
		return nil
	}, releasecommon.ReleasePipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for release pipelinerun to succeed")
}

func assertReleaseCRSucceeded(devFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, component *appservice.Component) {
	Eventually(func() error {
		buildPr, err := devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(component.Name, fbcAppName, devNamespace, "")
		Expect(err).ShouldNot(HaveOccurred())
		snapshot, err := devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPr.Name , "", devNamespace)
		Expect(err).ShouldNot(HaveOccurred())
		releaseCR, err := devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
		if err != nil {
			return err
		}
		GinkgoWriter.Println("Release CR: ", releaseCR.Name)
		if !releaseCR.IsReleased() {
			return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
		}
		return nil
	}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
}

func createFBCEnterpriseContractPolicy(fbcECPName string,managedFw framework.Framework, devNamespace, managedNamespace string) {
	defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
		Description: "Red Hat's enterprise requirements",
		PublicKey:   "k8s://openshift-pipelines/public-key",
		Sources: []ecp.Source{{
			Name:   "Default",
			Policy: []string{ecPolicyLibPath, ecPolicyReleasePath},
			Data:   []string{ecPolicyDataBundle, ecPolicyDataPath},
		}},
		Configuration: &ecp.EnterpriseContractPolicyConfiguration{
			Exclude:     []string{"cve", "step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			Include:     []string{"minimal"},
		},
	}

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(fbcECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createFBCReleasePlanAdmission(fbcRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, fbcAppName, fbcECPName, pathInRepoValue, hotfix, issueId string) {
	var err error
	data, err := json.Marshal(map[string]interface{}{
		"fbc": map[string]interface{}{
			"fromIndex":            constants.FromIndex,
			"targetIndex":          constants.TargetIndex,
			"binaryImage":          constants.BinaryImage,
			"publishingCredentials": "fbc-preview-publishing-credentials",
			"iibServiceConfigSecret": "iib-preview-services-config",
			"iibOverwriteFromIndexCredential": "iib-preview-overwritefromimage-credential",
			"requestUpdateTimeout": "420",
			"buildTimeoutSeconds":  "480",
			"hotfix": hotfix,
			"issueId": issueId,
		},
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(fbcRPAName, managedNamespace, "", devNamespace, fbcECPName, fbcServiceAccountName, []string{fbcAppName}, true, &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasecommon.RelSvcCatalogURL},
			{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
			{Name: "pathInRepo", Value: pathInRepoValue},
		},
	}, &runtime.RawExtension{
		Raw: data,
	})
	Expect(err).NotTo(HaveOccurred())
}
