package spi

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

	"github.com/avast/retry-go/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	client "github.com/redhat-appstudio/e2e-tests/pkg/clients/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type User struct {
	Framework                    *framework.Framework
	LinkedAccessTokenName        string
	SPIGithubWorkSpaceSecretName string
	WorkspaceURL                 string
	APIProxyClientA              *crclient.Client
	APIProxyClientB              *crclient.Client
	APIProxyClientC              *crclient.Client
}

/*
 * Component: spi
 * Description: SVPI-495 - Test automation to ensure that a user can't access and use secrets from another workspace

 * User A is the owner of workspace A and has access to workspace C as the maintainer
 * User B is the owner of workspace B
 * User C is the owner of workspace C

 * Test cases:
	* check if user can access the SPIAccessToken from another workspace
	* check if user can read the GitHub repo from another workspace
	* check if user can read the secret from another workspace
	* check if user's pod deployed in the user's workspace can read the GitHub repo from another workspace
*/

var _ = framework.SPISuiteDescribe(Label("spi-suite", "access-control"), func() {

	defer GinkgoRecover()

	var userA, userB, userC User

	Describe("SVPI-495 - Test automation to ensure that a user can't access and use secrets from another workspace", Ordered, func() {

		delete := func(user User) {
			Expect(user.Framework.AsKubeAdmin.SPIController.DeleteAllBindingTokensInASpecificNamespace(user.Framework.UserNamespace)).To(Succeed())
			Expect(user.Framework.AsKubeAdmin.SPIController.DeleteAllAccessTokensInASpecificNamespace(user.Framework.UserNamespace)).To(Succeed())
			Expect(user.Framework.AsKubeAdmin.SPIController.DeleteAllAccessTokenDataInASpecificNamespace(user.Framework.UserNamespace)).To(Succeed())
			Expect(user.Framework.SandboxController.DeleteUserSignup(user.Framework.UserName)).To(BeTrue())
		}

		createAPIProxyClient := func(userToken, proxyURL string) *crclient.Client {
			APIProxyClient, err := client.CreateAPIProxyClient(userToken, proxyURL)
			Expect(err).NotTo(HaveOccurred())
			client := APIProxyClient.KubeRest()
			return &client
		}

		createSPITokenBinding := func(user User) {
			namespace := user.Framework.UserNamespace
			secretName := user.SPIGithubWorkSpaceSecretName

			// creates SPITokenBinding
			SPITokenBinding, err := user.Framework.AsKubeDeveloper.SPIController.CreateSPIAccessTokenBinding(SPITokenBindingName, namespace, GithubPrivateRepoURL, secretName, "kubernetes.io/basic-auth")
			Expect(err).NotTo(HaveOccurred())

			// start of upload token
			// SPITokenBinding to be in AwaitingTokenData phase
			// wait SPITokenBinding to be in AwaitingTokenData phase before trying to upload a token
			Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
				SPITokenBinding, err = user.Framework.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPITokenBinding.Status.Phase
			}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenBindingPhaseAwaitingTokenData))

			// uploads username and token using rest endpoint
			// the UploadUrl in SPITokenBinding should be available before uploading the token
			Eventually(func() string {
				SPITokenBinding, err = user.Framework.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())

				return SPITokenBinding.Status.UploadUrl
			}, 1*time.Minute, 10*time.Second).ShouldNot(BeEmpty(), fmt.Sprintf(".Status.UploadUrl for SPIAccessTokenBinding %s/%s is not set", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName()))
			Expect(err).NotTo(HaveOccurred())

			// LinkedAccessToken should exist
			linkedAccessTokenName := SPITokenBinding.Status.LinkedAccessTokenName
			Expect(linkedAccessTokenName).NotTo(BeEmpty())

			// keep LinkedAccessToken name
			username := user.Framework.UserName
			if strings.HasPrefix(username, "spi-user-a") {
				userA.LinkedAccessTokenName = linkedAccessTokenName
			} else if strings.HasPrefix(username, "spi-user-b") {
				userB.LinkedAccessTokenName = linkedAccessTokenName
			} else {
				userC.LinkedAccessTokenName = linkedAccessTokenName
			}

			// get the url to manually upload the token
			uploadURL := SPITokenBinding.Status.UploadUrl

			// Get the token for the current openshift user
			bearerToken, err := utils.GetOpenshiftToken()
			Expect(err).NotTo(HaveOccurred())

			// build and upload the payload using the uploadURL. it should return 204
			oauthCredentials := `{"access_token":"` + utils.GetEnv(constants.GITHUB_TOKEN_ENV, "") + `"}`
			statusCode, err := user.Framework.AsKubeDeveloper.SPIController.UploadWithRestEndpoint(uploadURL, oauthCredentials, bearerToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).Should(Equal(204))

			// SPITokenBinding to be in Injected phase
			Eventually(func() v1beta1.SPIAccessTokenBindingPhase {
				SPITokenBinding, err = user.Framework.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(SPITokenBinding.Name, namespace)
				Expect(err).NotTo(HaveOccurred())
				return SPITokenBinding.Status.Phase
			}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenBindingPhaseInjected), fmt.Sprintf("SPIAccessTokenBinding %s/%s is not in %s phase", SPITokenBinding.GetNamespace(), SPITokenBinding.GetName(), v1beta1.SPIAccessTokenBindingPhaseInjected))

			// SPIAccessToken exists and is in Read phase
			Eventually(func() (v1beta1.SPIAccessTokenPhase, error) {
				SPIAccessToken, err := user.Framework.AsKubeDeveloper.SPIController.GetSPIAccessToken(SPITokenBinding.Status.LinkedAccessTokenName, namespace)
				if err != nil {
					return "", fmt.Errorf("can't get SPI access token %s/%s: %+v", namespace, SPITokenBinding.Status.LinkedAccessTokenName, err)
				}

				return SPIAccessToken.Status.Phase, nil
			}, 1*time.Minute, 5*time.Second).Should(Equal(v1beta1.SPIAccessTokenPhaseReady), fmt.Sprintf("SPIAccessToken %s/%s should be in ready phase", namespace, SPITokenBinding.Status.LinkedAccessTokenName))
			// end of upload token
		}

		// check if guestUser can access a primary's secret in the primary's workspace
		checkSecretAccess := func(client crclient.Client, primaryUser User, shouldAccess bool) {
			// checks that guest user can access a primary's secret in primary's workspace
			spiAccessToken := &v1beta1.SPIAccessToken{}
			err := client.Get(context.Background(), types.NamespacedName{Name: primaryUser.LinkedAccessTokenName, Namespace: primaryUser.Framework.UserNamespace}, spiAccessToken)

			if shouldAccess {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		}

		// check if guest user can read a primary's GitHub repo in the primary's workspace
		checkRepositoryReading := func(client crclient.Client, primaryUser User, shouldRead bool) {
			// create SPIFileContentRequest in primary's workspace URL
			spiFcr := v1beta1.SPIFileContentRequest{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "spi-file-content-request-",
					Namespace:    primaryUser.Framework.UserNamespace,
				},
				Spec: v1beta1.SPIFileContentRequestSpec{RepoUrl: GithubPrivateRepoURL, FilePath: GithubPrivateRepoFilePath},
			}
			err := client.Create(context.Background(), &spiFcr)

			if shouldRead {
				Expect(err).NotTo(HaveOccurred())

				// check that guest user can read a primary's GitHub repo in the primary's workspace
				Eventually(func() v1beta1.SPIFileContentRequestStatus {
					err = client.Get(context.Background(), types.NamespacedName{Name: spiFcr.Name, Namespace: spiFcr.Namespace}, &spiFcr)
					Expect(err).NotTo(HaveOccurred())

					return spiFcr.Status
				}, 2*time.Minute, 10*time.Second).Should(MatchFields(IgnoreExtras, Fields{
					"Content": Not(BeEmpty()),
					"Phase":   Equal(v1beta1.SPIFileContentRequestPhaseDelivered),
				}), fmt.Sprintf("content not provided by SPIFileContentRequest %s/%s", primaryUser.Framework.UserNamespace, spiFcr.Name))
			} else {
				Expect(err).To(HaveOccurred())
			}
		}

		// check if guest user can read a primary's secret in the primary's workspace
		checkSecretReading := func(client crclient.Client, primaryUser User, shouldRead bool) {
			resultSecret := &corev1.Secret{}
			err := client.Get(context.Background(), types.NamespacedName{Name: primaryUser.SPIGithubWorkSpaceSecretName, Namespace: primaryUser.Framework.UserNamespace}, resultSecret)

			if shouldRead {
				Expect(err).ToNot(HaveOccurred())
				token := resultSecret.Data["password"]
				Expect(token).ToNot(BeEmpty())
			} else {
				Expect(err).To(HaveOccurred())
			}
		}

		// check that guest user can make a request to create a SPIFileContentRequest in the primary's workspace
		makeGhReadRequestFromPod := func(guestUser, primaryUser User, podName, podNamespace, spiFcrName string, shouldAccess bool) {
			namespace := primaryUser.Framework.UserNamespace
			spiFcrData := fmt.Sprintf(`---
apiVersion: appstudio.redhat.com/v1beta1
kind: SPIFileContentRequest
metadata:
  name: %s
  namespace: %s
spec:
  filePath: README.md
  repoUrl: %s
`, spiFcrName, namespace, GithubPrivateRepoURL)
			readRequest := fmt.Sprintf(
				"curl '%s' \\"+
					"-k \\"+
					"-H 'Authorization: Bearer %s' \\"+
					"-X POST \\"+
					"-H 'Content-Type: application/yaml' \\"+
					"--connect-timeout 30 \\"+
					"--max-time 300 \\"+
					"-d '%s'",
				fmt.Sprintf("%s/apis/appstudio.redhat.com/v1beta1/namespaces/%s/spifilecontentrequests", primaryUser.WorkspaceURL, namespace),
				guestUser.Framework.UserToken,
				spiFcrData,
			)
			request := guestUser.Framework.AsKubeAdmin.CommonController.KubeInterface().CoreV1().RESTClient().
				Post().Namespace(podNamespace).
				Resource("pods").
				Name(podName).
				SubResource("exec").
				VersionedParams(&corev1.PodExecOptions{
					Command: []string{
						"/bin/sh",
						"-c",
						readRequest,
					},
					Stdin:  false,
					Stdout: true,
					Stderr: true,
					TTY:    true,
				}, scheme.ParameterCodec)

			config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				clientcmd.NewDefaultClientConfigLoadingRules(),
				&clientcmd.ConfigOverrides{},
			)
			restConfig, err := config.ClientConfig()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				stop := false
				exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", request.URL())
				Expect(err).NotTo(HaveOccurred())

				buffer := &bytes.Buffer{}
				errBuffer := &bytes.Buffer{}
				err = exec.Stream(remotecommand.StreamOptions{
					Stdout: buffer,
					Stderr: errBuffer,
				})
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("error: %v", err))
				Expect(errBuffer.String()).To(BeEmpty(), fmt.Sprintf("stderr: %v", errBuffer.String()))

				// default of attempts: 10
				err = retry.Do(
					func() error {
						_, err = primaryUser.Framework.AsKubeDeveloper.SPIController.GetSPIFileContentRequest(spiFcrName, namespace)
						if err != nil && shouldAccess {
							GinkgoWriter.Printf("buffer info: %s\n", buffer.String())
							return err
						}
						stop = true
						return nil
					},
				)
				return stop
			}, 5*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("SPIFileContentRequest '%s' in namespace '%s' was not created", spiFcrName, namespace))
		}

		// check if guest user's pod deployed in guest user's workspace should be able to construct an API request that reads code in the Github repo for primary's user workspace
		checkRepoReadingFromPod := func(client crclient.Client, guestUser, primaryUser User, shouldRead bool) {
			namespace := guestUser.Framework.UserNamespace
			p := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    namespace,
					GenerateName: "pod-",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "read",
							Image: "quay.io/redhat-appstudio/buildah:v1.28",
							// workaround to force the pod to be running
							Command: []string{
								"sleep",
								"600",
							},
						},
					},
				},
			}

			// create pod
			pod, err := guestUser.Framework.AsKubeAdmin.CommonController.KubeInterface().CoreV1().Pods(namespace).Create(context.Background(), p, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			// check that pod started
			Eventually(func() corev1.PodPhase {
				pod, err := guestUser.Framework.AsKubeAdmin.CommonController.GetPod(namespace, pod.Name)
				Expect(err).NotTo(HaveOccurred())

				return pod.Status.Phase
			}, 8*time.Minute, 5*time.Second).Should(Equal(corev1.PodRunning), fmt.Sprintf("Pod %s/%s not created successfully", namespace, pod.Name))
			Expect(err).NotTo(HaveOccurred())

			// make read request
			spiFcrName := utils.GetGeneratedNamespace("pod-spi-file-content-request")
			makeGhReadRequestFromPod(guestUser, primaryUser, pod.Name, pod.Namespace, spiFcrName, shouldRead)

			spiFcr := v1beta1.SPIFileContentRequest{}
			if shouldRead {
				// check that guest user's pod can read a primary's GitHub repo in the primary's workspace
				Eventually(func() v1beta1.SPIFileContentRequestStatus {
					err = client.Get(context.Background(), types.NamespacedName{Name: spiFcrName, Namespace: primaryUser.Framework.UserNamespace}, &spiFcr)
					Expect(err).NotTo(HaveOccurred())

					return spiFcr.Status
				}, 2*time.Minute, 10*time.Second).Should(MatchFields(IgnoreExtras, Fields{
					"Content": Not(BeEmpty()),
					"Phase":   Equal(v1beta1.SPIFileContentRequestPhaseDelivered),
				}), fmt.Sprintf("content not provided by SPIFileContentRequest %s/%s", primaryUser.Framework.UserNamespace, spiFcrName))
			} else {
				// check that guest user's pod can not read a primary's GitHub repo in the primary's workspace
				err = client.Get(context.Background(), types.NamespacedName{Name: spiFcrName, Namespace: primaryUser.Framework.UserNamespace}, &spiFcr)
				Expect(err).To(HaveOccurred())
			}
		}

		BeforeAll(func() {
			// Initialize the tests controllers for user A
			// test user A is the owner and have access to the workspace A
			fwUserA, err := framework.NewFramework(utils.GetGeneratedNamespace("spi-user-a"))
			Expect(err).NotTo(HaveOccurred())
			namespaceUserA := fwUserA.UserNamespace
			Expect(namespaceUserA).NotTo(BeEmpty())
			userA = User{
				Framework:                    fwUserA,
				SPIGithubWorkSpaceSecretName: "e2e-github-secret-workspace-a",
				WorkspaceURL:                 fmt.Sprintf("%s/workspaces/%s", fwUserA.ProxyUrl, fwUserA.UserName),
			}

			// Initialize the tests controllers for user B
			// test user B is the owner and have access to the workspace B
			fwUserB, err := framework.NewFramework(utils.GetGeneratedNamespace("spi-user-b"))
			Expect(err).NotTo(HaveOccurred())
			namespaceUserB := fwUserB.UserNamespace
			Expect(namespaceUserB).NotTo(BeEmpty())
			userB = User{
				Framework:                    fwUserB,
				SPIGithubWorkSpaceSecretName: "e2e-github-secret-workspace-b",
				WorkspaceURL:                 fmt.Sprintf("%s/workspaces/%s", fwUserB.ProxyUrl, fwUserB.UserName),
			}

			// Initialize the tests controllers for user C
			// test user C is the owner and have access to the workspace C
			fwUserC, err := framework.NewFramework(utils.GetGeneratedNamespace("spi-user-c"))
			Expect(err).NotTo(HaveOccurred())
			namespaceUserC := fwUserC.UserNamespace
			Expect(namespaceUserC).NotTo(BeEmpty())
			userC = User{
				Framework:                    fwUserC,
				SPIGithubWorkSpaceSecretName: "e2e-github-secret-workspace-c",
				WorkspaceURL:                 fmt.Sprintf("%s/workspaces/%s", fwUserC.ProxyUrl, fwUserC.UserName),
			}

			// create api proxy client with user A token and user's A workspace URL
			userA.APIProxyClientA = createAPIProxyClient(userA.Framework.UserToken, userA.WorkspaceURL)
			// create api proxy client with user A token and user's B workspace URL
			userA.APIProxyClientB = createAPIProxyClient(userA.Framework.UserToken, userB.WorkspaceURL)
			// create api proxy client with user A token and user's C workspace URL
			userA.APIProxyClientC = createAPIProxyClient(userA.Framework.UserToken, userC.WorkspaceURL)

			// create api proxy client with user B token and user's A workspace URL
			userB.APIProxyClientA = createAPIProxyClient(userB.Framework.UserToken, userA.WorkspaceURL)
			// create api proxy client with user B token and user's B workspace URL
			userB.APIProxyClientB = createAPIProxyClient(userB.Framework.UserToken, userB.WorkspaceURL)
			// create api proxy client with user B token and user's C workspace URL
			userB.APIProxyClientC = createAPIProxyClient(userB.Framework.UserToken, userC.WorkspaceURL)

			// create api proxy client with user C token and user's A workspace URL
			userC.APIProxyClientA = createAPIProxyClient(userC.Framework.UserToken, userA.WorkspaceURL)
			// create api proxy client with user C token and user's B workspace URL
			userC.APIProxyClientB = createAPIProxyClient(userC.Framework.UserToken, userB.WorkspaceURL)
			// create api proxy client with user C token and user's C workspace URL
			userC.APIProxyClientC = createAPIProxyClient(userC.Framework.UserToken, userC.WorkspaceURL)

			// share workspace C with user A with maintainer roles
			_, err = fwUserC.AsKubeAdmin.CommonController.CreateSpaceBinding(fwUserA.UserName, fwUserC.UserName, "maintainer")
			Expect(err).NotTo(HaveOccurred())

			// check if workspace C was shared with user A
			Eventually(func() error {
				err = fwUserC.AsKubeAdmin.CommonController.CheckWorkspaceShare(fwUserA.UserName, fwUserC.UserNamespace)
				return err
			}, 1*time.Minute, 5*time.Second).Should(BeNil(), fmt.Sprintf("error checking if the workspace C (%s) was shared with user A (%s)", fwUserC.UserNamespace, fwUserA.UserName))

			// check if user C is provisioned after workspace share
			Eventually(func() error {
				_, err := fwUserC.SandboxController.GetUserProvisionedNamespace(fwUserC.UserName)
				return err
			}, 1*time.Minute, 5*time.Second).Should(BeNil(), fmt.Sprintf("error getting provisioned usernamespace for user %s", fwUserC.UserName))

			// create SPITokenBinding for user A
			createSPITokenBinding(userA)

			// create SPITokenBinding for user B
			createSPITokenBinding(userB)

			// create SPITokenBinding for user C
			createSPITokenBinding(userC)
		})

		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				delete(userA)
				delete(userB)
				delete(userC)
			}
		})

		It("checks that user A can access the SPIAccessToken A in workspace A", func() {
			checkSecretAccess(*userA.APIProxyClientA, userA, true)
		})

		It("checks that user A can not access the SPIAccessToken B in workspace B", func() {
			checkSecretAccess(*userA.APIProxyClientB, userB, false)
		})

		It("checks that user A can access the SPIAccessToken C in workspace C", func() {
			// workspace C was shared with user A
			checkSecretAccess(*userA.APIProxyClientC, userC, true)
		})

		It("checks that user B can not access the SPIAccessToken A in workspace A", func() {
			checkSecretAccess(*userB.APIProxyClientA, userA, false)
		})

		It("checks that user B can access the SPIAccessToken B in workspace B", func() {
			checkSecretAccess(*userB.APIProxyClientB, userB, true)
		})

		It("checks that user B can not access the SPIAccessToken C in workspace C", func() {
			checkSecretAccess(*userB.APIProxyClientC, userC, false)
		})

		It("checks that user C can not access the SPIAccessToken A in workspace A", func() {
			checkSecretAccess(*userC.APIProxyClientA, userA, false)
		})

		It("checks that user C can not access the SPIAccessToken B in workspace B", func() {
			checkSecretAccess(*userC.APIProxyClientB, userB, false)
		})

		It("checks that user C can access the SPIAccessToken C in workspace C", func() {
			checkSecretAccess(*userC.APIProxyClientC, userC, true)
		})

		It("checks that user A can read the GitHub repo in workspace A", func() {
			checkRepositoryReading(*userA.APIProxyClientA, userA, true)
		})

		It("checks that user A can not read the GitHub repo in workspace B", func() {
			checkRepositoryReading(*userA.APIProxyClientB, userB, false)
		})

		It("checks that user A can read the GitHub repo in workspace C", func() {
			// workspace C was shared with user A
			checkRepositoryReading(*userA.APIProxyClientC, userC, true)
		})

		It("checks that user B can not read the GitHub repo in workspace A", func() {
			checkRepositoryReading(*userB.APIProxyClientA, userA, false)
		})

		It("checks that user B can read the GitHub repo in workspace B", func() {
			checkRepositoryReading(*userB.APIProxyClientB, userB, true)
		})

		It("checks that user B can not read the GitHub repo in workspace C", func() {
			checkRepositoryReading(*userB.APIProxyClientC, userC, false)
		})

		It("checks that user C can not read the GitHub repo in workspace A", func() {
			checkRepositoryReading(*userC.APIProxyClientA, userA, false)
		})

		It("checks that user C can not read the GitHub repo in workspace B", func() {
			checkRepositoryReading(*userC.APIProxyClientB, userB, false)
		})

		It("checks that user C can read the GitHub repo in workspace C", func() {
			checkRepositoryReading(*userC.APIProxyClientC, userC, true)
		})

		It("checks that user A can read the secret in workspace A", func() {
			checkSecretReading(*userA.APIProxyClientA, userA, true)
		})

		It("checks that user A can not read the secret in workspace B", func() {
			checkSecretReading(*userA.APIProxyClientB, userB, false)
		})

		It("checks that user A can not read the secret in workspace C", func() {
			// although workspace C is shared with user A, the role given is maintainer,
			// which does not have any permissions for secrets object
			checkSecretReading(*userA.APIProxyClientC, userC, false)
		})

		It("checks that user B can not read the secret in workspace A", func() {
			checkSecretReading(*userB.APIProxyClientA, userA, false)
		})

		It("checks that user B can read the secret in workspace B", func() {
			checkSecretReading(*userB.APIProxyClientB, userB, true)
		})

		It("checks that user B can not read the secret in workspace C", func() {
			checkSecretReading(*userB.APIProxyClientC, userC, false)
		})

		It("checks that user C can not read the secret in workspace A", func() {
			checkSecretReading(*userC.APIProxyClientA, userA, false)
		})

		It("checks that user C can not read the secret in workspace B", func() {
			checkSecretReading(*userC.APIProxyClientB, userB, false)
		})

		It("checks that user C can read the secret in workspace C", func() {
			checkSecretReading(*userC.APIProxyClientC, userC, true)
		})

		It("checks that a user's A pod deployed in workspace A should be able to construct an API request that reads code in the Github repo for workspace A", func() {
			checkRepoReadingFromPod(*userA.APIProxyClientA, userA, userA, true)
		})

		It("checks that a user's A pod deployed in workspace A should not be able to construct an API request that reads code in the Github repo for workspace B", func() {
			checkRepoReadingFromPod(*userA.APIProxyClientB, userA, userB, false)
		})

		It("checks that a user's A pod deployed in workspace A should be able to construct an API request that reads code in the Github repo for workspace C", func() {
			// workspace C was shared with user A
			checkRepoReadingFromPod(*userA.APIProxyClientC, userA, userC, true)
		})

		It("checks that a user's B pod deployed in workspace B should not be able to construct an API request that reads code in the Github repo for workspace A", func() {
			checkRepoReadingFromPod(*userB.APIProxyClientA, userB, userA, false)
		})

		It("checks that a user's B pod deployed in workspace B should be able to construct an API request that reads code in the Github repo for workspace B", func() {
			checkRepoReadingFromPod(*userB.APIProxyClientB, userB, userB, true)
		})

		It("checks that a user's B pod deployed in workspace B should not be able to construct an API request that reads code in the Github repo for workspace C", func() {
			checkRepoReadingFromPod(*userB.APIProxyClientC, userB, userC, false)
		})

		It("checks that a user's C pod deployed in workspace C should not be able to construct an API request that reads code in the Github repo for workspace A", func() {
			checkRepoReadingFromPod(*userC.APIProxyClientA, userC, userA, false)
		})

		It("checks that a user's C pod deployed in workspace C should not be able to construct an API request that reads code in the Github repo for workspace B", func() {
			checkRepoReadingFromPod(*userC.APIProxyClientB, userC, userB, false)
		})

		It("checks that a user's C pod deployed in workspace C should be able to construct an API request that reads code in the Github repo for workspace C", func() {
			checkRepoReadingFromPod(*userC.APIProxyClientC, userC, userC, true)
		})
	})
})
