package pipelines

import (
	"encoding/json"
	"fmt"
	"time"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/devfile/library/v2/pkg/util"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	fbcSourceGitURL            = "https://github.com/redhat-appstudio-qe/fbc-sample-repo-test"
	fbcCompRepoName            = "fbc-sample-repo-test"
	fbcCompRevision            = "94d5b8ccbcdf4d5a8251657bc3266b848c9ec331"
	fbcCompDefaultBranchName   = "main"
	fbcDockerFilePath          = "catalog.Dockerfile"
	targetPort                 = 50051
	relSvcCatalogPathInRepo    = "pipelines/managed/fbc-release/fbc-release.yaml"
)

var (
	devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
	managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)
	devFw *framework.Framework
	managedFw *framework.Framework
	snapshot *appservice.Snapshot
	releaseCR *releaseapi.Release
	releaseCRList *releaseapi.ReleaseList
	managedPipelineRun *pipeline.PipelineRun
	buildPipelineRun *pipeline.PipelineRun
	preGAPipelineRun *pipeline.PipelineRun
	hotfixPipelineRun *pipeline.PipelineRun
	stagedPipelineRun *pipeline.PipelineRun
	fbcComponent *appservice.Component
	err error

	// PaC related variables
	fbcPacBranchName string
	fbcCompBaseBranchName string
)

var _ = framework.ReleasePipelinesSuiteDescribe("FBC e2e-tests", Label("release-pipelines", "fbc-release"), func() {
	defer GinkgoRecover()

	var (
		devNamespace = devWorkspace + "-tenant"
		managedNamespace = managedWorkspace + "-tenant"

		issueId = "bz12345"
		productName = "preGA-product"
		productVersion = "v2"

		fbcApplicationName = "fbc-pipelines-app-" + util.GenerateRandomString(4)

		fbcReleasePlanName = "fbc-pipelines-rp-" + util.GenerateRandomString(4)
		fbcStagedRPName = "fbc-staged-rp-" + util.GenerateRandomString(4)
		fbcHotfixRPName = "fbc-hotfix-rp-" + util.GenerateRandomString(4)
		fbcPreGARPName = "fbc-prega-rp-" + util.GenerateRandomString(4)

		fbcReleasePlanAdmissionName = "fbc-pipelines-rpa-" + util.GenerateRandomString(4)
		fbcStagedRPAName = "fbc-staged-rpa-" + util.GenerateRandomString(4)
		fbcHotfixRPAName = "fbc-hotfix-rpa-" + util.GenerateRandomString(4)
		fbcPreGARPAName = "fbc-prega-rpa-" + util.GenerateRandomString(4)

		fbcEnterpriseContractPolicyName = "fbc-pipelines-policy-" + util.GenerateRandomString(4)
	)

	Describe("with FBC happy path", Label("fbcHappyPath"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("created application :", fbcApplicationName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlanWithRPA(fbcReleasePlanName, devNamespace, fbcApplicationName, managedNamespace, fbcReleasePlanAdmissionName, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlanWithRPA(fbcStagedRPName, devNamespace, fbcApplicationName, managedNamespace, fbcStagedRPAName, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			
			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlanWithRPA(fbcHotfixRPName, devNamespace, fbcApplicationName, managedNamespace, fbcHotfixRPAName, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlanWithRPA(fbcPreGARPName, devNamespace, fbcApplicationName, managedNamespace, fbcPreGARPAName, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			
			createFBCEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
			
			createFBCReleasePlanAdmission(fbcReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcEnterpriseContractPolicyName, relSvcCatalogPathInRepo, false, "", false, "", "", false)
			createFBCReleasePlanAdmission(fbcStagedRPAName, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcEnterpriseContractPolicyName, relSvcCatalogPathInRepo, false, "", false, "", "", true)
			createFBCReleasePlanAdmission(fbcHotfixRPAName, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcEnterpriseContractPolicyName, relSvcCatalogPathInRepo, true, issueId, false, "", "", false)
			createFBCReleasePlanAdmission(fbcPreGARPAName, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcEnterpriseContractPolicyName, relSvcCatalogPathInRepo, false, issueId, true, productName, productVersion, false)
		})

		AfterAll(func() {
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}
			// delete CRs			
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcStagedRPAName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcHotfixRPAName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcPreGARPAName, managedNamespace, false)).NotTo(HaveOccurred())
			
			deleteTestBranches()
		})

		var _ = Describe("Post-release verification", func() {
			It(fmt.Sprintf("creates component from git source %s", fbcSourceGitURL), func() {
				fbcComponent, fbcPacBranchName, fbcCompBaseBranchName = releasecommon.CreateComponentWithNewBranch(*devFw, devNamespace, fbcApplicationName, fbcCompRepoName, fbcSourceGitURL, fbcCompRevision, "4.13", fbcDockerFilePath, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), constants.DefaultFbcBuilderPipelineBundle))
				GinkgoWriter.Printf("Component %s is created", fbcComponent.GetName())
			})

			It("Creates a push snapshot for a release", func() {
				snapshot = releasecommon.CreatePushSnapshot(devWorkspace, devNamespace, fbcApplicationName, fbcCompRepoName, fbcPacBranchName, buildPipelineRun, fbcComponent)
			})

			It("verifies the release CRs are created", func() {
				assertReleasesCRCreated(devFw, managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponent.GetName())
			})
			
			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(devFw, managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponent.GetName())
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(devFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponent.GetName())
			})
		})
	})

})

