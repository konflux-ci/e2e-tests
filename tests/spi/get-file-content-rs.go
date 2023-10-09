package spi

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

/*
 * Component: spi
 * Description: SVPI-402 - Get file content from a private Github repository
 * Use case: Remote Secret Usage
 */

var _ = framework.SPISuiteDescribe(Label("spi-suite", "get-file-content-rs"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace, targetSecretName string
	var remoteSecret *rs.RemoteSecret
	var SPIFcr *v1beta1.SPIFileContentRequest
	remoteSecretName := "test-remote-secret"
	AfterEach(framework.ReportFailure(&fw))

	Describe("SVPI-402 - Get file content from a private Github repository with Remote Secret", Ordered, func() {
		BeforeAll(func() {
			if os.Getenv("CI") != "true" {
				Skip(fmt.Sprintln("test skipped on local execution"))
			}
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			Expect(namespace).NotTo(BeEmpty())

			// collect SPI ResourceQuota metrics (temporary)
			err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("get-file-content-rs", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			// collect SPI ResourceQuota metrics (temporary)
			err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("get-file-content-rs", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())

			if !CurrentSpecReport().Failed() {
				Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
			}
		})

		It("creates RemoteSecret", func() {
			targets := []rs.RemoteSecretTarget{{Namespace: namespace}}
			labels := map[string]string{"appstudio.redhat.com/sp.host": "github.com"}

			remoteSecret, err = fw.AsKubeDeveloper.RemoteSecretController.CreateRemoteSecret(remoteSecretName, namespace, targets, v1.SecretTypeBasicAuth, labels)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err = fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionFalse(remoteSecret.Status.Conditions, "DataObtained")
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s is not waiting for data", namespace, remoteSecretName))
		})

		It("creates upload secret", func() {
			data := map[string]string{
				"password": utils.GetEnv(constants.GITHUB_TOKEN_ENV, ""),
			}

			s := fw.AsKubeDeveloper.RemoteSecretController.BuildSecret(remoteSecret.Name, v1.SecretTypeBasicAuth, data)

			_, err = fw.AsKubeAdmin.CommonController.CreateSecret(namespace, s)
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

		It("creates SPIFileContentRequest", func() {
			SPIFcr, err = fw.AsKubeDeveloper.SPIController.CreateSPIFileContentRequest("gh-spi-filecontent-request", namespace, GithubPrivateRepoURL, GithubPrivateRepoFilePath)
			Expect(err).NotTo(HaveOccurred())

			SPIFcr, err = fw.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(SPIFcr.Name, namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("SPIFileContentRequest should be in Delivered phase and content should be provided", func() {
			fw.AsKubeDeveloper.SPIController.IsSPIFileContentRequestInDeliveredPhase(SPIFcr)
		})
	})
})
