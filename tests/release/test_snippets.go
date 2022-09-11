package release

import (
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

// Test various functions towards full happy-path tests construction
var _ = framework.ReleaseSuiteDescribe("test-release-service-test-snippets", Label("release"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace string
	var managedNamespace string

	var _ = Describe("test-snippets - create and delete namespaces", func() {
		BeforeAll(func() {
			// Recreate random namespaces names per each test because if using same namespace names, the next test will not be able to create the namespaces as they are terminating
			devNamespace = "user-" + uuid.New().String()
			managedNamespace = "managed-" + uuid.New().String()

			//debug
			fmt.Printf("debug: devNamespace = %s;  managedNamespace = %s \n", devNamespace, managedNamespace)
		})

		var _ = Describe("Create dev and managed namespaces", func() {
			It("Create dev namespace.", func() {
				// Create the dev namespace
				_, err := framework.CommonController.CreateTestNamespace(devNamespace)
				Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", devNamespace, err)

				//debug
				if err != nil {
					fmt.Printf("debug: CreateTestNamespace Error = %s \n", err.Error())
				}
			})

			It("Create managed namespace.", func() {
				// Create the dev namespace
				_, err := framework.CommonController.CreateTestNamespace(managedNamespace)
				Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

				//debug
				if err != nil {
					fmt.Printf("debug: CreateTestNamespace Error = %s \n", err.Error())
				}
			})
		})

		var _ = Describe("Delete dev and managed namespaces", func() {
			It("Delete dev namespace.", func() {
				// Create the dev namespace
				err := framework.CommonController.DeleteNamespace(devNamespace)
				Expect(err).NotTo(HaveOccurred(), "Error when deleting namespace '%s': %v", devNamespace, err)

				//debug
				if err != nil {
					fmt.Printf("debug: DeleteNamespace Error = %s \n", err.Error())
				}
			})

			It("Delete managed namespace.", func() {
				// Create the dev namespace
				err := framework.CommonController.DeleteNamespace(managedNamespace)
				Expect(err).NotTo(HaveOccurred(), "Error when deleting namespace '%s': %v", managedNamespace, err)

				//debug
				if err != nil {
					fmt.Printf("debug: CreateTestNamespace Error = %s \n", err.Error())
				}
			})
		})
	})
})
