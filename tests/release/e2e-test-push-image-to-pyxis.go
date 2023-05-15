package release

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"knative.dev/pkg/apis"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleaseSuiteDescribe("[HACBS-1571]test-release-e2e-push-image-to-pyxis", Label("release", "pushPyxis", "HACBS"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	var fw *framework.Framework
	var err error
	var kubeController tekton.KubeController

	var devNamespace, managedNamespace string
	var imageIDs []string
	var pyxisKeyDecoded, pyxisCertDecoded []byte

	BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace("e2e-pyxis"))
		fmt.Println("NewFramework:", fw)
		fmt.Println("NewFramework Error:", err)
		Expect(err).NotTo(HaveOccurred())

		kubeController = tekton.KubeController{
			Commonctrl: *fw.AsKubeAdmin.CommonController,
			Tektonctrl: *fw.AsKubeAdmin.TektonController,
		}

		devNamespace = fw.UserNamespace
		managedNamespace = utils.GetGeneratedNamespace("pyxis-managed")

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: ", err)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty())

		destinationAuthJson := utils.GetEnv("QUAY_OAUTH_TOKEN_RELEASE_DESTINATION", "")
		Expect(destinationAuthJson).ToNot(BeEmpty())

		keyPyxisStage := os.Getenv(constants.PYXIS_STAGE_KEY_ENV)
		Expect(keyPyxisStage).ToNot(BeEmpty())

		certPyxisStage := os.Getenv(constants.PYXIS_STAGE_CERT_ENV)
		Expect(certPyxisStage).ToNot(BeEmpty())

		// Create secret for the build registry repo "redhat-appstudio-qe".
		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(hacbsReleaseTestsTokenSecret, devNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred())

		// Create secret for the release registry repo "hacbs-release-tests".
		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(redhatAppstudioUserSecret, managedNamespace, destinationAuthJson)
		Expect(err).ToNot(HaveOccurred())

		// Linking the build secret to the pipline service account in dev namespace.
		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(devNamespace, hacbsReleaseTestsTokenSecret, serviceAccount, true)
		Expect(err).ToNot(HaveOccurred())

		publicKey, err := kubeController.GetTektonChainsPublicKey()
		Expect(err).ToNot(HaveOccurred())

		// Creating k8s secret to access Pyxis stage based on base64 decoded of key and cert
		rawDecodedTextStringData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyPyxisStage)))
		Expect(err).ToNot(HaveOccurred())
		rawDecodedTextData, err := base64.StdEncoding.DecodeString(string(certPyxisStage))
		Expect(err).ToNot(HaveOccurred())

		pyxisKeyDecoded = rawDecodedTextStringData
		pyxisCertDecoded = rawDecodedTextData

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pyxis",
				Namespace: managedNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"cert": rawDecodedTextData,
				"key":  rawDecodedTextStringData,
			},
		}

		_, err = fw.AsKubeAdmin.CommonController.CreateSecret(managedNamespace, secret)
		Expect(err).ToNot(HaveOccurred())

		Expect(kubeController.CreateOrUpdateSigningSecret(
			publicKey, publicSecretNameAuth, managedNamespace)).To(Succeed())

		defaultEcPolicy, err := kubeController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred())

		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   string(publicKey),
			Sources:     defaultEcPolicy.Spec.Sources,
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"minimal", "slsa2"},
				Exclude:     []string{"cve"},
			},
		}

		_, err = fw.AsKubeAdmin.CommonController.CreateServiceAccount(releaseStrategyServiceAccountDefault, managedNamespace, managednamespaceSecret)
		Expect(err).NotTo(HaveOccurred())

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, redhatAppstudioUserSecret, releaseStrategyServiceAccountDefault, true)
		Expect(err).ToNot(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy("mvp-push-to-external-registry-strategy", managedNamespace, "push-to-external-registry", "quay.io/hacbs-release/pipeline-push-to-external-registry:0.8", releaseStrategyPolicyDefault, releaseStrategyServiceAccountDefault, paramsReleaseStrategyPyxis)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationNameDefault, managedNamespace, "", "", "mvp-push-to-external-registry-strategy")
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicyDefault, managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateHasApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.HasController.CreateComponent(applicationNameDefault, componentName, devNamespace, gitSourceComponentUrl, "", containerImageUrl, "", "", true)
		Expect(err).NotTo(HaveOccurred())

		outputContainerImage := "quay.io/redhat-appstudio-qe/test-release-images"
		_, err = fw.AsKubeAdmin.HasController.CreateComponent(applicationNameDefault, "simple-python", devNamespace, "https://github.com/devfile-samples/devfile-sample-python-basic", "", "", outputContainerImage, "", false)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterAll(func() {

		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that a PipelineRun is created in dev namespace.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(devNamespace)
				componentsCounter := 0
				if err != nil || prList == nil || len(prList.Items) < 1 {
					return false
				}
				for _, pr := range prList.Items {
					if pr.Name == componentName || pr.Name == "simple-python" {
						componentsCounter++
					}
				}

				return componentsCounter < 2
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that the PipelineRun in dev namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(devNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue() &&
					prList.Items[1].HasStarted() && prList.Items[1].IsDone() && prList.Items[1].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies that a PipelineRun is created in managed namespace.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					return false
				}

				return strings.Contains(prList.Items[0].Name, "release")
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("verifies a PipelineRun started in managed namespace succeeded.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue() &&
					prList.Items[1].HasStarted() && prList.Items[1].IsDone() && prList.Items[1].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("validate the result of task create-pyxis-image contains two image ids.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					return false
				}

				for _, pr := range prList.Items {
					pr, err := kubeController.Tektonctrl.GetPipelineRun(pr.Name, managedNamespace)
					if err != nil {
						Expect(err).NotTo(HaveOccurred())
					}

					tr, err := kubeController.GetTaskRunStatus(fw.AsKubeAdmin.CommonController.KubeRest(), pr, "create-pyxis-image")
					if err != nil {
						Expect(err).NotTo(HaveOccurred())
					}

					re := regexp.MustCompile("[a-fA-F0-9]{24}")
					returnedImageID := re.FindAllString(tr.Status.TaskRunResults[0].Value.StringVal, -1)

					if len(returnedImageID) > len(imageIDs) {
						imageIDs = returnedImageID
					}
				}

				return len(imageIDs[0]) > 10 && len(imageIDs[1]) > 10
			}).Should(BeTrue())
		})

		It("tests a Release should have been created in the dev namespace and succeeded.", func() {
			Eventually(func() bool {
				releaseCreated, err := fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				if releaseCreated == nil || err != nil {
					return false
				}

				return releaseCreated.IsReleased()
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("validates that imageIds was pushed to pyxis are identical to those in task create-pyxis-image.", func() {
			Eventually(func() bool {
				isImageExist := 0
				for _, imageId := range imageIDs {

					url := fmt.Sprintf("https://pyxis.preprod.api.redhat.com/v1/images/id/%s", imageId)

					// Create a TLS configuration with the key and certificate
					cert, err := tls.X509KeyPair(pyxisCertDecoded, pyxisKeyDecoded)
					if err != nil {
						fmt.Println("Error creating TLS certificate and key:", err)
						Expect(err).NotTo(HaveOccurred())
					}

					// Create a client with the custom TLS configuration
					client := &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{
								Certificates: []tls.Certificate{cert},
							},
						},
					}

					// Send GET request
					request, err := http.NewRequest("GET", url, nil)
					if err != nil {
						fmt.Println("Error creating GET request:", err)
						Expect(err).NotTo(HaveOccurred())
					}

					response, err := client.Do(request)
					if err != nil {
						fmt.Println("Error sending GET request:", err)
						Expect(err).NotTo(HaveOccurred())
					}

					defer response.Body.Close()

					// Read the response body
					body, err := io.ReadAll(response.Body)
					if err != nil {
						fmt.Println("Error reading response body:", err)
						Expect(err).NotTo(HaveOccurred())
					}

					// Print the response
					if strings.Contains(string(body), imageId) {
						isImageExist++
					}

				}

				return isImageExist == len(imageIDs)
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})
	})
})
