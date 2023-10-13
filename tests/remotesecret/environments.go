package remotesecret

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ = framework.SPISuiteDescribe(Label("rs-environment"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var targetNamespace string
	var targetNamespace_2 string
	var cfg *rest.Config
	const ManagedEnvironmentSecretName = "envsecret"
	const applicationName = "rsenvapp"
	const componentName = "rsenvcomp"
	const targetSecretName = "target-secret-test"
	const remoteSecretName = "target-secret-test"
	const targetSecretName_2 = "target-secret-test-2"
	const remoteSecretName_2 = "target-secret-test-2"

	const cdqName = "target-secret-test"
	const gitSourceRepo = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"

	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	compDetected := appservice.ComponentDetectionDescription{}
	componentObj := &appservice.Component{}

	Describe("(SVPI-632) RemoteSecret has to be created with target namespace and Environment and (SVPI-633) in all Environments of certain component and application, ", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("spi-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			targetNamespace = fw.UserNamespace + "-target"
			targetNamespace_2 = fw.UserNamespace + "-target-2"
			Expect(namespace).NotTo(BeEmpty())
		})

		// Remove all resources created by the tests in case the suite was successfull
		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				if err := fw.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(fw.UserNamespace, 60*time.Second); err != nil {
					if err := fw.AsKubeAdmin.StoreAllArtifactsForNamespace(fw.UserNamespace); err != nil {
						Fail(fmt.Sprintf("error archiving artifacts:\n%s", err))
					}
					Fail(fmt.Sprintf("error deleting all componentns in namespace:\n%s", err))
				}
				Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
				Expect(fw.AsKubeAdmin.CommonController.DeleteAllSnapshotEnvBindingsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
				Expect(fw.AsKubeAdmin.IntegrationController.DeleteAllSnapshotsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
				Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
				Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(fw.UserNamespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllGitOpsDeploymentsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())
				Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())

				Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(targetNamespace)).To(Succeed())
				Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(targetNamespace_2)).To(Succeed())

			}
		})

		It("create target namespaces", func() {
			_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(targetNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(targetNamespace_2)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create managed-gitops.redhat.com/managed-environment secret type", func() {
			// get Kubeconfig of running cluster; it will be used as target for creating the test Environments
			cfg, err = config.GetConfig()
			Expect(err).NotTo(HaveOccurred())
			kubeConfData, err := utils.CreateKubeconfigFileForRestConfig(*cfg)
			Expect(err).NotTo(HaveOccurred())

			data := make(map[string][]byte)
			data["kubeconfig"] = []byte(kubeConfData)

			// create the secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ManagedEnvironmentSecretName,
					Namespace: fw.UserNamespace,
				},
				Type: corev1.SecretType("managed-gitops.redhat.com/managed-environment"),
				Data: data,
			}

			_, err = fw.AsKubeAdmin.CommonController.CreateSecret(fw.UserNamespace, secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create environments", func() {
			// Deletes current development environment
			Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 1*time.Minute)).To(Succeed())

			// Creates two environments using the same cluster as target
			ephemeralEnvironmentObj := &appservice.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetNamespace,
					Namespace: fw.UserNamespace,
				},
				Spec: appservice.EnvironmentSpec{
					DeploymentStrategy: appservice.DeploymentStrategy_AppStudioAutomated,
					DisplayName:        targetNamespace,
					Tags:               []string{"managed"},
					UnstableConfigurationFields: &appservice.UnstableEnvironmentConfiguration{
						ClusterType: appservice.ConfigurationClusterType_OpenShift,
						KubernetesClusterCredentials: appservice.KubernetesClusterCredentials{
							TargetNamespace:            targetNamespace,
							APIURL:                     cfg.Host,
							ClusterCredentialsSecret:   ManagedEnvironmentSecretName,
							AllowInsecureSkipTLSVerify: true,
							Namespaces:                 []string{targetNamespace},
						},
					},
				},
			}
			err := fw.AsKubeAdmin.RemoteSecretController.KubeRest().Create(context.TODO(), ephemeralEnvironmentObj)
			Expect(err).NotTo(HaveOccurred())

			ephemeralEnvironmentObj = &appservice.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetNamespace_2,
					Namespace: fw.UserNamespace,
				},
				Spec: appservice.EnvironmentSpec{
					DeploymentStrategy: appservice.DeploymentStrategy_AppStudioAutomated,
					DisplayName:        targetNamespace_2,
					Tags:               []string{"managed"},
					UnstableConfigurationFields: &appservice.UnstableEnvironmentConfiguration{
						ClusterType: appservice.ConfigurationClusterType_OpenShift,
						KubernetesClusterCredentials: appservice.KubernetesClusterCredentials{
							TargetNamespace:            targetNamespace_2,
							APIURL:                     cfg.Host,
							ClusterCredentialsSecret:   ManagedEnvironmentSecretName,
							AllowInsecureSkipTLSVerify: true,
							Namespaces:                 []string{targetNamespace_2},
						},
					},
				},
			}
			err = fw.AsKubeAdmin.RemoteSecretController.KubeRest().Create(context.TODO(), ephemeralEnvironmentObj)
			Expect(err).NotTo(HaveOccurred())

		})

		It("(SVPI-633) create remote secret #1 with only application and component and injects data", func() {
			labels := map[string]string{
				"appstudio.redhat.com/application": applicationName,
				"appstudio.redhat.com/component":   componentName,
			}
			_, err := fw.AsKubeAdmin.RemoteSecretController.CreateRemoteSecretWithLabels(remoteSecretName, fw.UserNamespace, targetSecretName, labels)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName, namespace)
				if k8sErrors.IsNotFound(err) {
					return false
				}

				return remoteSecret.Name == remoteSecretName
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s data not injected", namespace, remoteSecretName))

			fakeData := map[string]string{"username": "john", "password": "doe"}

			_, err = fw.AsKubeAdmin.RemoteSecretController.CreateUploadSecret(remoteSecretName, namespace, remoteSecretName, v1.SecretTypeOpaque, fakeData)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(remoteSecret.Status.Conditions, "DataObtained")
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s data not injected", namespace, remoteSecretName))

		})

		It("(SVPI-632) create remote secret #2 with application,component and environment and injects data", func() {
			labels := map[string]string{
				"appstudio.redhat.com/application": applicationName,
				"appstudio.redhat.com/environment": targetNamespace_2,
				"appstudio.redhat.com/component":   componentName,
			}
			_, err := fw.AsKubeAdmin.RemoteSecretController.CreateRemoteSecretWithLabels(remoteSecretName_2, fw.UserNamespace, targetSecretName_2, labels)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName_2, namespace)
				if k8sErrors.IsNotFound(err) {
					return false
				}

				return remoteSecret.Name == remoteSecretName_2
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s data not injected", namespace, remoteSecretName_2))

			fakeData := map[string]string{"username": "john", "password": "doe"}

			_, err = fw.AsKubeAdmin.RemoteSecretController.CreateUploadSecret(remoteSecretName_2, namespace, remoteSecretName_2, v1.SecretTypeOpaque, fakeData)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName_2, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(remoteSecret.Status.Conditions, "DataObtained")
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s data not injected", namespace, remoteSecretName_2))

		})

		It("create RHTAP application and check its health", func() {
			createdApplication, err := fw.AsKubeDeveloper.HasController.CreateApplication(applicationName, fw.UserNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdApplication.Spec.DisplayName).To(Equal(applicationName))
			Expect(createdApplication.Namespace).To(Equal(fw.UserNamespace))

			Eventually(func() string {
				application, err = fw.AsKubeAdmin.HasController.GetApplication(applicationName, fw.UserNamespace)
				Expect(err).NotTo(HaveOccurred())

				return application.Status.Devfile
			}, 3*time.Minute, 100*time.Millisecond).Should(Not(BeEmpty()), fmt.Sprintf("timed out waiting for gitOps repository to be created for the %s application in %s namespace", applicationName, fw.UserNamespace))

			Eventually(func() bool {
				// application info should be stored even after deleting the application in application variable
				gitOpsRepository := utils.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return fw.AsKubeAdmin.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("timed out waiting for HAS controller to create gitops repository for the %s application in %s namespace", applicationName, fw.UserNamespace))
		})

		It("create Red Hat AppStudio ComponentDetectionQuery for Component repository", func() {
			_, err := fw.AsKubeAdmin.HasController.CreateComponentDetectionQuery(cdqName, fw.UserNamespace, gitSourceRepo, "", "", "", false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("validate CDQ object information", func() {
			// Validate that the CDQ completes successfully
			Eventually(func() (appservice.ComponentDetectionMap, error) {
				// application info should be stored even after deleting the application in application variable
				cdq, err = fw.AsKubeAdmin.HasController.GetComponentDetectionQuery(cdqName, fw.UserNamespace)
				if err != nil {
					return nil, err
				}
				return cdq.Status.ComponentDetected, nil
				// Validate that the completed CDQ only has one detected component
			}, 1*time.Minute, 1*time.Second).Should(HaveLen(1), fmt.Sprintf("ComponentDetectionQuery %s/%s does not have the expected amount of components", fw.UserNamespace, "rscdqname"))

			// Get the stub CDQ and validate its content
			for _, compDetected = range cdq.Status.ComponentDetected {
				Expect(compDetected.DevfileFound).To(BeTrue(), "DevfileFound was not set to true")
				Expect(compDetected.Language).To(Equal("Java"), "Detected language was not java")
				Expect(compDetected.ProjectType).To(Equal("Quarkus"), "Detected framework was not quarkus")
			}
		})

		It("create Red Hat AppStudio Quarkus component", func() {
			compDetected.ComponentStub.ComponentName = componentName
			componentObj, err = fw.AsKubeAdmin.HasController.CreateComponent(compDetected.ComponentStub, fw.UserNamespace, "", "", applicationName, true, map[string]string{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("wait for component pipeline to finish", func() {
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(componentObj, "", 2, fw.AsKubeAdmin.TektonController)).To(Succeed())
		})

		It("(SVPI-633) secret #1 should exist in all environments' target namespace", func() {
			Eventually(func() bool {
				rs_1, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace, targetSecretName)
				if err != nil {
					return false
				}

				rs_2, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName)
				if err != nil {
					return false
				}

				return rs_1.Name == targetSecretName && rs_2.Name == targetSecretName
			}, 2*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("secrets %s is not created in all environments", targetSecretName))

		})

		It("(SVPI-632) secret #2 should exist (only) in the target environment", func() {
			Eventually(func() bool {
				targetSecret_2, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName_2)
				if k8sErrors.IsNotFound(err) {
					return false
				}

				_, errNotFound := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace, targetSecretName_2)

				return targetSecret_2.Name == targetSecretName_2 && k8sErrors.IsNotFound(errNotFound)
			}, 2*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("secret %s is not created in the specified environment", targetSecretName_2))

		})

		It("secrets #1 and #2 should be deleted when Environment is deleted", func() {
			// Delete the existing Environments
			Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())

			Eventually(func() bool {
				// Secrets should not exist anymore in target namespaces
				_, errRs1Ns1 := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace, targetSecretName)
				_, errRs1Ns2 := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName)
				_, errRs2Ns2 := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName_2)

				return k8sErrors.IsNotFound(errRs1Ns1) && k8sErrors.IsNotFound(errRs1Ns2) && k8sErrors.IsNotFound(errRs2Ns2)
			}, 2*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("secrets %s is not created in all environments", targetSecretName))

		})

	})
})