func assertReleasesCRCreated(devFw, managedFw *framework.Framework, devNamespace, managedNamespace, fbcAppName string, componentName string) {

	snapshot, err = devFw.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentName, devNamespace)
	Expect(err).ToNot(HaveOccurred())
	GinkgoWriter.Println("snapshot: ", snapshot.Name)
	Eventually(func() error {
		
		releaseCRList, err = devFw.AsKubeDeveloper.ReleaseController.GetReleasesBySnapshot(snapshot.Name, devNamespace)
		if err != nil {
			return err
		}
		if len(releaseCRList.Items) < 4 {
			return fmt.Errorf("expected 4 release CRs, got %d", len(releaseCRList.Items))
		}
		for _, r := range releaseCRList.Items {
			GinkgoWriter.Printf("Release CR: %s, Namespace: %s, Snapshot: %s\n", r.Name, r.Namespace, r.Spec.Snapshot)
		}
		return nil
	}, 5*time.Minute, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for Release being created for snapshot %s/%s", devNamespace, snapshot.Name)
}

func assertReleasePipelineRunSucceeded(devFw, managedFw *framework.Framework, devNamespace, managedNamespace, fbcAppName string, componentName string) {
	Eventually(func() error {
		for _, releaseCR := range releaseCRList.Items {
			managedPipelineRun, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
			if err != nil {
				return fmt.Errorf("PipelineRun has not been created yet for release %s/%s", releaseCR.GetNamespace(), releaseCR.GetName())
			}

			for _, condition := range managedPipelineRun.Status.Conditions {
				GinkgoWriter.Printf("PipelineRun %s reason: %s\n", managedPipelineRun.Name, condition.Reason)
			}

			if !managedPipelineRun.IsDone() {
				return fmt.Errorf("PipelineRun %s has still not finished yet", managedPipelineRun.Name)
			}

			if managedPipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
				continue	
			} else {
				storeCRsIntoLog()
				prLogs, err := tekton.GetFailedPipelineRunLogs(
					managedFw.AsKubeAdmin.ReleaseController.KubeRest(), 
					managedFw.AsKubeAdmin.ReleaseController.KubeInterface(), 
					managedPipelineRun,
				)
				if err != nil {
					GinkgoWriter.Printf("failed to get PipelineRun logs for %s/%s: %+v\n", managedPipelineRun.Namespace, managedPipelineRun.Name, err)
					Expect(err).ShouldNot(HaveOccurred())
					return nil
				}
									
				Expect(prLogs).To(Equal(""), fmt.Sprintf("The failed PipelineRun %s log: %s", managedPipelineRun.Name, prLogs))
				return nil
			}
		}
		return nil
	}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the release PipelineRun to be finished for the release %s/%s", releaseCR.GetName(), releaseCR.GetNamespace()))
}

