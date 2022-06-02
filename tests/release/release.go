package release

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"k8s.io/klog/v2"
)

const (
	DemoNamespace       = "demo"
	ManagedNamespace    = "managed"
	ApplicationSnapshot = "m5-snapshot"
	ReleaseLinkDemo     = "m5-release-link-demo"
	ReleaseLinkManaged  = "m5-release-link-managed"
	ReleaseName         = "m5-release"
	ComponentName       = "m4-component"
	ApplicationName     = "m4-app"
)

var _ = framework.ReleaseStrategyDescribe("test-demo", func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	// Create required resources before Test
	BeforeAll(func() {
		demo, err := framework.HasController.CreateTestNamespace(DemoNamespace)
		klog.Info("NameSpace created: ", demo.Name)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", DemoNamespace, err)

		namespace, err := framework.HasController.CreateTestNamespace(ManagedNamespace)
		klog.Info("NameSpace created: ", namespace.Name)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", ManagedNamespace, err)
	})

	// teardown after test is ened
	// AfterAll(func() {

	// 	_, err := framework.HasController.DeleteTestNamespace(DemoNamespace)
	// 	klog.Info("NameSpace '%s' is deleted!': ", DemoNamespace)
	// 	// Expect(err).NotTo(HaveOccurred(), "Error when deleting '%s' namespace: %v", DemoNamespace, err)

	// 	_, err = framework.HasController.DeleteTestNamespace(ManagedNamespace)
	// 	klog.Info("NameSpace '%s' is deleted!': ", ManagedNamespace)
	// 	// Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", ManagedNamespace, err)
	// 	klog.Info("AfetrAll is Done!: '%v'", err)
	// })

	// Create resources for Happy Path demo
	var _ = Describe("Happy-path test", func() {
		It("Create Release Link in namespace demo", func() {
			_, err := framework.ReleaseController.CreateReleaseLink("demo", "demo", "Users's ReleaseLink", "m4-app", "managed", "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Link in namespace managed", func() {
			_, err := framework.ReleaseController.CreateReleaseLink("managed", "managed", "Managed Workspace's ReleaseLink", "m4-app", "demo", "m4-strategy")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Strategy", func() {
			_, err := framework.ReleaseController.CreateReleaseStrategy("m4-strategy", "managed", "m4-release-pipeline", "quay.io/hacbs-release/m4:0.1-alpine")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create a Release for M5", func() {
			_, err := framework.ReleaseController.CreateRelease(ReleaseName, DemoNamespace, ApplicationSnapshot, ReleaseLinkDemo)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	// 1-The release succeeded
	// 2-The reason is succeeded
	// 3-There's a PR in the managed workspace
	// 4-The ReleasePipelineRun in the release status points to an existing PR (you can leave this for M5)
	//
	// Verification of test resources: Demo
	var _ = Describe("Happy-path Test Verification", func() {

		// Verify the release
		It("Test the release ", func() {
			currentrelease, err := framework.ReleaseController.GetRelease(DemoNamespace)
			klog.Info("Release is %s : %s", currentrelease, err)
			Expect(err).NotTo(HaveOccurred())
			// TODO u got release, need to test status and reason
		})
	})
})
