package e2e

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	client "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	OCPManagedEnvironmentSecretName = "ocp-kubeconfig-secret"
)

var (
	OCPDeploymentsTargetNamespace = utils.GetGeneratedNamespace("byoc-deployment")
)

var _ = framework.E2ESuiteDescribe(Label("byoc", "openshift"), Ordered, func() {
	defer GinkgoRecover()
	var fw *framework.Framework
	var err error
	var byocKubeconfig string
	var byocAPIServerURL string
	var ephemeralClusterClient *kubernetes.Clientset

	Describe("Deploy RHTAP application in Openshift clusters", func() {
		BeforeAll(func() {
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("byoc-ocp"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("initializes openshfit environment client", func() {
			byocKubeconfig = utils.GetEnv("BYOC_KUBECONFIG", "")
			if byocKubeconfig == "" {
				Fail("Please provide BYOC_KUBECONFIG env pointing to a valid openshift kubeconfig file")
			}

			config, err := clientcmd.BuildConfigFromFlags("", byocKubeconfig)

			Expect(err).NotTo(HaveOccurred())
			byocAPIServerURL = config.Host

			ephemeralClusterClient, err = client.NewKubeFromKubeConfigFile(byocKubeconfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates managed-gitops.redhat.com/managed-environment secret type", func() {
			kubeConfData, err := os.ReadFile(byocKubeconfig)
			data := make(map[string][]byte)
			data["kubeconfig"] = kubeConfData
			Expect(err).NotTo(HaveOccurred())

			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      OCPManagedEnvironmentSecretName,
					Namespace: fw.UserNamespace,
				},
				Type: v1.SecretType(ManagedEnvironmentType),
				Data: data,
			}

			_, err = fw.AsKubeAdmin.CommonController.CreateSecret(fw.UserNamespace, secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates environment", func() {
			// Note: It is not possible yet to configure integration service snapshots to deploy to a specific environment.
			// As an workaround for now: Deletes the development environment and recreate it with kubernetes cluster information
			Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 1*time.Minute)).NotTo(HaveOccurred())

			environment, err := fw.AsKubeAdmin.GitOpsController.CreateEphemeralEnvironment(KubernetesEnvironmentName, fw.UserNamespace, OCPDeploymentsTargetNamespace, byocAPIServerURL, OCPManagedEnvironmentSecretName)
			Expect(environment.Name).To(Equal(KubernetesEnvironmentName))
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates namespace in the ephemeral target cluster", func() {
			ns, err := ephemeralClusterClient.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: OCPDeploymentsTargetNamespace}}, metav1.CreateOptions{})
			Expect(ns.Name).To(Equal(OCPDeploymentsTargetNamespace))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
