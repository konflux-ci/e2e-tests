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
)

/*
 * Component: remote secret
 * Description: SVPI-558 - Test all the options of the authz of remote secret target deployment
 * Test case: Target to the same namespace where the remote secret lives is always deployed
 */

var _ = framework.RemoteSecretSuiteDescribe(Label("remote-secret", "target-current-namespace"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var remoteSecret *v1beta1.RemoteSecret
	remoteSecretName := fmt.Sprintf("test-remote-secret-%s", util.GenerateRandomString(4))
	targetSecretName := ""

	Describe("SVPI-558 - Target to the same namespace where the remote secret lives is always deployed", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
			}
		})

		It("creates RemoteSecret with a target that shares the same namespace", func() {
			targets := []v1beta1.RemoteSecretTarget{{Namespace: namespace}}
			remoteSecret, err = fw.AsKubeDeveloper.RemoteSecretController.CreateRemoteSecret(remoteSecretName, namespace, targets, v1.SecretTypeOpaque, map[string]string{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err = fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName, namespace)
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
				remoteSecret, err = fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(remoteSecret.Status.Conditions, "Deployed")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s is not in deployed phase", namespace, remoteSecretName))
		})

		It("checks targets in RemoteSecret status", func() {
			remoteSecret, err = fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecret.Name, namespace)
			Expect(err).NotTo(HaveOccurred())

			targets := remoteSecret.Status.Targets
			Expect(targets).To(HaveLen(1))

			// get targetSecretName
			targetSecretName = fw.AsKubeDeveloper.RemoteSecretController.GetTargetSecretName(targets, namespace)
			Expect(targetSecretName).ToNot(BeEmpty())
		})

		It("checks if secret was created in target namespace", func() {
			_, err = fw.AsKubeAdmin.CommonController.GetSecret(namespace, targetSecretName)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
