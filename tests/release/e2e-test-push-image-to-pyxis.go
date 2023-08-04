package release

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/devfile/library/v2/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gopkg.in/yaml.v2"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/release"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-1571]test-release-e2e-push-image-to-pyxis", Label("release", "pushPyxis", "HACBS"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	var fw *framework.Framework
	var err error
	var kubeController tekton.KubeController
	var devNamespace, managedNamespace, compName, additionalCompName string
	var imageIDs []string
	var pyxisKeyDecoded, pyxisCertDecoded []byte
	var releasePR1, releasePR2 *v1beta1.PipelineRun
	scGitRevision := fmt.Sprintf("test-pyxis-%s", util.GenerateRandomString(4))

	var component1, component2 *appservice.Component
	var snapshot1, snapshot2 *appservice.Snapshot
	var releaseCR1, releaseCR2 *releaseApi.Release

	var componentDetected, additionalComponentDetected appservice.ComponentDetectionDescription

	BeforeAll(func() {
		fw, err = framework.NewFramework("release-e2e-pyxis")
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		kubeController = tekton.KubeController{
			Commonctrl: *fw.AsKubeAdmin.CommonController,
			Tektonctrl: *fw.AsKubeAdmin.TektonController,
		}

		managedNamespace = utils.GetGeneratedNamespace("release-e2e-pyxis-managed")

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace")

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
		Expect(keyPyxisStage).ToNot(BeEmpty())

		certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
		Expect(certPyxisStage).ToNot(BeEmpty())

		// Create secret for the release registry repo "hacbs-release-tests".
		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(redhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		// Linking the build secret to the pipeline service account in dev namespace.
		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, serviceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		publicKey, err := kubeController.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())

		// Creating k8s secret to access Pyxis stage based on base64 decoded of key and cert
		pyxisKeyDecoded, err = base64.StdEncoding.DecodeString(string(keyPyxisStage))
		Expect(err).ToNot(HaveOccurred())
		pyxisCertDecoded, err = base64.StdEncoding.DecodeString(string(certPyxisStage))
		Expect(err).ToNot(HaveOccurred())

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pyxis",
				Namespace: managedNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"cert": pyxisCertDecoded,
				"key":  pyxisKeyDecoded,
			},
		}

		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
		Expect(err).ToNot(HaveOccurred())

		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

		defaultEcPolicy, err := kubeController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred())

		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   string(publicKey),
			Sources:     defaultEcPolicy.Spec.Sources,
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"minimal", "slsa2"},
				Exclude:     []string{"cve"},
			},
		}

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret, nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(devNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, redhatAppstudioUserSecret, releaseStrategyServiceAccountDefault, true)
		Expect(err).ToNot(HaveOccurred())

		// using cdq since git ref is not known
		compName = componentName
		cdq, err := fw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(compName, devNamespace, gitSourceComponentUrl, "", "", "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "Expected length of the detected Components was not 1")

		for _, compDetected := range cdq.Status.ComponentDetected {
			compName = compDetected.ComponentStub.ComponentName
			componentDetected = compDetected
		}

		// using cdq since git ref is not known
		additionalCompName = additionalComponentName
		cdq, err = fw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(additionalCompName, devNamespace, additionalGitSourceComponentUrl, "", "", "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(cdq.Status.ComponentDetected).To(HaveLen(1), "Expected length of the detected Components was not 1")

		for _, compDetected := range cdq.Status.ComponentDetected {
			additionalCompName = compDetected.ComponentStub.ComponentName
			additionalComponentDetected = compDetected
		}

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		components := []release.Component{{Name: compName, Repository: releasedImagePushRepo}, {Name: additionalCompName, Repository: additionalReleasedImagePushRepo}}
		sc := fw.AsKubeAdmin.ReleaseController.GenerateReleaseStrategyConfig(components)
		scYaml, err := yaml.Marshal(sc)
		Expect(err).ShouldNot(HaveOccurred())

		scPath := "release-push-to-pyxis.yaml"
		Expect(fw.AsKubeAdmin.CommonController.Github.CreateRef(constants.StrategyConfigsRepo, constants.StrategyConfigsDefaultBranch, constants.StrategyConfigsRevision, scGitRevision)).To(Succeed())
		_, err = fw.AsKubeAdmin.CommonController.Github.CreateFile(constants.StrategyConfigsRepo, scPath, string(scYaml), scGitRevision)
		Expect(err).ShouldNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-push-to-external-registry-strategy", managedNamespace, "push-to-external-registry", "quay.io/hacbs-release/pipeline-push-to-external-registry:0.12", releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, []releaseApi.Params{
			{Name: "extraConfigGitUrl", Value: fmt.Sprintf("https://github.com/%s/strategy-configs.git", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))},
			{Name: "extraConfigPath", Value: scPath},
			{Name: "extraConfigGitRevision", Value: scGitRevision},
			{Name: "pyxisServerType", Value: "stage"},
			{Name: "pyxisSecret", Value: "pyxis"},
			{Name: "tag", Value: "latest"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, "", "", "mvp-push-to-external-registry-strategy")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", releaseStrategyServiceAccountDefault, managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		err = fw.AsKubeAdmin.CommonController.Github.DeleteRef(constants.StrategyConfigsRepo, scGitRevision)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("Reference does not exist"))
		}
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that Component 1 can be created and build PipelineRun is created for it in dev namespace and succeeds", func() {
			component1, err = fw.AsKubeAdmin.HasController.CreateComponent(componentDetected.ComponentStub, devNamespace, "", "", applicationNameDefault, true, map[string]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component1, "", 2)).To(Succeed())
		})

		It("verifies that Component 2 can be created and build PipelineRun is created for it in dev namespace and succeeds", func() {
			component2, err = fw.AsKubeAdmin.HasController.CreateComponent(additionalComponentDetected.ComponentStub, devNamespace, "", "", applicationNameDefault, true, map[string]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component2, "", 2)).To(Succeed())
		})

		It("tests that Snapshot is created for each Component", func() {
			Eventually(func() error {
				snapshot1, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", "", component1.GetName(), devNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Snapshot for component %s/%s: %v\n", component1.GetNamespace(), component1.GetName(), err)
					return err
				}
				snapshot2, err = fw.AsKubeAdmin.IntegrationController.GetSnapshot("", "", component2.GetName(), devNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Snapshot for component %s/%s: %v\n", component2.GetNamespace(), component2.GetName(), err)
					return err
				}
				return nil
			}, snapshotCreationTimeout, defaultInterval).Should(Succeed(), "timed out waiting for Snapshots to be created in %s namespace", devNamespace)
		})

		It("tests that associated Release CR is created for each Component's Snapshot", func() {
			Eventually(func() error {
				releaseCR1, err = fw.AsKubeAdmin.ReleaseController.GetRelease("", snapshot1.GetName(), devNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Release CR for snapshot %s/%s: %v\n", snapshot1.GetNamespace(), component1.GetName(), err)
					return err
				}
				releaseCR2, err = fw.AsKubeAdmin.ReleaseController.GetRelease("", snapshot2.GetName(), devNamespace)
				if err != nil {
					GinkgoWriter.Printf("cannot get the Release CR for snapshot %s/%s: %v\n", snapshot2.GetNamespace(), component1.GetName(), err)
					return err
				}
				return nil
			}, releaseCreationTimeout, defaultInterval).Should(Succeed(), "timed out waiting for Release CRs to be created in %s namespace", devNamespace)
		})

		It("verifies that Release PipelineRun is triggered for each Release CR", func() {
			Eventually(func() error {
				releasePR1, err = fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR1.GetName(), releaseCR1.GetNamespace())
				if err != nil {
					GinkgoWriter.Printf("release pipelineRun for Release %s/%s not created yet: %+v\n", releaseCR1.GetNamespace(), releaseCR1.GetName(), err)
					return err
				}
				releasePR2, err = fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR2.GetName(), releaseCR2.GetNamespace())
				if err != nil {
					GinkgoWriter.Printf("release pipelineRun for Release %s/%s not created yet: %+v\n", releaseCR2.GetNamespace(), releaseCR2.GetName(), err)
					return err
				}
				var errMsg string
				for _, pr := range []*v1beta1.PipelineRun{releasePR1, releasePR2} {
					Expect(utils.HasPipelineRunFailed(pr)).ToNot(BeTrue(), fmt.Sprintf("Release PipelineRun %s/%s failed", pr.GetNamespace(), pr.GetName()))
					if !pr.HasStarted() {
						errMsg += fmt.Sprintf("Release PipelineRun %s/%s did not started yet\n", pr.GetNamespace(), pr.GetName())
					}
				}
				if len(errMsg) > 1 {
					return fmt.Errorf(errMsg)
				}
				return nil
			}, releasePipelineRunCreationTimeout, constants.PipelineRunPollingInterval).Should(Succeed(), fmt.Sprintf("timed out waiting for a PipelineRun to start for each Release CR in %s namespace", managedNamespace))
		})

		It("verifies a release PipelineRun for each component succeeded in managed namespace", func() {
			Eventually(func() error {
				var errMsg string
				for _, pr := range []*v1beta1.PipelineRun{releasePR1, releasePR2} {
					pr, err = fw.AsKubeAdmin.TektonController.GetPipelineRun(pr.GetName(), pr.GetNamespace())
					Expect(err).ShouldNot(HaveOccurred())
					Expect(utils.HasPipelineRunFailed(pr)).ToNot(BeTrue(), fmt.Sprintf("Release PipelineRun %s/%s failed", pr.GetNamespace(), pr.GetName()))
					if pr.IsDone() {
						Expect(utils.HasPipelineRunSucceeded(pr)).To(BeTrue(), fmt.Sprintf("Release PipelineRun %s/%s did not succceed", pr.GetNamespace(), pr.GetName()))
					} else {
						errMsg += fmt.Sprintf("Release PipelineRun %s/%s did not finish yet\n", pr.GetNamespace(), pr.GetName())
					}
				}
				if len(errMsg) > 1 {
					return fmt.Errorf(errMsg)
				}
				return nil
			}, releasePipelineRunCompletionTimeout, constants.PipelineRunPollingInterval).Should(Succeed())
		})

		It("validate the result of task create-pyxis-image contains image ids", func() {
			Eventually(func() []string {
				re := regexp.MustCompile("[a-fA-F0-9]{24}")

				releasePR1, err = fw.AsKubeAdmin.TektonController.GetPipelineRun(releasePR1.GetName(), releasePR1.GetNamespace())
				Expect(err).NotTo(HaveOccurred())
				releasePR2, err = fw.AsKubeAdmin.TektonController.GetPipelineRun(releasePR2.GetName(), releasePR2.GetNamespace())
				Expect(err).NotTo(HaveOccurred())

				trReleasePr, err := kubeController.GetTaskRunStatus(fw.AsKubeAdmin.CommonController.KubeRest(), releasePR1, "create-pyxis-image")
				Expect(err).NotTo(HaveOccurred())

				trAdditionalReleasePr, err := kubeController.GetTaskRunStatus(fw.AsKubeAdmin.CommonController.KubeRest(), releasePR2, "create-pyxis-image")
				Expect(err).NotTo(HaveOccurred())

				trReleaseImageIDs := re.FindAllString(trReleasePr.Status.TaskRunResults[0].Value.StringVal, -1)
				trAdditionalReleaseIDs := re.FindAllString(trAdditionalReleasePr.Status.TaskRunResults[0].Value.StringVal, -1)

				Expect(trReleaseImageIDs).ToNot(BeEmpty(), fmt.Sprintf("Invalid ImageID in results of task create-pyxis-image. taskrun results: %+v", trReleasePr.Status.TaskRunResults[0]))
				Expect(trAdditionalReleaseIDs).ToNot(BeEmpty(), fmt.Sprintf("Invalid ImageID in results of task create-pyxis-image. taskrun results: %+v", trAdditionalReleasePr.Status.TaskRunResults[0]))

				Expect(trReleaseImageIDs).ToNot(HaveLen(len(trAdditionalReleaseIDs)), "the number of image IDs should not be the same in both taskrun results. (%+v vs. %+v)", trReleasePr.Status.TaskRunResults[0], trAdditionalReleasePr.Status.TaskRunResults[0])

				if len(trReleaseImageIDs) > len(trAdditionalReleaseIDs) {
					imageIDs = trReleaseImageIDs
				} else {
					imageIDs = trAdditionalReleaseIDs
				}

				return imageIDs
			}, avgControllerQueryTimeout, defaultInterval).Should(HaveLen(2))
		})

		It("tests that associated Release CR has completed for each Component's Snapshot", func() {
			Eventually(func() error {
				var errMsg string
				for _, cr := range []*releaseApi.Release{releaseCR1, releaseCR2} {
					cr, err = fw.AsKubeAdmin.ReleaseController.GetRelease("", cr.Spec.Snapshot, devNamespace)
					Expect(err).ShouldNot(HaveOccurred())
					if !cr.IsReleased() {
						errMsg += fmt.Sprintf("release %s/%s is not marked as finished yet", cr.GetNamespace(), cr.GetName())
					}
				}
				if len(errMsg) > 1 {
					return fmt.Errorf(errMsg)
				}
				return nil
			}, releaseCreationTimeout, defaultInterval).Should(Succeed(), "timed out waiting for Release CRs to be created in %s namespace", devNamespace)
		})

		It("validates that imageIds from task create-pyxis-image exist in Pyxis.", func() {
			for _, imageID := range imageIDs {
				Eventually(func() error {
					body, err := fw.AsKubeAdmin.ReleaseController.GetPyxisImageByImageID(pyxisStageImagesApiEndpoint, imageID,
						[]byte(pyxisCertDecoded), []byte(pyxisKeyDecoded))
					Expect(err).NotTo(HaveOccurred(), "failed to get response body")

					sbomImage := release.Image{}
					Expect(json.Unmarshal(body, &sbomImage)).To(Succeed(), "failed to unmarshal body content: %s", string(body))

					if sbomImage.ContentManifest.ID == "" {
						return fmt.Errorf("ContentManifest field is empty for sbom image: %+v", sbomImage)
					}

					return nil
				}, releaseCreationTimeout, defaultInterval).Should(Succeed())
			}
		})
	})
})
