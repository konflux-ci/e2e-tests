package release

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"k8s.io/klog/v2"
)

const (
	DemoNamespace         = "demo"
	ManagedNamespace      = "managed"
	ApplicationSnapshot   = "m5-snapshot"
	ReleaseLinkDemo       = "m5-release-link-demo"
	ReleaseLinkManaged    = "m5-release-link-managed"
	ReleaseStrategy       = "m5-strategy"
	ReleaseName           = "m5-release"
	ComponentName         = "m4-component"
	Pipeline              = "release-pipeline"
	ApplicationName       = "m5-app"
	ReleaseStrategyBundle = "quay.io/hacbs-release/demo:m5-alpine"
	Image_1               = "quay.io/redhat-appstudio/component1@sha256:d5e85e49c89df42b221d972f5b96c6507a8124717a6e42e83fd3caae1031d514"
	Image_2               = "quay.io/redhat-appstudio/component2@sha256:a01dfd18cf8ca8b68770b09a9b6af0fd7c6d1f8644c7ab97f0e06c34dfc5860e"
	Image_3               = "quay.io/redhat-appstudio/component3@sha256:d90a0a33e4c5a1daf5877f8dd989a570bfae4f94211a8143599245e503775b1f"
)

var timeout = 300
var interval = 1
var _ = framework.ReleaseStrategyDescribe("test-demo", func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	// Create required resources before Test
	BeforeAll(func() {
		demo, err := framework.HasController.CreateTestNamespace(DemoNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", demo.Name, err)

		namespace, err := framework.HasController.CreateTestNamespace(ManagedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", namespace.Name, err)

	})

	// teardown after test is ened
	// AfterAll(func() {

	// 	_, err := framework.HasController.DeleteTestNamespace(DemoNamespace)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	_, err = framework.HasController.DeleteTestNamespace(ManagedNamespace)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	klog.Info("AfetrAll is Done!: ", err)
	// })

	// Create resources for Happy Path demo
	var _ = Describe("Happy-path test", func() {

		It("Create a an ApplicationSnapshot for M5", func() {
			_, err := framework.ReleaseController.CreateApplicationSnapshot(ApplicationSnapshot, DemoNamespace, Image_1, Image_2, Image_3, ApplicationName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Strategy", func() {
			_, err := framework.ReleaseController.CreateReleaseStrategy(ReleaseStrategy, ManagedNamespace, Pipeline, ReleaseStrategyBundle)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Link in namespace demo", func() {
			_, err := framework.ReleaseController.CreateReleaseLink(ReleaseLinkDemo, DemoNamespace, "Users's ReleaseLink", ApplicationName, ManagedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create Release Link in namespace managed", func() {
			_, err := framework.ReleaseController.CreateReleaseLink(ReleaseLinkManaged, ManagedNamespace, "Managed Workspace's ReleaseLink", ApplicationName, DemoNamespace, ReleaseStrategy)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create a Release for M5", func() {
			_, err := framework.ReleaseController.CreateRelease(ReleaseName, DemoNamespace, ApplicationSnapshot, ReleaseLinkDemo)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	// Verification of test resources: Demo
	var _ = Describe("Happy-path Test Verification", func() {

		// Check if there is a Pipelinerun in managed namespace
		It("Test if a PipelineRun in managed namespace", func() {
			pr, err := framework.ReleaseController.GetPipelineRunInNamespace(ManagedNamespace)
			Expect(err).NotTo(HaveOccurred())
			currentrelease, err := framework.ReleaseController.GetRelease(DemoNamespace)
			if err != nil {
				klog.Info("Release has not been created yet.")
				Expect(err).NotTo(HaveOccurred())
			}
			flag := false
			split := strings.Split(currentrelease.Status.ReleasePipelineRun, "/")
			if len(split) < 1 {
				time.Sleep(150)
				flag = true
			} else {
				releaseNamespace, releasePr := split[0], split[1]
				klog.Info("Pipeline in Release: ", releasePr)
				klog.Info("NameSpace from Release: ", releaseNamespace)
				Expect(releasePr).Should(Equal(pr.Name))
				Expect(releaseNamespace).Should(Equal(ManagedNamespace))
			}
			if flag {
				klog.Infof("The value of PipelineRun from Release is empty! split value: %v", split)
			}
		})

		// Verify the release Status we expect it be True
		It("Status of Release created should be true ", func() {
			Eventually(func() string {
				currentrelease, err := framework.ReleaseController.GetRelease(DemoNamespace)
				if err != nil {
					klog.Info("Release has not been created yet.")
					return "Unknown"
				}
				releaseStatus := currentrelease.Status.Conditions[0].Status
				klog.Info("Release Sataus: ", string(releaseStatus))
				return string(releaseStatus)
			}, timeout, interval).Should(Equal("True"), "timed out when waiting for the Release Status to be True")
		})

		// Verify Release Reason, we expect it be Succeeded
		It("Test Release Reason expexted to be Succeeded", func() {
			Eventually(func() string {
				currentrelease, err := framework.ReleaseController.GetRelease(DemoNamespace)
				if err != nil {
					klog.Info("Release has not been created yet.")
					return "Unknown"
				}
				releaseReason := currentrelease.Status.Conditions[0].Reason
				klog.Info("Release Reason: ", releaseReason)
				return releaseReason
			}, timeout, interval).Should(Equal("Succeeded"), "timed out when waiting for the Release Reason be Succeeded")
		})

		It("Delete Namespaces of test ", func() {
			_, err := framework.HasController.DeleteTestNamespace(DemoNamespace)
			Expect(err).NotTo(HaveOccurred())
			_, err = framework.HasController.DeleteTestNamespace(ManagedNamespace)
			Expect(err).NotTo(HaveOccurred())
			klog.Info("AfetrAll is Done!: ", err)
		})

	})
})
