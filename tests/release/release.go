package release

import (
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"
	"knative.dev/pkg/apis"
)

var myEnvironment = []gitopsv1alpha1.EnvVarPair{
	{Name: releaseEnvironment, Value: releaseEnvironment},
}
var managednamespaceSecret = []corev1.ObjectReference{
	{Name: hacbsReleaseTestsTokenSecret},
}

var _ = framework.ReleaseSuiteDescribe("test-release-service-happy-path", Label("release"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace = uuid.New().String()
	var managedNamespace = uuid.New().String()

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

		sourceAuthJson := utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_SOURCE)
		destinationAuthJson := utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_DESTINATION)

		_, err = framework.ReleaseController.CreateRegistryJsonSecret(redhatAppstudioUserSecret, devNamespace, sourceAuthJson, sourceKeyName)
		Expect(err).ToNot(HaveOccurred())

		_, err = framework.ReleaseController.CreateRegistryJsonSecret(hacbsReleaseTestsTokenSecret, managedNamespace, destinationAuthJson, destinationKeyName)
		Expect(err).ToNot(HaveOccurred())

		// Copy the public key from tekton-chains/signing-secrets to a new
		// secret that contains just the public key to ensure that access
		// to password and private key are not needed.
		publicKey, err := kubeController.GetPublicKey("signing-secrets", constants.TEKTON_CHAINS_NS)
		Expect(err).ToNot(HaveOccurred())
		GinkgoWriter.Printf("Copy public key from %s/signing-secrets to a new secret\n", constants.TEKTON_CHAINS_NS)
		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

	})

	AfterAll(func() {
		// Delete the dev and managed namespaces with all the resources created in them
		Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
		Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	})

	var _ = Describe("Creation of the 'Happy path' resources", func() {

		It("Create PVC", func() {
			_, err := framework.TektonController.CreatePVCAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Service Account", func() {
			_, err := framework.CommonController.CreateServiceAccount(releaseStrategyServiceAccount, managedNamespace, managednamespaceSecret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Environment", func() {
			_, err := framework.GitOpsController.CreateEnvironment(releaseEnvironment, managedNamespace, gitopsv1alpha1.DeploymentStrategy_Manual, gitopsv1alpha1.EnvironmentType_POC, myEnvironment)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Strategy", func() {
			_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy, releaseStrategyServiceAccount)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ReleasePlan in dev namespace", func() {
			_, err := framework.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationName, managedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ReleasePlanAdmission in managed namespace", func() {
			_, err := framework.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationName, managedNamespace, releaseEnvironment, "", releaseStrategyName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("CreateApplication", func() {
			_, err := framework.HasController.CreateHasApplication(applicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("CreateComponent", func() {
			_, err := framework.HasController.CreateComponent(applicationName, componentName, devNamespace, gitSourceComponentUrl, "", "", "", "")
			Expect(err).NotTo(HaveOccurred())
		})

	})

	var _ = Describe("Post-release verification", func() {

		It("A PipelineRun should have been created in the dev namespace", func() {
			Eventually(func() bool {
				pipelineRun, err := framework.TektonController.GetPipelineRunInNamespace(devNamespace)
				myPR := &pipelineRun.Items[0]
				if pipelineRun == nil || err != nil {
					return false
				}
				return myPR.HasStarted() && myPR.IsDone() && myPR.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
		})
	})

	It("A PipelineRun should have been created in the managed namespace", func() {
		Eventually(func() bool {
			pipelineRun, err := framework.TektonController.GetPipelineRunInNamespace(managedNamespace)
			myPR := &pipelineRun.Items[0]
			if pipelineRun == nil || err != nil {
				return false
			}
			return myPR.HasStarted() && myPR.IsDone() && myPR.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
		}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	})

})
