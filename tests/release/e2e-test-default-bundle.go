package release

import (
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"knative.dev/pkg/apis"

	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klog "k8s.io/klog/v2"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-738]test-release-service-happy-path", Label("release", "defaultBundle", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var defaultEcPolicy = ecp.EnterpriseContractPolicySpec{}
	var devNamespace = uuid.New().String()
	var managedNamespace = uuid.New().String()

	var cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: buildPipelineBundleDefaultName,
		},
		Data: map[string]string{
			"default_build_bundle": buildPipelineBundleDefault,
		},
	}

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework("release-e2e-bundle")
		Expect(err).NotTo(HaveOccurred())
		kubeController := tekton.KubeController{
			Commonctrl: *fw.AsKubeAdmin.CommonController,
			Tektonctrl: *fw.AsKubeAdmin.TektonController,
		}

		_, err := fw.AsKubeAdmin.CommonController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating devNamespace: %v", err)
		klog.Info("Dev Namespace created: ", devNamespace)

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: %v", err)
		klog.Info("Managed Namespace created: ", managedNamespace)

		klog.Infof("Wait until the 'pipeline' SA is created in %s namespace \n", managedNamespace)
		Eventually(func() bool {
			sa, err := fw.AsKubeAdmin.CommonController.GetServiceAccount("pipeline", managedNamespace)
			return sa != nil && err == nil
		}, pipelineServiceAccountCreationTimeout, defaultInterval).Should(BeTrue(), "timed out when waiting for the \"pipeline\" SA to be created")

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		destinationAuthJson := utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_DESTINATION, "")
		Expect(destinationAuthJson).ToNot(BeEmpty())

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateRegistryJsonSecret(redhatAppstudioUserSecret, managedNamespace, destinationAuthJson, destinationKeyName)
		Expect(err).ToNot(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, "pipeline")
		Expect(err).ToNot(HaveOccurred())

		klog.Info("Dev Namespace:", devNamespace)
		klog.Info("Managed Namespace:", managedNamespace)

		// Copy the public key from tekton-chains/signing-secrets to a new
		// secret that contains just the public key to ensure that access
		// to password and private key are not needed.
		publicKey, err := kubeController.GetPublicKey("signing-secrets", constants.TEKTON_CHAINS_NS)
		Expect(err).ToNot(HaveOccurred())
		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

		defaultEcPolicy = ecp.EnterpriseContractPolicySpec{
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
			Exceptions: &ecp.EnterpriseContractPolicyExceptions{
				NonBlocking: []string{"tasks", "attestation_task_bundle", "java", "test", "not_useful"},
			},
		}

		_, err = fw.AsKubeAdmin.CommonController.CreateConfigMap(cm, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy(releaseStrategyDefaultName, managedNamespace, releasePipelineNameDefault, releasePipelineBundleDefault, releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, paramsReleaseStrategy)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, "", "", releaseStrategyDefaultName)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicy)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRole(roleName, managedNamespace, roleRules)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-relase-service-account-binding", managedNamespace, "ServiceAccount", roleName, "Role", "role-m6-service-account", "rbac.authorization.k8s.io")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateHasApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateComponent(applicationNameDefault, componentName, devNamespace, gitSourceComponentUrl, "", containerImageUrl, "", "", false)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that in dev namespace will be created a PipelineRun.", func() {
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
	})
})
