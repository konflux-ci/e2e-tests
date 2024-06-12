package pipelines

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

const (
	fbcServiceAccountName   = "release-service-account"
	fbcSourceGitURL         = "https://github.com/redhat-appstudio-qe/fbc-sample-repo"
	fbcDockerFilePath       = "catalog.Dockerfile"
	targetPort              = 50051
	relSvcCatalogPathInRepo = "pipelines/fbc-release/fbc-release.yaml"
)

var snapshot *appservice.Snapshot
var releaseCR *releaseapi.Release
var buildPR *tektonv1.PipelineRun
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

			component = releasecommon.CreateComponent(*devFw, devNamespace, fbcApplicationName, fbcComponentName, fbcSourceGitURL, "", "4.13", fbcDockerFilePath, constants.DefaultFbcBuilderPipelineBundle)

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
				assertBuildPipelineRunSucceeded(*devFw, devNamespace, managedNamespace, fbcApplicationName, component)
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

			component = releasecommon.CreateComponent(*devFw, devNamespace, fbcHotfixAppName, fbcHotfixCompName, fbcSourceGitURL, "", "4.13", fbcDockerFilePath, constants.DefaultFbcBuilderPipelineBundle)

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
				assertBuildPipelineRunSucceeded(*devFw, devNamespace, managedNamespace, fbcHotfixAppName, component)
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

			component = releasecommon.CreateComponent(*devFw, devNamespace, fbcPreGAAppName, fbcPreGACompName, fbcSourceGitURL, "", "4.13", fbcDockerFilePath, constants.DefaultFbcBuilderPipelineBundle)

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
				assertBuildPipelineRunSucceeded(*devFw, devNamespace, managedNamespace, fbcPreGAAppName, component)
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

func assertBuildPipelineRunSucceeded(devFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, component *appservice.Component) {
	dFw := releasecommon.NewFramework(devWorkspace)
	devFw = *dFw
	// Create a ticker that ticks every 3 minutes
	ticker := time.NewTicker(3 * time.Minute)
	// Schedule the stop of the ticker after 5 minutes
	time.AfterFunc(5*time.Minute, func() {
		ticker.Stop()
		fmt.Println("Stopped executing every 3 minutes.")
	})
	// Run a goroutine to handle the ticker ticks
	go func() {
		for range ticker.C {
			dFw = releasecommon.NewFramework(devWorkspace)
			devFw = *dFw
		}
	}()
	Eventually(func() error {
		buildPR, err = devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(component.Name, fbcAppName, devNamespace, "")
		if err != nil {
			GinkgoWriter.Printf("Build PipelineRun has not been created yet for the component %s/%s\n", devNamespace, component.Name)
			return err
		}
		if !buildPR.IsDone() {
			return fmt.Errorf("build pipelinerun %s in namespace %s did not finish yet", buildPR.Name, buildPR.Namespace)
		}
		if buildPR.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
			snapshot, err = devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPR.Name, "", devNamespace)
			if err != nil {
				return err
			}
			return nil
		} else {
			return fmt.Errorf(tekton.GetFailedPipelineRunLogs(devFw.AsKubeDeveloper.HasController.KubeRest(), devFw.AsKubeDeveloper.HasController.KubeInterface(), buildPR))
		}
	}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the build PipelineRun to be finished for the component %s/%s", devNamespace, component.Name))
}

func assertReleasePipelineRunSucceeded(devFw, managedFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, component *appservice.Component) {
	snapshot, err = devFw.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", component.Name, devNamespace)
	Expect(err).ToNot(HaveOccurred())
	GinkgoWriter.Println("snapshot: ", snapshot.Name)
	Eventually(func() error {
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

	Expect(mFw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
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
			Exclude: []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			Include: []string{"@slsa3"},
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
