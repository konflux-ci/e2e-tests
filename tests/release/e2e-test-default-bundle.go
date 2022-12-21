package release

import (
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"knative.dev/pkg/apis"

	// gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
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

var _ = framework.ReleaseSuiteDescribe("test-release-service-happy-path", Label("release", "defaultBundle"), func() {
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

		// Kasem TODO findout why it's

		//utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_SOURCE, ""))
		sourceAuthJson := "ewogICJhdXRocyI6IHsKICAgICJxdWF5LmlvIjogewogICAgICAiYXV0aCI6ICJjbVZzWldGelpTMWxNbVVyY21Wc1pXRnpaVjlsTW1VNlVUTkNRa1JOTWtaTk4waFNUVWxHVWxsR09GaFNOVWROTlZkUlFqWklXVXRKTjBwWVJrbzJTMXBQVUV4WFJVOVVWamxUUVVOVk9WRkZXRGxTU2pCRlJ3PT0iLAogICAgICAiZW1haWwiOiAiIgogICAgfQogIH0KfQ=="

		//utils.GetEnv(constants.QUAY_OAUTH_TOKEN_RELEASE_DESTINATION, "")
		destinationAuthJson := "ewogICJhdXRocyI6IHsKICAgICJxdWF5LmlvIjogewogICAgICAiYXV0aCI6ICJhR0ZqWW5NdGNtVnNaV0Z6WlMxMFpYTjBjeXR0TlY5eWIySnZkRjloWTJOdmRXNTBPazh5V0ZSWlFVaFJSa1ZZVEZCWVFWRk5WRUpZTnpNMVFWcEhNbGhIVFRGSU1qVkZRMVZTV0ZkUlVFWlpSVW8xUzBwVk1rZzRUazlVV0UxQk1sZFBWRVk9IiwKICAgICAgImVtYWlsIjogIiIKICAgIH0KICB9Cn0="

		Expect(sourceAuthJson).ToNot(BeEmpty())
		Expect(destinationAuthJson).ToNot(BeEmpty())

		// _, err = framework.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		// _, err = framework.ReleaseController.CreateRegistryJsonSecret(redhatAppstudioUserSecret, devNamespace, sourceAuthJson, sourceKeyName)
		// Expect(err).ToNot(HaveOccurred())

		_, err = framework.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, managedNamespace, destinationAuthJson)
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

	var _ = Describe("Creation of the 'Happy path' resources", func() {

		It("Create PVC", func() {
			_, err := framework.TektonController.CreatePVCAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Service Account", func() {
			_, err := framework.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
			Expect(err).NotTo(HaveOccurred())
		})

		// It("Create Environment", func() {
		// 	_, err := framework.GitOpsController.CreateEnvironment(releaseEnvironment, managedNamespace, gitopsv1alpha1.DeploymentStrategy_Manual, gitopsv1alpha1.EnvironmentType_POC, myEnvironment)
		// 	Expect(err).NotTo(HaveOccurred())
		// })

		It("Create Release Strategy", func() {
			_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineNameDefault, releasePipelineBundleDefault, releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, paramsReleaseStrategy)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ReleasePlan in dev namespace", func() {
			_, err := framework.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ReleasePlanAdmission in managed namespace", func() {
			_, err := framework.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, releaseEnvironment, "", releaseStrategyName)
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

		It("A PipelineRun should have been created in the dev namespace", func() {
			Eventually(func() bool {
				pipelineRun, err := framework.TektonController.GetPipelineRunInNamespace(devNamespace)
				if pipelineRun == nil || err != nil || len(pipelineRun.Items) < 1 {
					return false
				}
				return (&pipelineRun.Items[0]).HasStarted() && (&pipelineRun.Items[0]).IsDone() && (&pipelineRun.Items[0]).Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
		})
	})

	It("A PipelineRun should have been created in the managed namespace", func() {
		Eventually(func() bool {
			pipelineRun, err := framework.TektonController.GetPipelineRunInNamespace(managedNamespace)
			if pipelineRun == nil || err != nil || len(pipelineRun.Items) < 1 {
				return false
			}
			return (&pipelineRun.Items[0]).HasStarted() && (&pipelineRun.Items[0]).IsDone() && (&pipelineRun.Items[0]).Status.GetCondition(apis.ConditionSucceeded).IsTrue()
		}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	})

})
