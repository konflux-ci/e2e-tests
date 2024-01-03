package spi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
 * Component: spi
 * Description: SVPI-407 - Check ImagePullSecret usage for the private Quay image
                SVPI-408 - Check the secret that can be used with scopeo Tekton task to authorize a copy of one private Quay image to the second Quay image repository
 * Note: To avoid code repetition, SVPI-408 was integrated with SVPI-407

  * Flow of the test:
	* 1º - creates SPITokenBinding
	* 2º - uploads token
	* 3º - creates a Pod from a Private Quay image
	* 4º - checks the secret that can be used with scopeo Tekton task to authorize a copy of one private Quay image to the second Quay image repository
*/

var _ = framework.SPISuiteDescribe(Label("spi-suite", "quay-imagepullsecret-usage"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var QuayAuthToken string
	var QuayAuthUser string
	AfterEach(framework.ReportFailure(&fw))

	Describe("SVPI-407 - Check ImagePullSecret usage for the private Quay image", Ordered, func() {
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
			err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("token-upload-rest-endpoint", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())

			// Quay username and token are required by SPI to generate valid credentials
			QuayAuthToken = utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "")
			QuayAuthUser = utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "")
			Expect(QuayAuthToken).NotTo(BeEmpty())
			Expect(QuayAuthUser).NotTo(BeEmpty())
		})

		// Clean up after running these tests and before the next tests block: can't have multiple AccessTokens in Injected phase
		AfterAll(func() {
			// collect SPI ResourceQuota metrics (temporary)
			err := fw.AsKubeAdmin.CommonController.GetResourceQuotaInfo("quay-imagepullsecret-usage", namespace, "appstudio-crds-spi")
			Expect(err).NotTo(HaveOccurred())

			if !CurrentSpecReport().Failed() {
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.SPIController.DeleteAllAccessTokenDataInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.CommonController.DeleteAllServiceAccountsInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.TektonController.DeleteAllTasksInASpecificNamespace(namespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.TektonController.DeleteAllTaskRunsInASpecificNamespace(namespace)).To(Succeed())
			}
		})

		var SPITokenBinding *v1beta1.SPIAccessTokenBinding
		var SPIAccessToken *v1beta1.SPIAccessToken
		var QuaySPITokenBindingName = "quay-spi-token-binding"
		var SecretName = "test-secret-dockerconfigjson"
		var TestQuayPrivateRepoURL = fmt.Sprintf("%s:test", QuayPrivateRepoURL)

		It("creates SPITokenBinding", func() {
			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(QuaySPITokenBindingName, namespace, TestQuayPrivateRepoURL, SecretName, corev1.SecretTypeDockerConfigJson)
			Expect(err).NotTo(HaveOccurred())

			SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		// start of upload token
		It("SPITokenBinding to be in AwaitingTokenData phase", func() {
			// wait SPITokenBinding to be in AwaitingTokenData phase before trying to upload a token
			Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPITokenBinding.Status.Phase
			}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData))
		})

		It("uploads username and token using rest endpoint", func() {
			// the UploadUrl in SPITokenBinding should be available before uploading the token
			Eventually(func() string {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPITokenBinding.Status.UploadUrl
			}, 1*time.Minute, 10*time.Second).ShouldNot(BeEmpty(), fmt.Sprintf(".Status.UploadUrl for SPIAccessTokenBinding %s/%s is not set", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName()))
			Expect(err).NotTo(HaveOccurred())

			// linked accessToken token should exist
			linkedAccessTokenName := SPITokenBinding.Status.LinkedAccessTokenName
			Expect(linkedAccessTokenName).NotTo(BeEmpty())

			// get the url to manually upload the token
			uploadURL := SPITokenBinding.Status.UploadUrl

			// Get the token for the current openshift user
			bearerToken, err := utils.GetOpenshiftToken()
			Expect(err).NotTo(HaveOccurred())

			// build and upload the payload using the uploadURL. it should return 204
			oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.QUAY_OAUTH_TOKEN_ENV, "") + `", "username":"` + utils.GetEnv(constants.QUAY_OAUTH_USER_ENV, "") + `"}`
			statusCode, err := fw.AsKubeDeveloper.SPIController.UploadWithRestEndpoint(uploadURL, oauthCredentials, bearerToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).Should(Equal(204))
		})

		It("SPITokenBinding to be in Injected phase", func() {
			Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
				SPITokenBinding, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())
				return SPITokenBinding.Status.Phase
			}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseInjected), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenBindingPhaseInjected))
		})

		It("SPIAccessToken exists and is in Ready phase", func() {
			Eventually(func() (v1beta1.SPIAccessTokenPhase, error) {
				SPIAccessToken, err = fw.AsKubeDeveloper.SPIController.GetSPIAccessToken(SPITokenBinding.Status.LinkedAccessTokenName, namespace)
				if err != nil {
					return "", err
				}
				return SPIAccessToken.Status.Phase, nil
			}, 2*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenPhaseReady), "SPIAccessToken for SPITokenBinding %s/%s should be in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenPhaseReady)
		})

		// Create a pod using the generated ImagePullSecret to pull a private quay image
		It("creates a Pod from a Private Quay image", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "rtw"},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "quay-image",
							Image:           TestQuayPrivateRepoURL,
							ImagePullPolicy: corev1.PullAlways,
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: SPITokenBinding.Status.SyncedObjectRef.Name},
					},
				}}

			pod, err = fw.AsKubeAdmin.CommonController.KubeInterface().CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
			pod, err := fw.AsKubeAdmin.CommonController.GetPod(namespace, pod.Name)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() corev1.PodPhase {
				pod, err := fw.AsKubeAdmin.CommonController.GetPod(namespace, pod.Name)
				Expect(err).NotTo(HaveOccurred())

				return pod.Status.Phase
			}, 3*time.Minute, 5*time.Second).Should(Equal(corev1.PodRunning), fmt.Sprintf("Pod %s/%s did not have the status %s", pod.GetNamespace(), pod.GetName(), corev1.PodRunning))
		})

		Describe("SVPI-408 - Check the secret that can be used with skopeo Tekton task to authorize a copy of one private Quay image to the second Quay image repository", Ordered, func() {
			serviceAccountName := "tekton-task-service-account"

			It("creates service account for the TaskRun referencing the docker config json secret", func() {
				secrets := []corev1.ObjectReference{
					{Name: SecretName},
				}
				_, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(serviceAccountName, namespace, secrets, nil)
				Expect(err).NotTo(HaveOccurred())
				_, err = fw.AsKubeAdmin.CommonController.GetServiceAccount(serviceAccountName, namespace)
				Expect(err).NotTo(HaveOccurred())
			})

			/*
				ClusterTask is getting deprecated. See https://tekton.dev/docs/pipelines/tasks/#task-vs-clustertask and https://cloud.redhat.com/blog/migration-from-clustertasks-to-tekton-resolvers-in-openshift-pipelines.
				Based on discussions in slack, even though ClusterTasks are getting deprecated, for now, someone using OpenShift Pipelines should use the ClusterTasks anyway.
				Eventually, there will be a release that provides those as regular Tasks in a special namespace so cluster resolver can be used. At that point, we should switch over.

			*/
			var TaskRun *tektonv1.TaskRun
			taskRunName := "skopeo-run"

			It("creates taskrun", func() {
				srcImageURL := fmt.Sprintf("docker://%s", TestQuayPrivateRepoURL)
				destTag := fmt.Sprintf("spi-test-%s", strings.Replace(uuid.New().String(), "-", "", -1))
				destImageURL := fmt.Sprintf("docker://%s:%s", QuayPrivateRepoURL, destTag)

				TaskRun, err = fw.AsKubeAdmin.TektonController.CreateTaskRunCopy(taskRunName, namespace, serviceAccountName, srcImageURL, destImageURL)
				Expect(err).NotTo(HaveOccurred())
				TaskRun, err = fw.AsKubeDeveloper.TektonController.GetTaskRun(taskRunName, namespace)
				Expect(err).NotTo(HaveOccurred())
			})

			It("checks if taskrun is complete", func() {
				Eventually(func() *metav1.Time {
					TaskRun, err = fw.AsKubeDeveloper.TektonController.GetTaskRun(taskRunName, namespace)
					Expect(err).NotTo(HaveOccurred())

					return TaskRun.Status.CompletionTime
				}, 5*time.Minute, 5*time.Second).ShouldNot(BeNil(), "timed out waiting for taskrun %s/%s to get completed", TaskRun.GetNamespace(), TaskRun.GetName())
			})

			It("checks if taskrun is successful", func() {
				TaskRun, err = fw.AsKubeDeveloper.TektonController.GetTaskRun(taskRunName, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(TaskRun.Status.Conditions).NotTo(BeEmpty())
				Expect(TaskRun.Status.Conditions[0].Status).To(Equal(corev1.ConditionTrue))
			})
		})

	})
})
