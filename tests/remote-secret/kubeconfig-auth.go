package remotesecret

import (
	"fmt"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

/*
 * Component: remote secret
 * Description: SVPI-558 - Test all the options of the authz of remote secret target deployment
 * Test case: Authentication using Kubeconfig
 */

var _ = framework.RemoteSecretSuiteDescribe(Label("remote-secret", "kubeconfig-auth"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var cfg *rest.Config
	var remoteSecret *v1beta1.RemoteSecret
	targetNamespace := fmt.Sprintf("test-target-namespace-%s", util.GenerateRandomString(4))
	secretName := fmt.Sprintf("test-remote-kubeconfig-%s", util.GenerateRandomString(4))
	remoteSecretName := "test-remote-cluster-secret"
	targetSecretName := ""

	Describe("SVPI-558 - Authentication using Kubeconfig", Ordered, func() {
		BeforeAll(func() {
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("rs-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())

			_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(targetNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating %s namespace: %v", targetNamespace, err)

			// get Kubeconfig
			cfg, err = config.GetConfig()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
				Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(targetNamespace)).To(Succeed())
			}
		})

		It("creates a secret with a kubeconfig", func() {
			kubeconfig := fmt.Sprintf(`
apiVersion: v1
kind: Config
current-context: ctx
clusters:
- name: cluster
  cluster:
    insecure-skip-tls-verify: %v
    server: %s
users:
- name: user
  user:
    token: %s
contexts:
- name: ctx
  context:
    cluster: cluster
    user: user
    namespace: %s`, cfg.Insecure, cfg.Host, cfg.BearerToken, namespace)

			s := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					"kubeconfig": []byte(kubeconfig),
				},
			}

			_, err = fw.AsKubeAdmin.CommonController.CreateSecret(namespace, s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates RemoteSecret with previously created namespace as target", func() {
			targets := []v1beta1.RemoteSecretTarget{
				{
					ApiUrl:                   cfg.Host,
					ClusterCredentialsSecret: secretName,
					Namespace:                targetNamespace,
				},
			}
			remoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.CreateRemoteSecret(remoteSecretName, namespace, targets, v1.SecretTypeOpaque, map[string]string{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.GetRemoteSecret(remoteSecretName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionFalse(remoteSecret.Status.Conditions, "DataObtained")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s is not waiting for data", namespace, remoteSecretName))
		})

		It("creates upload secret", func() {
			data := map[string]string{"a": "b", "c": "d"}

			_, err = fw.AsKubeAdmin.RemoteSecretController.CreateUploadSecret(remoteSecret.Name, namespace, remoteSecret.Name, v1.SecretTypeOpaque, data)
			Expect(err).NotTo(HaveOccurred())
		})

		It("checks if remote secret was deployed", func() {
			Eventually(func() bool {
				remoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.GetRemoteSecret(remoteSecretName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(remoteSecret.Status.Conditions, "Deployed")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s is not in deployed phase", namespace, remoteSecretName))
		})

		It("checks targets in RemoteSecret status", func() {
			remoteSecret, err = fw.AsKubeAdmin.RemoteSecretController.GetRemoteSecret(remoteSecret.Name, namespace)
			Expect(err).NotTo(HaveOccurred())

			targets := remoteSecret.Status.Targets
			Expect(targets).To(HaveLen(1))

			// get targetSecretName
			targetSecretName = fw.AsKubeAdmin.RemoteSecretController.GetTargetSecretName(targets, targetNamespace)
			Expect(targetSecretName).ToNot(BeEmpty())
		})

		It("checks if secret was created in target namespaces", func() {
			_, err = fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace, targetSecretName)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
