package release

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"k8s.io/klog/v2"
)

const (
	DemoNamespace              = "demo"
	ManagedNamespace           = "managed"
	Demo_secret_yaml_file_path = "/home/nshprai/Setup/App-Studio/e2e-tests/resources/secrets/release-e2e-release-e2e-secret.yml"
)

var _ = framework.ReleaseStrategyDescribe("test-demo", func() {
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		namespace, err := framework.HasController.CreateTestNamespace(DemoNamespace)
		klog.Infof("Namespace %s created", namespace.Name)
		Expect(err).NotTo(HaveOccurred(), "Error when creating '%s' namespace: %v", DemoNamespace, err)

		namespace, err = framework.HasController.CreateTestNamespace(ManagedNamespace)
		klog.Infof("Namespace %s created", namespace.Name)
		Expect(err).NotTo(HaveOccurred(), "Error when creating '%s' namespace: %v", ManagedNamespace, err)

		// _, err = framework.ReleaseController.CreateSecret(Demo_secret_yaml_file_path)
		// _, err = framework.ReleaseController.CreateSecretV2(Demo_secret_yaml_file_path)
		// _, err = framework.ReleaseController.CreateSecretV3("redhat-appstudio-registry-pull-secret", DemoNamespace)
		// Expect(err).NotTo(HaveOccurred(), "Error when creating secret in '%s' namespace: %v", DemoNamespace, err)
	})

	AfterAll(func() {
		err := framework.ReleaseController.DeleteTestNamespace(DemoNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error while deleting '%s' namespace: %v", DemoNamespace, err)

		err = framework.ReleaseController.DeleteTestNamespace(ManagedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error while deleting '%s' namespace: %v", ManagedNamespace, err)

		Eventually(func() bool {
			// demo and managed namespaces should not exist

			ret1 := framework.ReleaseController.CheckIfNamespaceExists(DemoNamespace)
			ret2 := framework.ReleaseController.CheckIfNamespaceExists(ManagedNamespace)

			// return True if only one namespace still exists
			// return False if both demo and managed namespaces don't exist
			return ret1 || ret2
		}, 2*time.Minute, 1000*time.Millisecond).Should(BeFalse(), "Release controller didn't remove demo and/or managed namespace")

	})

	//
	var _ = Describe("Happy-path test", func() {
		// var book *books.Book

		// BeforeEach(func() {
		//   book = &books.Book{
		// 	Title: "Les Miserables",
		// 	Author: "Victor Hugo",
		// 	Pages: 2783,
		//   }
		//   Expect(book.IsValid()).To(BeTrue())
		// })

		Describe("Creating prerequisites for the Happy-path", func() {
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

			// It("Create Application", func() {
			// 	_, err := framework.HasController.CreateHasApplication("m4-app", "demo")
			// 	Expect(err).NotTo(HaveOccurred())
			// })

			// It("Create component", func() {
			// 	framework.HasController.CreateComponent("m4-app", "m4-component", "demo", "https://github.com/sbose78/devfile-sample-code-with-quarkus", "quay.io/release-e2e/devfile-sample-code-with-quarqus:dev-code-with-quarkus", "")
			// })
		})

		Describe("Testing the Happy-path", func() {
			// It("Create Release Link in namespace managed", func() {
			// 	_, err := framework.ReleaseController.CreateReleaseLink("managed", "managed", "Managed Workspace's ReleaseLink", "m4-app", "demo", "m4-strategy")
			// 	Expect(err).NotTo(HaveOccurred())
			// })

			// It("Create Release Strategy", func() {
			// 	_, err := framework.ReleaseController.CreateReleaseStrategy("m4-strategy", "managed", "m4-release-pipeline", "quay.io/hacbs-release/m4:0.1-alpine")
			// 	Expect(err).NotTo(HaveOccurred())
			// })

			// It("Create Application", func() {
			// 	_, err := framework.HasController.CreateHasApplication("m4-app", "demo")
			// 	Expect(err).NotTo(HaveOccurred())
			// })

		})

	})

})
