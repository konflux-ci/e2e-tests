package pipelines

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releasecommon "github.com/redhat-appstudio/e2e-tests/tests/release"
	releaseapi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	fbcServiceAccountName   = "release-service-account"
	fbcSourceGitURL         = "https://github.com/redhat-appstudio-qe/fbc-sample-repo"
	targetPort              = 50051
	relSvcCatalogPathInRepo = "pipelines/fbc-release/fbc-release.yaml"
)

var snapshot *appservice.Snapshot
var releaseCR *releaseapi.Release
var releasePr, buildPr *tektonv1.PipelineRun
var err error
var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)
var devFw *framework.Framework
var mFw *framework.Framework
var managedFw *framework.Framework

var _ = framework.ReleasePipelinesSuiteDescribe("FBC e2e-tests", Label("release-pipelines", "fbc-tests"), func() {
	defer GinkgoRecover()

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var issueId = "bz12345"
	var productName = "preGA-product"
	var productVersion = "v2"
	var fbcApplicationName = "fbc-pipelines-app-" + util.GenerateRandomString(4)
	var fbcHotfixAppName = "fbc-hotfix-app-" + util.GenerateRandomString(4)
	var fbcPreGAAppName = "fbc-prega-app-" + util.GenerateRandomString(4)
	var fbcComponentName = "fbc-pipelines-comp-" + util.GenerateRandomString(4)
	var fbcHotfixCompName = "fbc-hotfix-comp-" + util.GenerateRandomString(4)
	var fbcPreGACompName = "fbc-prega-comp-" + util.GenerateRandomString(4)
	var fbcReleasePlanName = "fbc-pipelines-rp-" + util.GenerateRandomString(4)
	var fbcHotfixRPName = "fbc-hotfix-rp-" + util.GenerateRandomString(4)
	var fbcPreGARPName = "fbc-prega-rp-" + util.GenerateRandomString(4)
	var fbcReleasePlanAdmissionName = "fbc-pipelines-rpa-" + util.GenerateRandomString(4)
	var fbcHotfixRPAName = "fbc-hotfix-rpa-" + util.GenerateRandomString(4)
	var fbcPreGARPAName = "fbc-prega-rpa-" + util.GenerateRandomString(4)
	var fbcEnterpriseContractPolicyName = "fbc-pipelines-policy-" + util.GenerateRandomString(4)
	var fbcHotfixECPolicyName = "fbc-hotfix-policy-" + util.GenerateRandomString(4)
	var fbcPreGAECPolicyName = "fbc-prega-policy-" + util.GenerateRandomString(4)

	AfterEach(framework.ReportFailure(&devFw))

	Describe("with FBC happy path", Label("fbcHappyPath"), func() {
		var component *appservice.Component
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			managedNamespace = managedFw.UserNamespace

			// Linking the build secret to the pipeline service account in dev namespace.
			err = devFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, releasecommon.HacbsReleaseTestsTokenSecret, constants.DefaultPipelineServiceAccount, true)
			Expect(err).ToNot(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("Created application :", fbcApplicationName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcReleasePlanName, devNamespace, fbcApplicationName, managedNamespace, "true", nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcEnterpriseContractPolicyName, relSvcCatalogPathInRepo, "false", "", "", "", "")
			component = releasecommon.CreateComponentByCDQ(*devFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponentName, fbcSourceGitURL)
			createFBCEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)

		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
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
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcHotfixAppName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("Created application :", fbcHotfixAppName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcHotfixRPName, devNamespace, fbcHotfixAppName, managedNamespace, "true", nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcHotfixRPAName, *managedFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcHotfixECPolicyName, relSvcCatalogPathInRepo, "true", issueId, "false", "", "")
			component = releasecommon.CreateComponentByCDQ(*devFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcHotfixCompName, fbcSourceGitURL)
			createFBCEnterpriseContractPolicy(fbcHotfixECPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcHotfixAppName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcHotfixECPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcHotfixRPAName, managedNamespace, false)).NotTo(HaveOccurred())
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

	Describe("with FBC pre-GA process", Label("fbcPreGA"), func() {
		var component *appservice.Component

		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcPreGAAppName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("Created application :", fbcPreGAAppName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcPreGARPName, devNamespace, fbcPreGAAppName, managedNamespace, "true", nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcPreGARPAName, *managedFw, devNamespace, managedNamespace, fbcPreGAAppName, fbcPreGAECPolicyName, relSvcCatalogPathInRepo, "false", issueId, "true", productName, productVersion)
			component = releasecommon.CreateComponentByCDQ(*devFw, devNamespace, managedNamespace, fbcPreGAAppName, fbcPreGACompName, fbcSourceGitURL)
			createFBCEnterpriseContractPolicy(fbcPreGAECPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			if !CurrentSpecReport().Failed() {
				Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcPreGAAppName, devNamespace, false)).NotTo(HaveOccurred())
				Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcPreGAECPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
				Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcPreGARPAName, managedNamespace, false)).NotTo(HaveOccurred())
			}
		})

		var _ = Describe("FBC pre-GA post-release verification", func() {
			It("verifies that a build PipelineRun is created in dev namespace and succeeds", func() {
				assertBuildPipelineRunCreated(*devFw, devNamespace, managedNamespace, fbcPreGAAppName, component)
			})

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(*devFw, *managedFw, devNamespace, managedNamespace, fbcPreGAAppName, component)
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(*devFw, devNamespace, managedNamespace, fbcPreGAAppName, component)
			})
		})
	})
})