func assertReleaseCRSucceeded(devFw *framework.Framework, devNamespace, managedNamespace, fbcAppName string, componentName string) {
	devFw = releasecommon.NewFramework(devWorkspace)
	Eventually(func() error {
		releaseCRList, err := devFw.AsKubeDeveloper.ReleaseController.GetReleasesBySnapshot(snapshot.Name, devNamespace)
		if err != nil {
			return fmt.Errorf("failed to get releases by snapshot %s: %w", snapshot.Name, err)
		}
		
		if len(releaseCRList.Items) < 4 {
			return fmt.Errorf("expected %d release CRs, got %d", 4, len(releaseCRList.Items))
		}
		
		for _, r := range releaseCRList.Items {
			GinkgoWriter.Printf("Checking Release CR: %s, Namespace: %s, Snapshot: %s\n", 
				r.Name, r.Namespace, r.Spec.Snapshot)
			
			if err = releasecommon.CheckReleaseStatus(&r); err != nil {
				return fmt.Errorf("release %s/%s status check failed: %w", 
					r.Namespace, r.Name, err)
			}
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

func createFBCReleasePlanAdmission(fbcRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, fbcAppName, fbcECPName, pathInRepoValue string, hotfix bool, issueId string, preGA bool, productName, productVersion string, isStagedIndex bool) {
	var err error
	var targetIndex string

	if productName == "" {
		productName = "testProductName"
	}
	if isStagedIndex {
		targetIndex = ""
	} else {
		targetIndex = constants.TargetIndex
	}

	data, err := json.Marshal(map[string]interface{}{
		"fbc": map[string]interface{}{
			"fromIndex":             constants.FromIndex,
			"stagedIndex":           isStagedIndex,
			"targetIndex":           targetIndex,
			"publishingCredentials": "fbc-preview-publishing-credentials",
			"requestTimeoutSeconds": 1500,
			"buildTimeoutSeconds":   1500,
			"hotfix":                hotfix,
			"issueId":               issueId,
			"preGA":                 preGA,
			"productName":           productName,
			"productVersion":        productVersion,
			"allowedPackages":       []string{"example-operator"},
		},
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(fbcRPAName, managedNamespace, "", devNamespace, fbcECPName, releasecommon.ReleasePipelineServiceAccountDefault, []string{fbcAppName}, true, &tektonutils.PipelineRef{
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

func storeCRsIntoLog() {
	managedFw = releasecommon.NewFramework(managedWorkspace)
	// store managedPipelineRun and Release CR
	if err = managedFw.AsKubeDeveloper.TektonController.StorePipelineRun(managedPipelineRun.Name, managedPipelineRun); err != nil {
		GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", managedPipelineRun.GetNamespace(), managedPipelineRun.GetName(), err.Error())
	}
	if err = managedFw.AsKubeDeveloper.TektonController.StoreTaskRunsForPipelineRun(managedFw.AsKubeDeveloper.CommonController.KubeRest(), managedPipelineRun); err != nil {
		GinkgoWriter.Printf("failed to store TaskRuns for PipelineRun %s:%s: %s\n", managedPipelineRun.GetNamespace(), managedPipelineRun.GetName(), err.Error())
	}
}

func deleteTestBranches() {
	devFw = releasecommon.NewFramework(devWorkspace)
	managedFw = releasecommon.NewFramework(managedWorkspace)
	// Delete new branches created by PaC and a testing branch used as a component's base branch
	err = devFw.AsKubeDeveloper.CommonController.Github.DeleteRef(fbcCompRepoName, fbcPacBranchName)
	if err != nil {
		Expect(err.Error()).To(ContainSubstring(releasecommon.ReferenceDoesntExist))
	}
	err = devFw.AsKubeDeveloper.CommonController.Github.DeleteRef(fbcCompRepoName, fbcCompBaseBranchName)
	if err != nil {
		Expect(err.Error()).To(ContainSubstring(releasecommon.ReferenceDoesntExist))
	}
}
