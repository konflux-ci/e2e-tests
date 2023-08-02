package spi

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
 * Component: spi
 * Description: SVPI-541 - Basic remote secret functionalities
 */

// pending because https://github.com/redhat-appstudio/remote-secret/pull/57 will break the tests
// we will need to update the current test after merging the PR
var _ = framework.SPISuiteDescribe(Label("spi-suite", "remote-secret"), Pending, func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var remoteSecret *v1beta1.RemoteSecret
	targetNamespace1 := "spi-test-target1"
	targetNamespace2 := "spi-test-target2"
	remoteSecretName := "test-remote-secret"
	targetSecretName1 := ""
	targetSecretName2 := ""

	Describe("SVPI-541 - Basic remote secret functionalities", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())

			_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(targetNamespace1)
			Expect(err).NotTo(HaveOccurred(), "Error when creating %s namespace: %v", targetNamespace1, err)

			_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(targetNamespace2)
			Expect(err).NotTo(HaveOccurred(), "Error when creating %s namespace: %v", targetNamespace2, err)
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllRemoteSecretsInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.CommonController.DeleteAllSecretsInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.CommonController.DeleteAllSecretsInASpecificNamespace(targetNamespace1)).To(Succeed())
				Expect(fw.AsKubeAdmin.CommonController.DeleteAllSecretsInASpecificNamespace(targetNamespace2)).To(Succeed())
			}
		})

		It("creates RemoteSecret with previously created namespaces as targets", func() {
			remoteSecret, err = fw.AsKubeDeveloper.SPIController.CreateRemoteSecret(remoteSecretName, namespace, []string{targetNamespace1, targetNamespace2})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err = fw.AsKubeDeveloper.SPIController.GetRemoteSecret(remoteSecretName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionFalse(remoteSecret.Status.Conditions, "DataObtained")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s is not waiting for data", namespace, remoteSecretName))
		})

		It("creates upload secret", func() {
			s := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
					Labels: map[string]string{
						"appstudio.redhat.com/upload-secret": "remotesecret",
					},
					Annotations: map[string]string{
						"appstudio.redhat.com/remotesecret-name": remoteSecret.Name,
					},
				},
				Type: v1.SecretTypeOpaque,
				StringData: map[string]string{
					"a": "b",
					"c": "d",
				},
			}

			_, err = fw.AsKubeAdmin.CommonController.CreateSecret(namespace, s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("checks if remote secret was deployed", func() {
			Eventually(func() bool {
				remoteSecret, err = fw.AsKubeDeveloper.SPIController.GetRemoteSecret(remoteSecretName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(remoteSecret.Status.Conditions, "Deployed")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s is not in deployed phase", namespace, remoteSecretName))
		})

		It("checks targets in RemoteSecret status", func() {
			remoteSecret, err = fw.AsKubeDeveloper.SPIController.GetRemoteSecret(remoteSecret.Name, namespace)
			Expect(err).NotTo(HaveOccurred())

			targets := remoteSecret.Status.Targets
			Expect(targets).To(HaveLen(2))

			// get targetSecretName1
			targetSecretName1 = fw.AsKubeDeveloper.SPIController.GetTargetSecretName(targets, targetNamespace1)
			Expect(targetSecretName1).ToNot(BeEmpty())

			// get targetSecretName12
			targetSecretName2 = fw.AsKubeDeveloper.SPIController.GetTargetSecretName(targets, targetNamespace2)
			Expect(targetSecretName2).ToNot(BeEmpty())
		})

		It("checks if secret was created in target namespaces", func() {
			_, err = fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace1, targetSecretName1)
			Expect(err).NotTo(HaveOccurred())

			_, err = fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace2, targetSecretName2)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
