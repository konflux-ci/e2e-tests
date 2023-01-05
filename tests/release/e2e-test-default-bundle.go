package release

import (
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"

	//appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"

	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klog "k8s.io/klog/v2"
	"knative.dev/pkg/apis"
)

var managednamespaceSecret = []corev1.ObjectReference{
	{Name: redhatAppstudioUserSecret},
}

var roleRules = map[string][]string{
	"apiGroupsList": {""},
	"roleResources": {"secrets"},
	"roleVerbs":     {"get", "list", "watch"},
}

var paramsReleaseStrategyDefault = []appstudiov1alpha1.Params{
	{Name: "extraConfigGitUrl", Value: "https://github.com/scoheb/strategy-configs.git"},
	{Name: "extraConfigPath", Value: "m6.yaml"},
	{Name: "extraConfigRevision", Value: "main"},
}

var _ = framework.ReleaseSuiteDescribe("test-release-service-happy-path", Label("release", "defaultBundle"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

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
		kubeController := tekton.KubeController{
			Commonctrl: *framework.CommonController,
			Tektonctrl: *framework.TektonController,
		}

		demo, err := framework.CommonController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", demo.Name, err)
		klog.Info("Dev Namespace created: ", demo.Name)

		namespace, err := framework.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", namespace.Name, err)
		klog.Info("Managed Namespace created: ", namespace.Name)

		klog.Infof("Wait until the 'pipeline' SA is created in %s namespace \n", managedNamespace)
		Eventually(func() bool {
			sa, err := framework.CommonController.GetServiceAccount("pipeline", managedNamespace)
			return sa != nil && err == nil
		}, 1*time.Minute, defaultInterval).Should(BeTrue(), "timed out when waiting for the \"pipeline\" SA to be created")

		sourceAuthJson := utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_SOURCE, "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		destinationAuthJson := utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_DESTINATION, "")
		Expect(destinationAuthJson).ToNot(BeEmpty())

		_, err = framework.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())
		_, err = framework.ReleaseController.CreateRegistryJsonSecret(redhatAppstudioUserSecret, managedNamespace, destinationAuthJson, destinationKeyName)
		Expect(err).ToNot(HaveOccurred())

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

	})

	AfterAll(func() {
		// Delete the dev and managed namespaces with all the resources created in them
		Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
		Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	})

	var _ = Describe("Creation of the 'Happy path for defult pipeline bundle' resources", func() {

		It("Create buildPipelineDeafultBundle", func() {
			_, err := framework.CommonController.CreateConfigMap(cm, devNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ReleasePlan in dev namespace", func() {
			_, err := framework.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Strategy", func() {
			_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyDefaultName, managedNamespace, releasePipelineNameDefault, releasePipelineBundleDefault, releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, paramsReleaseStrategyDefault)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ReleasePlanAdmission in managed namespace", func() {
			_, err := framework.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, "", "", releaseStrategyDefaultName) //releaseEnvironment
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates EnterpriseContractPolicy in managed namespace.", func(ctx SpecContext) {
			_, err := framework.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicy)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(EnterpriseContractPolicyTimeout))

		It("Create PVC", func() {
			_, err := framework.TektonController.CreatePVCAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Service Account", func() {
			_, err := framework.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Role", func() {
			_, err := framework.CommonController.CreateRole(roleName, managedNamespace, roleRules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create RoleBinding", func() {
			_, err := framework.CommonController.CreateRoleBinding(roleBindingName, managedNamespace, subjectKind, roleName, roleRefKind, roleRefName, roleRefApiGroup)
			Expect(err).NotTo(HaveOccurred())
		})

		It("CreateApplication", func() {
			_, err := framework.HasController.CreateHasApplication(applicationNameDefault, devNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("CreateComponent", func() {
			_, err := framework.HasController.CreateComponent(applicationNameDefault, componentName, devNamespace, gitSourceComponentUrl, "", containerImageUrl, "", "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that in dev namespace will be created a PipelineRun.", func() {
			Eventually(func() bool {
				prList, err := framework.TektonController.ListAllPipelineRuns(devNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					klog.Error(err)
					return false
				}
				return strings.Contains(prList.Items[0].Name, componentName)
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that the PipelineRun in dev namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := framework.TektonController.ListAllPipelineRuns(devNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					klog.Error(err)
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		// TODO should be removed once the it's done by service
		// Adds manually the label of PaC to created snapshot in dev namespace
		It("gets snapshot created in dev namepsace and add labebl to the Snapshot.", func() {
			snapshotCreatedInDev, err := framework.ReleaseController.GetSnapshotInNamespace(devNamespace, componentName)
			Expect(err).NotTo(HaveOccurred())
			_, err = framework.ReleaseController.AddLabelToSnapshot(snapshotCreatedInDev)
			Expect(err).NotTo(HaveOccurred())
		})

		It("verifies that in managed namespace will be created a PipelineRun.", func() {
			Eventually(func() bool {
				prList, err := framework.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					klog.Error(err)
					return false
				}
				klog.Infof("pipelineList in managed namespace are : ", prList.Items[0].Name)
				return strings.Contains(prList.Items[0].Name, componentName)
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies a PipelineRun started in managed namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := framework.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					klog.Error(err)
					return false
				}
				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("tests a Release should have been created in the dev namespace and succeeded", func() {
			Eventually(func() bool {
				releaseCreated, err := framework.ReleaseController.GetReleaseInNamespace(devNamespace)
				if releaseCreated == nil || err != nil {
					klog.Error(err)
					return false
				}
				return releaseCreated.HasStarted() && releaseCreated.IsDone() && releaseCreated.Status.Conditions[0].Status == "True"
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})
	})
})
