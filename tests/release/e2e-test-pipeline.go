package release

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	releasePipelineDefault     = "m5-release-pipeline"
	releasePvcName             = "release-pvc"
	serviceAccountName         = "m5-service-account"
	secretName                 = "hacbs-release-tests-token"
	appNamePipelineTest        = "m5-application"
	componenetNamePipelineTest = "helloworld"
	componentUrl               = "https://github.com/scoheb/go-hello-world.git"
	componentDockerFileUrl     = "https://github.com/scoheb/go-hello-world/blob/main/Dockerfile"
	buildBundleName            = "build-pipelines-defaults"
	defaultBuildBundle         = "quay.io/redhat-appstudio/hacbs-templates-bundle:latest"
	releasePolicyDefault       = "m5-policy"
	releaseStrategyDefaultName = "m5-strategy"
)

var releaseStartegyParams = []v1alpha1.Params{
	{Name: "extraConfigGitUrl", Value: "https://github.com/davidmogar/strategy-configs.git"},
	{Name: "extraConfigPath", Value: "m5.yaml"},
	{Name: "extraConfigRevision", Value: "main"},
}

var serviceAccountSecretList = []corev1.ObjectReference{
	{
		Name: serviceAccountName,
	},
}

var _ = framework.ReleaseSuiteDescribe("release-suite-e2e-tekton-pipeline", func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace = uuid.New().String()
	var managedNamespace = uuid.New().String()

	BeforeAll(func() {
		// Create the dev namespace
		demo, err := framework.CommonController.CreateTestNamespace(devNamespace)
		klog.Info("Dev namespace:", devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", demo.Name, err)

		// Create the managed namespace
		namespace, err := framework.CommonController.CreateTestNamespace(managedNamespace)
		klog.Info("Release namespace:", managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", namespace.Name, err)
	})

	// AfterAll(func() {
	// 	// Delete the dev and managed namespaces with all the resources created in them
	// 	Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
	// 	Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	// })

	var _ = Describe("Creation of the 'tekton test-bundle e2e-test' resources", func() {
		It("Create PVC in", func() {
			err := framework.CommonController.CreatePVC(releasePvcName, managedNamespace, corev1.ReadWriteMany)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Strategy", func() {
			_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyDefaultName, managedNamespace, releasePipelineDefault, defaultBuildBundle, releasePolicyDefault, serviceAccountName, releaseStartegyParams)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Link in dev namespace", func() {
			_, err := framework.ReleaseController.CreateReleaseLink(sourceReleaseLinkName, devNamespace, appNamePipelineTest, managedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Link in managed namespace", func() {
			_, err := framework.ReleaseController.CreateReleaseLink(targetReleaseLinkName, managedNamespace, appNamePipelineTest, devNamespace, "m5-strategy") //releaseStrategyName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Service account", func() {
			_, err := framework.CommonController.CreateServiceAccount(serviceAccountName, managedNamespace, serviceAccountSecretList)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create ConfigMap ", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: buildBundleName},
				Data:       map[string]string{"default_build_bundle": defaultBuildBundle},
			}
			_, err = framework.CommonController.CreateConfigMap(cm, devNamespace)
			Expect(err).ToNot(HaveOccurred())

		})

		It("Create an application", func() {
			_, err := framework.HasController.CreateHasApplication(appNamePipelineTest, devNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create componenet", func() {
			_, err := framework.HasController.CreateComponent(appNamePipelineTest, componenetNamePipelineTest, devNamespace, componentUrl, componentDockerFileUrl, "", "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	// var _ = Describe("Post-release verification", func() {
	// 	It("A PipelineRun should have been created in the managed namespace", func() {
	// 		Eventually(func() error {
	// 			_, err := framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, devNamespace)

	// 			return err
	// 		}, 1*time.Minute, defaultInterval).Should(BeNil())
	// 	})

	// 	It("The PipelineRun should exist and succeed", func() {
	// 		Eventually(func() bool {
	// 			pipelineRun, err := framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, devNamespace)

	// 			if pipelineRun == nil || err != nil {
	// 				return false
	// 			}

	// 			return pipelineRun.HasStarted() && pipelineRun.IsDone() && pipelineRun.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
	// 		}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	// 	})

	// 	It("The Release should have succeeded", func() {
	// 		Eventually(func() bool {
	// 			release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

	// 			if err != nil {
	// 				return false
	// 			}

	// 			return release.IsDone() && meta.IsStatusConditionTrue(release.Status.Conditions, "Succeeded")
	// 		}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	// 	})

	// 	It("The Release should reference the release PipelineRun", func() {
	// 		var pipelineRun *v1beta1.PipelineRun

	// 		Eventually(func() bool {
	// 			pipelineRun, err = framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, devNamespace)

	// 			return pipelineRun != nil && err == nil
	// 		}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())

	// 		release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)
	// 		Expect(err).ShouldNot(HaveOccurred())
	// 		Expect(release.Status.ReleasePipelineRun).Should(Equal(fmt.Sprintf("%s/%s", pipelineRun.Namespace, pipelineRun.Name)))
	// 	})
	// })
})
