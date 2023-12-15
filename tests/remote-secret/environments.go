package remotesecret

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/has"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/gitops"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var _ = framework.RemoteSecretSuiteDescribe(Label("remote-secret", "rs-environment"), func() {

	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var namespace string
	var targetNamespace string
	var targetNamespace_2 string
	var targetNamespace_3 string
	var cfg *rest.Config
	const ManagedEnvironmentSecretName = "envsecret"
	const applicationName = "rsenvapp"
	const componentName = "rsenvcomp"
	const targetSecretName = "target-secret-test"
	const remoteSecretName = "target-secret-test"
	const targetSecretName_2 = "target-secret-test-2"
	const remoteSecretName_2 = "target-secret-test-2"
	const targetSecretName_3 = "target-secret-test-3"
	const remoteSecretName_3 = "target-secret-test-3"

	const cdqName = "target-secret-test"
	const gitSourceRepo = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"

	application := &appservice.Application{}
	cdq := &appservice.ComponentDetectionQuery{}
	compDetected := appservice.ComponentDetectionDescription{}
	componentObj := &appservice.Component{}

	Describe("Check RemoteSecret behavior when deployed in Environments of an Application and Component", Ordered, func() {
		BeforeAll(func() {
			// Initialize the tests controllers
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace("rs-demos"))
			Expect(err).NotTo(HaveOccurred())
			namespace = fw.UserNamespace
			targetNamespace = fw.UserNamespace + "-target"
			targetNamespace_2 = fw.UserNamespace + "-target-2"
			targetNamespace_3 = fw.UserNamespace + "-target-3"
			Expect(namespace).NotTo(BeEmpty())
		})

		// Remove all resources created by the tests in case the suite was successful
		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				// RHTAPBUGS-978: temporary timeout to 15min
				if err := fw.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(fw.UserNamespace, 15*time.Minute); err != nil {
					if err := fw.AsKubeAdmin.StoreAllArtifactsForNamespace(fw.UserNamespace); err != nil {
						Fail(fmt.Sprintf("error archiving artifacts:\n%s", err))
					}
					Fail(fmt.Sprintf("error deleting all components in namespace:\n%s", err))
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
				Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(targetNamespace_3)).To(Succeed())

			}
		})

		It("create target namespaces", func() {
			_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(targetNamespace)
			Expect(err).NotTo(HaveOccurred())

			_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(targetNamespace_2)
			Expect(err).NotTo(HaveOccurred())

			_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(targetNamespace_3)
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
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ManagedEnvironmentSecretName,
					Namespace: fw.UserNamespace,
				},
				Type: v1.SecretType("managed-gitops.redhat.com/managed-environment"),
				Data: data,
			}

			_, err = fw.AsKubeAdmin.CommonController.CreateSecret(fw.UserNamespace, secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create three Environments", func() {
			// Deletes current development environment
			Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 1*time.Minute)).To(Succeed())

			// Creates three environments using the same cluster as target
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
			err := fw.AsKubeAdmin.RemoteSecretController.KubeRest().Create(context.Background(), ephemeralEnvironmentObj)
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
			err = fw.AsKubeAdmin.RemoteSecretController.KubeRest().Create(context.Background(), ephemeralEnvironmentObj)
			Expect(err).NotTo(HaveOccurred())

			ephemeralEnvironmentObj = &appservice.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetNamespace_3,
					Namespace: fw.UserNamespace,
				},
				Spec: appservice.EnvironmentSpec{
					DeploymentStrategy: appservice.DeploymentStrategy_AppStudioAutomated,
					DisplayName:        targetNamespace_3,
					Tags:               []string{"managed"},
					UnstableConfigurationFields: &appservice.UnstableEnvironmentConfiguration{
						ClusterType: appservice.ConfigurationClusterType_OpenShift,
						KubernetesClusterCredentials: appservice.KubernetesClusterCredentials{
							TargetNamespace:            targetNamespace_3,
							APIURL:                     cfg.Host,
							ClusterCredentialsSecret:   ManagedEnvironmentSecretName,
							AllowInsecureSkipTLSVerify: true,
							Namespaces:                 []string{targetNamespace_3},
						},
					},
				},
			}
			err = fw.AsKubeAdmin.RemoteSecretController.KubeRest().Create(context.Background(), ephemeralEnvironmentObj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("(SVPI-633) create Remote Secret #1 with only Application and Component and inject data", func() {
			labels := map[string]string{
				"appstudio.redhat.com/application": applicationName,
				"appstudio.redhat.com/component":   componentName,
			}

			annotations := map[string]string{}

			_, err := fw.AsKubeAdmin.RemoteSecretController.CreateRemoteSecretWithLabelsAndAnnotations(remoteSecretName, fw.UserNamespace, targetSecretName, labels, annotations)
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

		It("(SVPI-632) create Remote Secret #2 with Application, Component and one Environment (#2) and injects data", func() {
			labels := map[string]string{
				"appstudio.redhat.com/application": applicationName,
				"appstudio.redhat.com/environment": targetNamespace_2,
				"appstudio.redhat.com/component":   componentName,
			}
			annotations := map[string]string{}

			_, err := fw.AsKubeAdmin.RemoteSecretController.CreateRemoteSecretWithLabelsAndAnnotations(remoteSecretName_2, fw.UserNamespace, targetSecretName_2, labels, annotations)
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

		It("(SVPI-654) create Remote Secret #3 with Application, Component and multiple Environments (#2, #3) and injects data", func() {
			labels := map[string]string{
				"appstudio.redhat.com/application": applicationName,
				"appstudio.redhat.com/component":   componentName,
			}
			annotations := map[string]string{
				"appstudio.redhat.com/environment": fmt.Sprintf("%s,%s", targetNamespace_2, targetNamespace_3),
			}
			_, err := fw.AsKubeAdmin.RemoteSecretController.CreateRemoteSecretWithLabelsAndAnnotations(remoteSecretName_3, fw.UserNamespace, targetSecretName_3, labels, annotations)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName_3, namespace)
				if k8sErrors.IsNotFound(err) {
					return false
				}

				return remoteSecret.Name == remoteSecretName_3
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s data not injected", namespace, remoteSecretName_3))

			fakeData := map[string]string{"username": "john", "password": "doe"}

			_, err = fw.AsKubeAdmin.RemoteSecretController.CreateUploadSecret(remoteSecretName_3, namespace, remoteSecretName_3, v1.SecretTypeOpaque, fakeData)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName_3, namespace)
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(remoteSecret.Status.Conditions, "DataObtained")
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), fmt.Sprintf("RemoteSecret %s/%s data not injected", namespace, remoteSecretName_3))

		})

		It("create the RHTAP Application and check its health", func() {
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
				gitOpsRepository := gitops.ObtainGitOpsRepositoryName(application.Status.Devfile)

				return fw.AsKubeAdmin.CommonController.Github.CheckIfRepositoryExist(gitOpsRepository)
			}, 1*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("timed out waiting for HAS controller to create gitops repository for the %s application in %s namespace", applicationName, fw.UserNamespace))
		})

		It("create the RHTAP ComponentDetectionQuery for Component repository", func() {
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

		It("create RHTAP Quarkus component", func() {
			compDetected.ComponentStub.ComponentName = componentName
			componentObj, err = fw.AsKubeAdmin.HasController.CreateComponent(compDetected.ComponentStub, fw.UserNamespace, "", "", applicationName, true, map[string]string{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("wait for Component pipeline to finish", func() {
			Expect(fw.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(componentObj, "",
				fw.AsKubeAdmin.TektonController, &has.RetryOptions{Retries: 2, Always: true})).To(Succeed())
		})

		It("(SVPI-633) secret #1 should exist in all environments' target namespace", func() {
			// Checking that secret #1 (targetSecretName) exists in all three target namespaces (targetNamespace, targetNamespace_2, targetNamespace_3)
			Eventually(func() bool {
				rs_1, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace, targetSecretName)
				if err != nil {
					return false
				}

				rs_2, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName)
				if err != nil {
					return false
				}

				rs_3, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_3, targetSecretName)
				if err != nil {
					return false
				}

				return rs_1.Name == targetSecretName && rs_2.Name == targetSecretName && rs_3.Name == targetSecretName
			}, 2*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("secrets %s is not created in all environments", targetSecretName))

		})

		It("(SVPI-632) secret #2 should exist (only) in the target environment", func() {
			// Checking that secret #2 (targetSecretName_2) exists only target namespaces #2 (targetNamespace_2)
			Eventually(func() bool {
				targetSecret_2, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName_2)
				if k8sErrors.IsNotFound(err) {
					return false
				}

				_, errNotFound := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace, targetSecretName_2)

				_, errNotFound_3 := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_3, targetSecretName_2)

				return targetSecret_2.Name == targetSecretName_2 && k8sErrors.IsNotFound(errNotFound) && k8sErrors.IsNotFound(errNotFound_3)
			}, 2*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("secret %s does not exists (only) in the specified environment", targetSecretName_2))

		})

		It("(SVPI-654) secret #3 should exist (only) in #2 and #3 environments target namespace", func() {
			Eventually(func() bool {
				rs_3, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_3, targetSecretName_3)
				if err != nil {
					return false
				}

				rs_2, err := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName_3)
				if err != nil {
					return false
				}

				_, errNotFound := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace, targetSecretName_3)

				return rs_3.Name == targetSecretName_3 && rs_2.Name == targetSecretName_3 && k8sErrors.IsNotFound(errNotFound)
			}, 2*time.Minute, 1*time.Second).Should(BeTrue(), fmt.Sprintf("secrets %s is not created in all environments", targetSecretName_3))

		})

		It("check targets in RemoteSecret #1 status contains target namespace #1, #2, #3", func() {
			remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName, namespace)
			Expect(err).NotTo(HaveOccurred())

			// remote secret #1 is deployed in all environment for the application and component
			// target should therefore contain target namespaces of all three environments
			Expect(fw.AsKubeAdmin.RemoteSecretController.RemoteSecretTargetsContainsNamespace(targetNamespace, remoteSecret)).To(BeTrue(), fmt.Sprintf("namespace %s is not in targets of %s", targetNamespace, remoteSecretName))
			Expect(fw.AsKubeAdmin.RemoteSecretController.RemoteSecretTargetsContainsNamespace(targetNamespace_2, remoteSecret)).To(BeTrue(), fmt.Sprintf("namespace %s is not in targets of %s", targetNamespace_2, remoteSecretName))
			Expect(fw.AsKubeAdmin.RemoteSecretController.RemoteSecretTargetsContainsNamespace(targetNamespace_3, remoteSecret)).To(BeTrue(), fmt.Sprintf("namespace %s is not in targets of %s", targetNamespace_3, remoteSecretName))
			Expect(remoteSecret.Status.Targets).To(HaveLen(3))
		})

		It("checks targets in RemoteSecret #2 status contains (only) target namespace #2", func() {
			remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName_2, namespace)
			Expect(err).NotTo(HaveOccurred())

			// remote secret #2 is deployed only on environment #2
			// target should therefore contain target namespaces of only environment #2
			Expect(fw.AsKubeAdmin.RemoteSecretController.RemoteSecretTargetsContainsNamespace(targetNamespace_2, remoteSecret)).To(BeTrue(), fmt.Sprintf("namespace %s is not in targets of %s", targetNamespace_2, remoteSecretName_2))
			Expect(remoteSecret.Status.Targets).To(HaveLen(1))
		})

		It("checks targets in RemoteSecret #3 status contains (only) target namespace #2 and #3", func() {
			remoteSecret, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName_3, namespace)
			Expect(err).NotTo(HaveOccurred())

			// remote secret #3 is deployed on environment #2 and #3 for the applications and component
			// target should therefore contain target namespaces of both environments
			Expect(fw.AsKubeAdmin.RemoteSecretController.RemoteSecretTargetsContainsNamespace(targetNamespace_2, remoteSecret)).To(BeTrue(), fmt.Sprintf("namespace %s is not in targets of %s", targetNamespace_2, remoteSecretName_3))
			Expect(fw.AsKubeAdmin.RemoteSecretController.RemoteSecretTargetsContainsNamespace(targetNamespace_3, remoteSecret)).To(BeTrue(), fmt.Sprintf("namespace %s is not in targets of %s", targetNamespace_3, remoteSecretName_3))
			Expect(remoteSecret.Status.Targets).To(HaveLen(2))
		})

		It("secrets #2 and #3 should be deleted when Environment is deleted", func() {
			// Delete the existing Environments
			Expect(fw.AsKubeAdmin.GitOpsController.DeleteAllEnvironmentsInASpecificNamespace(fw.UserNamespace, 30*time.Second)).To(Succeed())

			// SPI cleans targets only in RS which explicitly have the environment name in labels or annotations
			Eventually(func() bool {
				// Secrets should not exist anymore in target namespaces
				_, errRs2Ns2 := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName_2)
				_, errRs3Ns2 := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_2, targetSecretName_3)
				_, errRs3Ns3 := fw.AsKubeAdmin.CommonController.GetSecret(targetNamespace_3, targetSecretName_3)

				return k8sErrors.IsNotFound(errRs2Ns2) && k8sErrors.IsNotFound(errRs3Ns2) && k8sErrors.IsNotFound(errRs3Ns3)
			}, 2*time.Minute, 1*time.Second).Should(BeTrue(), "secrets #2 and #3 are not deleted when Environment is deleted")

		})

		It("after Environment is deleted target namespace #2 and #3 are removed from targets in RemoteSecret status", func() {
			remoteSecret_2, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName_2, namespace)
			Expect(err).NotTo(HaveOccurred())
			targets_2 := remoteSecret_2.Status.Targets
			Expect(targets_2).To(BeEmpty())

			remoteSecret_3, err := fw.AsKubeDeveloper.RemoteSecretController.GetRemoteSecret(remoteSecretName_3, namespace)
			Expect(err).NotTo(HaveOccurred())
			targets_3 := remoteSecret_3.Status.Targets
			Expect(targets_3).To(BeEmpty())
		})

	})
})
