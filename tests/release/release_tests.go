package release

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"k8s.io/klog"
)

var _ = framework.ReleaseSuiteDescribe("release-suite-test-role", Label("test-role-binding"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace = uuid.New().String()
	var managedNamespace = uuid.New().String()

	BeforeAll(func() {
		// Create the dev namespace
		demo, err := framework.CommonController.CreateTestNamespace(devNamespace)
		klog.Info("Dev namespace:", demo.Name)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", demo.Name, err)

		// Create the managed namespace
		namespace, err := framework.CommonController.CreateTestNamespace(managedNamespace)
		klog.Info("Release namespace:", namespace.Name)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", namespace.Name, err)
	})

	AfterAll(func() {
		// Delete the dev and managed namespaces with all the resources created in them
		Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
		Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	})

	var _ = Describe("Creation of the 'tekton test-bundle e2e-test' resources", func() {

		It("Create Role", func() {
			var roleRules = map[string][]string{
				"apiGroupsList": {""},
				"roleResources": {"secrets"},
				"roleVerbs":     {"get", "list", "watch"},
			}
			_, err := framework.CommonController.CreateRole(roleName, managedNamespace, roleRules)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Create RoleBinding", func() {
			_, err := framework.CommonController.CreateRoleBinding(roleBindingName, managedNamespace, subjectKind, serviceAccountName, roleRefKind, roleRefName, roleRefApiGroup)
			Expect(err).NotTo(HaveOccurred())

		})
	})

})
