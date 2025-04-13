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
	"k8s.io/apimachinery/pkg/api/errors"
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
	fbcServiceAccountName      = "release-service-account"
	fbcSourceGitURL            = "https://github.com/redhat-appstudio-qe/fbc-sample-repo-test"
	fbcCompRepoName            = "fbc-sample-repo-test"
	fbcCompRevision            = "139b6b8d9adca6bd6f0081482ecd284cbedc2681"
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

var _ = framework.ReleasePipelinesSuiteDescribe("FBC e2e-tests", Label("release-pipelines", "fbc-tests"), func() {
	defer GinkgoRecover()


	var (
		devNamespace = devWorkspace + "-tenant"
		managedNamespace = managedWorkspace + "-tenant"

		issueId = "bz12345"
		productName = "preGA-product"
		productVersion = "v2"
		
		fbcApplicationName = "fbc-pipelines-app-" + util.GenerateRandomString(4)
		fbcStagedAppName = "fbc-staged-app-" + util.GenerateRandomString(4)
		fbcHotfixAppName = "fbc-hotfix-app-" + util.GenerateRandomString(4)
		fbcPreGAAppName = "fbc-prega-app-" + util.GenerateRandomString(4)
		
		fbcReleasePlanName = "fbc-pipelines-rp-" + util.GenerateRandomString(4)
		fbcStagedRPName = "fbc-staged-rp-" + util.GenerateRandomString(4)
		fbcHotfixRPName = "fbc-hotfix-rp-" + util.GenerateRandomString(4)
		fbcPreGARPName = "fbc-prega-rp-" + util.GenerateRandomString(4)
		
		fbcReleasePlanAdmissionName = "fbc-pipelines-rpa-" + util.GenerateRandomString(4)
		fbcStagedRPAName = "fbc-staged-rpa-" + util.GenerateRandomString(4)
		fbcHotfixRPAName = "fbc-hotfix-rpa-" + util.GenerateRandomString(4)
		fbcPreGARPAName = "fbc-prega-rpa-" + util.GenerateRandomString(4)
		
		fbcEnterpriseContractPolicyName = "fbc-pipelines-policy-" + util.GenerateRandomString(4)
		fbcStagedECPolicyName = "fbc-staged-policy-" + util.GenerateRandomString(4)
		fbcHotfixECPolicyName = "fbc-hotfix-policy-" + util.GenerateRandomString(4)
		fbcPreGAECPolicyName = "fbc-prega-policy-" + util.GenerateRandomString(4)
	)

	Describe("with FBC happy path", Label("fbcHappyPath"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			managedNamespace = managedFw.UserNamespace

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("created application :", fbcApplicationName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcReleasePlanName, devNamespace, fbcApplicationName, managedNamespace, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
			Expect(sourceAuthJson).ToNot(BeEmpty())

			secret, err := devFw.AsKubeAdmin.CommonController.GetSecret(devNamespace, releasecommon.QuayTokenSecret)
			if secret == nil || errors.IsNotFound(err) {
				_, err = devFw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(releasecommon.QuayTokenSecret, devNamespace, sourceAuthJson)
				Expect(err).ToNot(HaveOccurred())
				err = devFw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, releasecommon.QuayTokenSecret, constants.DefaultPipelineServiceAccount, true)
				Expect(err).ToNot(HaveOccurred())
			}

			createFBCReleasePlanAdmission(fbcReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcEnterpriseContractPolicyName, relSvcCatalogPathInRepo, false, "", false, "", "", false)
			createFBCEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}
			// delete CRs			
		//	Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
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

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(devFw, managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponent.GetName())
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(devFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponent.GetName())
			})
		})
	})

	Describe("with FBC Staged Index", Label("fbcStagedIndex"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			managedNamespace = managedFw.UserNamespace

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcStagedAppName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Application %s is created", fbcStagedAppName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcStagedRPName, devNamespace, fbcStagedAppName, managedNamespace, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcStagedRPAName, *managedFw, devNamespace, managedNamespace, fbcStagedAppName, fbcStagedECPolicyName, relSvcCatalogPathInRepo, false, "", false, "", "", true)

			createFBCEnterpriseContractPolicy(fbcStagedECPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}
		//	Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcStagedAppName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcStagedECPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcStagedRPAName, managedNamespace, false)).NotTo(HaveOccurred())
			deleteTestBranches()
		})

		var _ = Describe("Post-release verification", func() {
			It(fmt.Sprintf("creates component from git source %s", fbcSourceGitURL), func() {
				fbcComponent, fbcPacBranchName, fbcCompBaseBranchName = releasecommon.CreateComponentWithNewBranch(*devFw, devNamespace, fbcStagedAppName, fbcCompRepoName, fbcSourceGitURL, fbcCompRevision, "4.13", fbcDockerFilePath, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), constants.DefaultFbcBuilderPipelineBundle))
				GinkgoWriter.Printf("Component %s is created", fbcComponent.GetName())
			})

			It("Creates a push snapshot for a release", func() {
				snapshot = releasecommon.CreatePushSnapshot(devWorkspace, devNamespace, fbcStagedAppName, fbcCompRepoName, fbcPacBranchName, stagedPipelineRun, fbcComponent)
			})

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(devFw, managedFw, devNamespace, managedNamespace, fbcStagedAppName, fbcComponent.GetName())
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(devFw, devNamespace, managedNamespace, fbcStagedAppName, fbcComponent.GetName())
			})
		})
	})

	Describe("with FBC hotfix process", Label("fbcHotfix"), func() {

		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcHotfixAppName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Application %s is created", fbcHotfixAppName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcHotfixRPName, devNamespace, fbcHotfixAppName, managedNamespace, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcHotfixRPAName, *managedFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcHotfixECPolicyName, relSvcCatalogPathInRepo, true, issueId, false, "", "", false)

			createFBCEnterpriseContractPolicy(fbcHotfixECPolicyName, *managedFw, devNamespace, managedNamespace)
		})

		AfterAll(func() {
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}
		//	Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcHotfixAppName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcHotfixECPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcHotfixRPAName, managedNamespace, false)).NotTo(HaveOccurred())
			deleteTestBranches()
		})

		var _ = Describe("FBC hotfix post-release verification", func() {

			It(fmt.Sprintf("creates component from git source %s", fbcSourceGitURL), func() {
				fbcComponent, fbcPacBranchName, fbcCompBaseBranchName = releasecommon.CreateComponentWithNewBranch(*devFw, devNamespace, fbcHotfixAppName, fbcCompRepoName, fbcSourceGitURL, fbcCompRevision, "4.13", fbcDockerFilePath, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), constants.DefaultFbcBuilderPipelineBundle))
				GinkgoWriter.Printf("Component %s is created", fbcComponent.GetName())
			})

			It("Creates a push snapshot for a release", func() {
				snapshot = releasecommon.CreatePushSnapshot(devWorkspace, devNamespace, fbcHotfixAppName, fbcCompRepoName, fbcPacBranchName, hotfixPipelineRun, fbcComponent)
			})

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(devFw, managedFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcComponent.GetName())
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(devFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcComponent.GetName())
			})
		})
	})

	Describe("with FBC pre-GA process", Label("fbcPreGA"), func() {

		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcPreGAAppName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Application %s is created", fbcPreGAAppName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcPreGARPName, devNamespace, fbcPreGAAppName, managedNamespace, "true", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCEnterpriseContractPolicy(fbcPreGAECPolicyName, *managedFw, devNamespace, managedNamespace)
			createFBCReleasePlanAdmission(fbcPreGARPAName, *managedFw, devNamespace, managedNamespace, fbcPreGAAppName, fbcPreGAECPolicyName, relSvcCatalogPathInRepo, false, issueId, true, productName, productVersion, false)
		})

		AfterAll(func() {
			if err = devFw.AsKubeDeveloper.ReleaseController.StoreRelease(releaseCR); err != nil {
				GinkgoWriter.Printf("failed to store Release %s:%s: %s\n", releaseCR.GetNamespace(), releaseCR.GetName(), err.Error())
			}
		//	Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcPreGAAppName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcPreGAECPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcPreGARPAName, managedNamespace, false)).NotTo(HaveOccurred())
			deleteTestBranches()
		})

		var _ = Describe("FBC pre-GA post-release verification", func() {
			It(fmt.Sprintf("creates component from git source %s", fbcSourceGitURL), func() {
				fbcComponent, fbcPacBranchName, fbcCompBaseBranchName = releasecommon.CreateComponentWithNewBranch(*devFw, devNamespace, fbcPreGAAppName, fbcCompRepoName, fbcSourceGitURL, fbcCompRevision, "4.13", fbcDockerFilePath, utils.MergeMaps(utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo), constants.DefaultFbcBuilderPipelineBundle))
				GinkgoWriter.Printf("Component %s is created", fbcComponent.GetName())
			})

			It("Creates a push snapshot for a release", func() {
				snapshot = releasecommon.CreatePushSnapshot(devWorkspace, devNamespace, fbcPreGAAppName, fbcCompRepoName, fbcPacBranchName, preGAPipelineRun, fbcComponent)
			})

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(devFw, managedFw, devNamespace, managedNamespace, fbcPreGAAppName, fbcComponent.GetName())
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(devFw, devNamespace, managedNamespace, fbcPreGAAppName, fbcComponent.GetName())
			})
		})
	})
})

