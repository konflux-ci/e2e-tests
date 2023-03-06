package release

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"knative.dev/pkg/apis"

	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-1199]test-release-e2e-with-deployment", Label("release", "withDeployment", "HACBS"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	var fw *framework.Framework
	var err error

	var devNamespace = utils.GetGeneratedNamespace("release-dev")
	var managedNamespace = utils.GetGeneratedNamespace("release-managed")

	BeforeAll(func() {
		fw, err = framework.NewFramework("release-e2e-bundle")
		Expect(err).NotTo(HaveOccurred())

		kubeController := tekton.KubeController{
			Commonctrl: *fw.AsKubeAdmin.CommonController,
			Tektonctrl: *fw.AsKubeAdmin.TektonController,
		}

		_, err := fw.AsKubeAdmin.CommonController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating devNamespace: %v", err)
		GinkgoWriter.Println("Dev Namespace created: %s ", devNamespace)

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: %v", err)
		GinkgoWriter.Println("Managed Namespace created: %s ", managedNamespace)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(redhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, "pipeline")
		Expect(err).ToNot(HaveOccurred())

		publicKey, err := kubeController.GetPublicKey("signing-secrets", constants.TEKTON_CHAINS_NS)
		Expect(err).ToNot(HaveOccurred())

		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

		defaultEcPolicy := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   string(publicKey),
			Sources: []ecp.Source{
				{
					Name: "ec-policies",
					Policy: []string{
						"git::https://github.com/hacbs-contract/ec-policies.git//policy",
					},
					Data: []string{
						"git::https://github.com/hacbs-contract/ec-policies.git//data",
					},
				},
			},
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"minimal"},
				Exclude:     []string{"cve"},
			},
		}

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-deploy-strategy", managedNamespace, "deploy-release", "quay.io/hacbs-release/pipeline-deploy-release:main", releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, paramsReleaseStrategyMvp)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.GitOpsController.CreateEnvironment(releaseEnvironment, managedNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, releaseEnvironment, "", "mvp-deploy-strategy")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicy)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateHasApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateComponent(applicationNameDefault, componentName, devNamespace, gitSourceComponentUrl, "", containerImageUrl, "", "", true)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterAll(func() {

		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that a PipelineRun is created in dev namespace.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(devNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					return false
				}

				return strings.Contains(prList.Items[0].Name, componentName)
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that the PipelineRun in dev namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(devNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that in managed namespace will be created a PipelineRun.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					return false
				}

				return strings.Contains(prList.Items[0].Name, "release")
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies a PipelineRun started in managed namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("tests a Release should have been created in the dev namespace and succeeded.", func() {
			Eventually(func() bool {
				releaseCreated, err := fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				if releaseCreated == nil || err != nil {
					return false
				}

				return releaseCreated.HasStarted() && releaseCreated.IsDone() && releaseCreated.Status.Conditions[0].Status == "True"
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("tests that copying application and component works as designed.", func(ctx SpecContext) {
			workingDir, err := os.Getwd()
			if err != nil {
				GinkgoWriter.Printf(err.Error())
			}

			// Download copy-applications.sh script from release-utils repo
			args := []string{"https://raw.githubusercontent.com/hacbs-release/release-utils/main/copy-application.sh", "-o", "copy-application.sh"}
			err = utils.ExecuteCommandInASpecificDirectory("curl", args, workingDir)
			Expect(err).NotTo(HaveOccurred())

			args = []string{"775", "copy-application.sh"}
			err = utils.ExecuteCommandInASpecificDirectory("chmod", args, workingDir)
			Expect(err).NotTo(HaveOccurred())

			// Copying application in dev namespace to managed namespace
			args = []string{managedNamespace, "-a", devNamespace + "/" + applicationNameDefault}
			err = utils.ExecuteCommandInASpecificDirectory("./copy-application.sh", args, workingDir)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(snapshotCreationTimeout+namespaceCreationTimeout*2))
	})

	It("tests a Release should report the deployment was successfull.", func() {
		Eventually(func() bool {
			releaseCreated, err := fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
			if releaseCreated == nil || err != nil || len(releaseCreated.Status.Conditions) < 2 {
				return false
			}

			return releaseCreated.Status.Conditions[1].Message == "1 of 1 components deployed" && releaseCreated.Status.Conditions[1].Type == "AllComponentsDeployed"
		}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
	})
})
