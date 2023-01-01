package release

import (
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"

	//gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"

	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	corev1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"
	// "knative.dev/pkg/apis"
)

// var myEnvironment = []gitopsv1alpha1.EnvVarPair{
// 	{Name: releaseEnvironment, Value: releaseEnvironment},
// }

var managednamespaceSecret = []corev1.ObjectReference{
	{Name: hacbsReleaseTestsTokenSecret},
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

	//var myEnvironment = []gitopsv1alpha1.EnvVarPair{}

	BeforeAll(func() {
		kubeController := tekton.KubeController{
			Commonctrl: *framework.CommonController,
			Tektonctrl: *framework.TektonController,
		}
		// Create the dev namespace
		demo, err := framework.CommonController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", demo.Name, err)
		klog.Info("Dev Namespace created: ", demo.Name)

		// Create the managed namespace
		namespace, err := framework.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", namespace.Name, err)
		klog.Info("Managed Namespace created: ", namespace.Name)

		// Wait until the "pipeline" SA is created and ready with secrets by the openshift-pipelines operator
		klog.Infof("Wait until the 'pipeline' SA is created in %s namespace \n", managedNamespace)
		Eventually(func() bool {
			sa, err := framework.CommonController.GetServiceAccount("pipeline", managedNamespace)
			return sa != nil && err == nil
		}, 1*time.Minute, defaultInterval).Should(BeTrue(), "timed out when waiting for the \"pipeline\" SA to be created")

		// Kasem TODO findout why it's

		//utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_SOURCE, ""))
		// for hacbs-release-tests  quay
		sourceAuthJson := "ewogICJhdXRocyI6IHsKICAgICJxdWF5LmlvIjogewogICAgICAiYXV0aCI6ICJhR0ZqWW5NdGNtVnNaV0Z6WlMxMFpYTjBjeXR0TlY5eWIySnZkRjloWTJOdmRXNTBPakpXTWpaQ1NrdExVMEZSTWpKWVZEQktOVGM1UmxJNVZ6azJOVlE1UlRkYVZWSlpNRVZNU3psVk1FdEJUVVE1U0ZGSk1sQk9WVUZNU2trMlRsVldNVGc9IiwKICAgICAgImVtYWlsIjogIiIKICAgIH0KICB9Cn0="

		//utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_DESTINATION, "")
		// for release-e2e quay
		destinationAuthJson := "ewogICJhdXRocyI6IHsKICAgICJxdWF5LmlvIjogewogICAgICAiYXV0aCI6ICJjbVZzWldGelpTMWxNbVVyY21Wc1pXRnpaVjlsTW1VNlVUTkNRa1JOTWtaTk4waFNUVWxHVWxsR09GaFNOVWROTlZkUlFqWklXVXRKTjBwWVJrbzJTMXBQVUV4WFJVOVVWamxUUVVOVk9WRkZXRGxTU2pCRlJ3PT0iLAogICAgICAiZW1haWwiOiAiIgogICAgfQogIH0KfQ=="

		Expect(sourceAuthJson).ToNot(BeEmpty())
		Expect(destinationAuthJson).ToNot(BeEmpty())

		// _, err = framework.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		// _, err = framework.ReleaseController.CreateRegistryJsonSecret(redhatAppstudioUserSecret, devNamespace, sourceAuthJson, sourceKeyName)
		// Expect(err).ToNot(HaveOccurred())

		_, err = framework.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		//_, err = framework.ReleaseController.CreateRegistryJsonSecret(hacbsReleaseTestsTokenSecret, managedNamespace, destinationAuthJson, destinationKeyName)
		Expect(err).ToNot(HaveOccurred())

		// Copy the public key from tekton-chains/signing-secrets to a new
		// secret that contains just the public key to ensure that access
		// to password and private key are not needed.
		publicKey, err := kubeController.GetPublicKey("signing-secrets", constants.TEKTON_CHAINS_NS)
		Expect(err).ToNot(HaveOccurred())
		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

		// Expect(err).ToNot(HaveOccurred())
		// GinkgoWriter.Printf("Copy public key from %s/signing-secrets to a new secret\n", constants.TEKTON_CHAINS_NS)
		// _, err = framework.ReleaseController.CreateRegistryJsonSecret(publicSecretNameAuth, managedNamespace, string(publicKey), publicSecretNameAuth)
		// Expect(err).ToNot(HaveOccurred())

	})

	// AfterAll(func() {
	// 	// Delete the dev and managed namespaces with all the resources created in them
	// 	Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
	// 	Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	// })

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
			_, err := framework.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, releaseEnvironment, "", releaseStrategyDefaultName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates EnterpriseContractPolicy in managed namespace.", func(ctx SpecContext) {
			_, err := framework.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, ecPolicy)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(EnterpriseContractPolicyTimeout))

		// It("Create Environment", func() {
		// 	_, err := framework.GitOpsController.CreateEnvironment(releaseEnvironment, managedNamespace, gitopsv1alpha1.DeploymentStrategy_Manual, gitopsv1alpha1.EnvironmentType_POC, displayEnvironment) //, myEnvironment)
		// 	Expect(err).NotTo(HaveOccurred())
		// })

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
		// var snapshotCreatedInDev = &appstudioApi.Snapshot{}

		It("A PipelineRun should have been created in the dev namespace", func() {
			Eventually(func() bool {
				prList, err := framework.TektonController.ListAllPipelineRuns(devNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					klog.Error(err)
					return false
				}

				return strings.Contains(prList.Items[0].Name, componentName)
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})
	})

	It("A PipelineRun should have been created in the dev namespace and succeeded", func() {
		Eventually(func() bool {
			prList, err := framework.TektonController.ListAllPipelineRuns(devNamespace)
			if prList == nil || err != nil || len(prList.Items) < 1 {
				klog.Error(err)
				return false
			}

			return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
		}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
	})

	// get snapshot from deb namespace
	It("gets snapshot created in dev namepsace", func() {
		snapshotCreatedInDev, err := framework.ReleaseController.GetSnapshotInNamespace(devNamespace, componentName)
		Expect(err).NotTo(HaveOccurred())
		klog.Info("Snapshot is : ", snapshotCreatedInDev.Name)
	})

})
