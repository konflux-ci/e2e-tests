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
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = framework.ReleasePipelinesSuiteDescribe("[RHTAPREL-373]fbc happy path e2e-test.", Label("release-pipelines", "fbcHappyPath"), func() {
	defer GinkgoRecover()

	const (
		fbcApplicationName              = "fbc-pipelines-aplication"
		fbcComponentName                = "fbc-pipelines-component"
		fbcReleasePlanName              = "fbc-pipelines-releaseplan"
		fbcReleasePlanAdmissionName     = "fbc-pipelines-releaseplanadmission"
		fbcEnterpriseContractPolicyName = "fbc-pipelines-policy"
		fbcServiceAccountName           = "release-service-account"
		fbcSourceGitUrl                 = "https://github.com/redhat-appstudio-qe/fbc-sample-repo"
		targetPort                      = 50051
		relSvcCatalogURL                = "https://github.com/redhat-appstudio/release-service-catalog"
		relSvcCatalogRevision           = "main"
		relSvcCatalogPathInRepo         = "pipelines/fbc-release/fbc-release.yaml"
		ecPolicyLibPath                 = "github.com/enterprise-contract/ec-policies//policy/lib"
		ecPolicyReleasePath             = "github.com/enterprise-contract/ec-policies//policy/release"
		ecPolicyDataPath                = "github.com/enterprise-contract/ec-policies//example/data"
	)

	var devWorkspace = os.Getenv(constants.RELEASE_DEV_WORKSPACE_ENV)
	var managedWorkspace = os.Getenv(constants.RELEASE_MANAGED_WORKSPACE_ENV)
	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"
	var releasePipelineRunCompletionTimeout = 20 * time.Minute
	var releaseCreationTimeout = 5 * time.Minute
	var defaultInterval = 100 * time.Millisecond

	var err error
	var devFw *framework.Framework
	var managedFw *framework.Framework
	var component *appservice.Component
	var releaseCR *releaseApi.Release
	var buildPr *v1beta1.PipelineRun
	var releasePr *v1beta1.PipelineRun
	var snapshot *appservice.Snapshot

	stageOptions := utils.Options{
		ToolchainApiUrl: os.Getenv(constants.TOOLCHAIN_API_URL_ENV),
		KeycloakUrl:     os.Getenv(constants.KEYLOAK_URL_ENV),
		OfflineToken:    os.Getenv(constants.OFFLINE_TOKEN_ENV),
	}

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

		_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcApplicationName, devFw.UserNamespace)
		Expect(err).NotTo(HaveOccurred())

		componentObj := appservice.ComponentSpec{
			ComponentName: fbcComponentName,
			Application:   fbcApplicationName,
			Source: appservice.ComponentSource{
				ComponentSourceUnion: appservice.ComponentSourceUnion{
					GitSource: &appservice.GitSource{
						URL: fbcSourceGitUrl,
					},
				},
			},
			TargetPort: targetPort,
		}
		component, err = devFw.AsKubeDeveloper.HasController.CreateComponent(componentObj, devFw.UserNamespace, "", "", fbcApplicationName, false, map[string]string{})
		GinkgoWriter.Println("component : ", component.Name)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcReleasePlanName, devNamespace, fbcApplicationName, managedNamespace, "true")
		Expect(err).NotTo(HaveOccurred())

		data, err := json.Marshal(map[string]interface{}{
			"fbc": map[string]interface{}{
				"fromIndex":            constants.FromIndex,
				"targetIndex":          constants.TargetIndex,
				"binaryImage":          constants.BinaryImage,
				"requestUpdateTimeout": "420",
				"buildTimeoutSeconds":  "480",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(fbcReleasePlanAdmissionName, managedNamespace, "", devNamespace, fbcEnterpriseContractPolicyName, fbcServiceAccountName, []string{fbcApplicationName}, true, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: relSvcCatalogURL},
				{Name: "revision", Value: relSvcCatalogRevision},
				{Name: "pathInRepo", Value: relSvcCatalogPathInRepo},
			},
		}, &runtime.RawExtension{
			Raw: data,
		})

		Expect(err).NotTo(HaveOccurred())

		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   "k8s://openshift-pipelines/public-key",
			Sources: []ecp.Source{{
				Name:   "Default",
				Policy: []string{ecPolicyLibPath, ecPolicyReleasePath},
				Data:   []string{ecPolicyDataPath},
			}},
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"minimal"},
				Exclude:     []string{"cve", "step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
				Include:     []string{"@slsa3"},
			},
		}

		_, err = managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred())

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
			Expect(devFw.AsKubeDeveloper.HasController.WaitForComponentPipelineToBeFinished(component, "", 2, devFw.AsKubeDeveloper.TektonController)).To(Succeed())
			buildPr, err = devFw.AsKubeDeveloper.HasController.GetComponentPipelineRun(component.Name, fbcApplicationName, devNamespace, "")
			Expect(err).ShouldNot(HaveOccurred())

		})

		It("verifies the fbc release pipelinerun is running and succeeds", func() {
			snapshot, err = devFw.AsKubeDeveloper.IntegrationController.GetSnapshot("", buildPr.Name, component.Name, devNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() error {
				releasePr, err = managedFw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedFw.UserNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				if !releasePr.IsDone() {
					return fmt.Errorf("release pipelinerun %s in namespace %s did not finish yet", releasePr.Name, releasePr.Namespace)
				}
				GinkgoWriter.Println("Release PR: ", releasePr.Name)
				Expect(tekton.HasPipelineRunSucceeded(releasePr)).To(BeTrue(), fmt.Sprintf("release pipelinerun %s/%s did not succeed", releasePr.GetNamespace(), releasePr.GetName()))
				return nil
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(Succeed(), "timed out when waiting for release pipelinerun to succeed")
		})

		It("verifies release CR completed and set succeeded.", func() {
			Eventually(func() error {
				releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
				if err != nil {
					return err
				}
				GinkgoWriter.Println("Release CR: ", releaseCR.Name)
				if !releaseCR.IsReleased() {
					return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
				}
				return nil
			}, releaseCreationTimeout, defaultInterval).Should(Succeed())
		})

	})
})