func assertBuildPipelineRunCreated(devFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, component *appservice.Component) {
	Expect(devFw.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(component, "", devFw.AsKubeDeveloper.TektonController, &has.RetryOptions{Retries: 3, Always: true}, nil)).To(Succeed())
	buildPr, err = devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(component.Name, fbcAppName, devNamespace, "")
	Expect(err).ShouldNot(HaveOccurred())
}

func assertReleasePipelineRunSucceeded(devFw, managedFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, component *appservice.Component) {
	Eventually(func() error {
		snapshot, err = devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPr.Name, "", devNamespace)
		if err != nil {
			return err
		}
		GinkgoWriter.Println("snapshot: ", snapshot.Name)
		releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
		if err != nil {
			return err
		}
		GinkgoWriter.Println("Release CR: ", releaseCR.Name)
		return nil
	}, 5*time.Minute, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for Snapshot and Release being created")

	mFw = releasecommon.NewFramework(managedWorkspace)
	// Create a ticker that ticks every 3 minutes
	ticker := time.NewTicker(3 * time.Minute)
	// Schedule the stop of the ticker after 15 minutes
	time.AfterFunc(15*time.Minute, func() {
		ticker.Stop()
		fmt.Println("Stopped executing every 3 minutes.")
	})
	// Run a goroutine to handle the ticker ticks
	go func() {
		for range ticker.C {
			mFw = releasecommon.NewFramework(managedWorkspace)
		}
	}()

	Eventually(func() error {
		releasePr, err = mFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(mFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
		if err != nil {
			return err
		}
		Expect(err).ShouldNot(HaveOccurred())
		GinkgoWriter.Println("Release PR: ", releasePr.Name)

		if !releasePr.IsDone() {
			return fmt.Errorf("release pipelinerun %s in namespace %s did not finish yet", releasePr.Name, releasePr.Namespace)
		}
		Expect(tekton.HasPipelineRunSucceeded(releasePr)).To(BeTrue(), fmt.Sprintf("release pipelinerun %s/%s did not succeed", releasePr.GetNamespace(), releasePr.GetName()))
		GinkgoWriter.Println("Release PR succeeded")
		return nil
	}, releasecommon.ReleasePipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for release pipelinerun to succeed")
}

func assertReleaseCRSucceeded(devFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, component *appservice.Component) {
	dFw := releasecommon.NewFramework(devWorkspace)
	Eventually(func() error {
		releaseCR, err = dFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
		if err != nil {
			return err
		}
		conditions := releaseCR.Status.Conditions
		if len(conditions) > 0 {
			for _, c := range conditions {
				if c.Type == "Released" && c.Status == "True" {
					GinkgoWriter.Println("Release CR is released")
				}
			}
		}

		if !releaseCR.IsReleased() {
			return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
		}
		return nil
	}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
}

func createFBCEnterpriseContractPolicy(fbcECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
	defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
		Description: "Red Hat's enterprise requirements",
		PublicKey:   "k8s://openshift-pipelines/public-key",
		Sources: []ecp.Source{{
			Name:   "Default",
			Policy: []string{releasecommon.EcPolicyLibPath, releasecommon.EcPolicyReleasePath},
			Data:   []string{releasecommon.EcPolicyDataBundle, releasecommon.EcPolicyDataPath},
		}},
		Configuration: &ecp.EnterpriseContractPolicyConfiguration{
			Exclude: []string{"cve", "step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			Include: []string{"minimal", "slsa3"},
		},
	}

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(fbcECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createFBCReleasePlanAdmission(fbcRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, fbcAppName, fbcECPName, pathInRepoValue, hotfix, issueId, preGA, productName, productVersion string) {
	var err error
	data, err := json.Marshal(map[string]interface{}{
		"fbc": map[string]interface{}{
			"fromIndex":                       constants.FromIndex,
			"targetIndex":                     constants.TargetIndex,
			"binaryImage":                     constants.BinaryImage,
			"publishingCredentials":           "fbc-preview-publishing-credentials",
			"iibServiceConfigSecret":          "iib-preview-services-config",
			"iibOverwriteFromIndexCredential": "iib-overwrite-fromimage-credentials",
			"requestUpdateTimeout":            "1500",
			"buildTimeoutSeconds":             "1500",
			"hotfix":                          hotfix,
			"issueId":                         issueId,
			"preGA":                           preGA,
			"productName":                     productName,
			"productVersion":                  productVersion,
			"allowedPackages":                 []string{"example-operator"},
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