func assertReleasePipelineRunSucceeded(devFw, managedFw *framework.Framework, devNamespace, managedNamespace, fbcAppName string, componentName string) {

	snapshot, err = devFw.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentName, devNamespace)
	Expect(err).ToNot(HaveOccurred())
	GinkgoWriter.Println("snapshot: ", snapshot.Name)
	Eventually(func() error {
		releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
		if err != nil {
			return err
		}
		GinkgoWriter.Println("Release CR: ", releaseCR.Name)
		return nil
	}, 5*time.Minute, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for Release being created")

	Eventually(func() error {
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
			return nil
		} else {
			storeCRsIntoLog()
			prLogs := ""
			if prLogs, err = tekton.GetFailedPipelineRunLogs(managedFw.AsKubeAdmin.ReleaseController.KubeRest(), managedFw.AsKubeAdmin.ReleaseController.KubeInterface(), managedPipelineRun); err != nil {
				GinkgoWriter.Printf("failed to get PLR logs: %+v", err)
				Expect(err).ShouldNot(HaveOccurred())
				return nil
			}
			GinkgoWriter.Printf("logs: %s", prLogs)
			Expect(prLogs).To(Equal(""), fmt.Sprintf("PipelineRun %s failed", managedPipelineRun.Name))
			return nil
		}
	}, releasecommon.BuildPipelineRunCompletionTimeout, releasecommon.DefaultInterval).Should(Succeed(), fmt.Sprintf("timed out when waiting for the release PipelineRun to be finished for the release %s/%s", releaseCR.GetName(), releaseCR.GetNamespace()))
}

func assertReleaseCRSucceeded(devFw *framework.Framework, devNamespace, managedNamespace, fbcAppName string, componentName string) {
	devFw = releasecommon.NewFramework(devWorkspace)
	Eventually(func() error {
		releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
		if err != nil {
			return err
		}
		err = releasecommon.CheckReleaseStatus(releaseCR)
		return err
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
