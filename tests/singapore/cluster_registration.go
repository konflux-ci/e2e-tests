package singapore

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

var _ = framework.SingaporeSuiteDescribe("Cluster Registration tests", func() {
	defer GinkgoRecover()

	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		// Check to see if the kubeconfig was provided for a cluster to be imported in appstudio
		Expect(utils.CheckIfEnvironmentExists(AppstudioImpoterdClusterKubeconfigEnv)).Should(BeTrue(), "%s environment variable is not set", "APPSTUDIO_KUBECONFIG")

		// Ensure Kind cluster has been created

		// and a kubeconfig is created

	})

	AfterAll(func() {

		// delete UserSingup: it will clear all space resources automatically
		err := framework.SingaporeController.DeleteAppstudioSandboxWorkspaceUserSignUp(UserAccountName, UserAccountNamespace)
		Expect(err).NotTo(HaveOccurred())

		err = framework.SingaporeController.DeleteRegisteredClusterCR(ImportedUserClusterName, ImportedUserClusterNamespace)
		Expect(err).NotTo(HaveOccurred())

	})

	// create appstudio workspace from sandbox
	// TDB: determine the correct way of deploying it: is a UserSignup CR enough?
	It("Create Appstudio workspace", func() {
		err := framework.SingaporeController.CreateAppstudioSandboxWorkspaceUserSignUp(UserAccountName, UserAccountNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	// Check for Space created
	It("Check if Appstudio workspace was created", func() {
		Eventually(func() bool {
			return framework.SingaporeController.CheckIfAppstudioSpaceExists(UserAccountName)
		}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "%s Appstudio Space not created", UserAccountName)
	})

	// Check for ClusterSet created
	It("Check if a ClusterSet  was created", func() {
		Eventually(func() bool {
			return framework.SingaporeController.CheckIfClusterSetExists(UserAccountName)
		}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "%s Cluster Set not created", UserAccountName)
	})

	// Create a RegisteredCluster custom resource
	// Cluster has to be registered inside the user's workspace in order to be included in the proper cluster set
	It("Create Registered Cluster CR", func() {
		err := framework.SingaporeController.CreateRegisteredClusterCR(ImportedUserClusterName, ImportedUserClusterNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	// Import cluster into AppStudio:
	// run oc get configmap -n <your_namespace> <name_of_cluster_to_import>-import -o jsonpath='{.data.importCommand}'
	// and run the command in the cluster you want to import
	It("Importing cluster in AppStudio", func() {
		configmapName := ImportedUserClusterName + "-import"

		Eventually(func() bool {
			_, err := framework.CommonController.GetConfigMap(configmapName, ImportedUserClusterNamespace)
			return err == nil
		}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "import secret not found")

		configmap, _ := framework.CommonController.GetConfigMap(configmapName, ImportedUserClusterNamespace)
		importCommand := configmap.Data["importCommand"]
		Expect(importCommand).NotTo(BeEmpty(), "importCommand data is empty")

		err = framework.SingaporeController.ExecuteImportCommand(AppstudioImpoterdClusterKubeconfigEnv, importCommand)
		Expect(err).NotTo(HaveOccurred())
	})

	// Check the cluster has been imported:
	// Watch the status.conditions of the RegisteredCluster CR. After several minutes the cluster should be successfully imported.
	//  oc get registeredcluster -n <your_namespace> -oyaml
	// The staus.clusterSecretRef will point to the Secret, <name_of_cluster_to_import>-cluster-secret ,containing the kubeconfig of the user cluster in data.kubeconfig.
	//  oc get secrets <name_of_cluster_to_import>-cluster-secret -n <your_namespace> -ojsonpath='{.data.kubeconfig}' | base64 -d
	It("Checking imported cluster health", func() {
		Eventually(func() bool {
			return framework.SingaporeController.CheckIfRegisteredClusterHasJoined(ImportedUserClusterName, ImportedUserClusterNamespace)
		}, 3*time.Minute, 15*time.Second).Should(BeTrue(), "%s Cluster Set not created", UserAccountName)

	})
})
