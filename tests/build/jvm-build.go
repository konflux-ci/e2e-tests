package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var (
	testProjectGitUrl   = utils.GetEnv("JVM_BUILD_SERVICE_TEST_REPO_URL", "https://github.com/redhat-appstudio-qe/hacbs-test-project")
	testProjectRevision = utils.GetEnv("JVM_BUILD_SERVICE_TEST_REPO_REVISION", "main")
)

var _ = framework.JVMBuildSuiteDescribe("JVM Build Service E2E tests", Label("jvm-build", "HACBS"), func() {
	var f *framework.Framework
	var err error

	defer GinkgoRecover()

	var testNamespace, applicationName, componentName, outputContainerImage string
	var componentPipelineRun *v1beta1.PipelineRun
	var timeout, interval time.Duration
	var doCollectLogs bool

	AfterAll(func() {
		abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
		if err != nil {
			GinkgoWriter.Printf("got error fetching artifactbuilds: %s\n", err.Error())
		}

		dbList, err := f.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
		if err != nil {
			GinkgoWriter.Printf("got error fetching dependencybuilds: %s\n", err.Error())
		}

		if CurrentSpecReport().Failed() || doCollectLogs {
			var testLogsDir string
			artifactDir := os.Getenv("ARTIFACT_DIR")
			var storeLogsInFiles bool

			if artifactDir != "" {
				testLogsDir = fmt.Sprintf("%s/jvm-build-service-test", artifactDir)
				err := os.MkdirAll(testLogsDir, 0755)
				if err != nil && !os.IsExist(err) {
					GinkgoWriter.Printf("cannot create a folder %s for storing test logs/resources: %+v\n", testLogsDir, err)
				} else {
					storeLogsInFiles = true
				}
			}
			// get jvm-build-service logs
			toDebug := map[string]string{}

			jvmPodList, jerr := f.AsKubeAdmin.CommonController.KubeInterface().CoreV1().Pods("jvm-build-service").List(context.TODO(), metav1.ListOptions{})
			if jerr != nil {
				GinkgoWriter.Printf("error listing jvm-build-service pods: %s\n", jerr.Error())
			}
			GinkgoWriter.Printf("found %d pods in jvm-build-service namespace\n", len(jvmPodList.Items))
			for _, pod := range jvmPodList.Items {
				var containers []corev1.Container
				containers = append(containers, pod.Spec.InitContainers...)
				containers = append(containers, pod.Spec.Containers...)
				for _, c := range containers {
					cLog, cerr := f.AsKubeAdmin.CommonController.GetContainerLogs(pod.Name, c.Name, pod.Namespace)
					if cerr != nil {
						GinkgoWriter.Printf("error getting logs for pod/container %s/%s: %s\n", pod.Name, c.Name, cerr.Error())
						continue
					}
					filename := fmt.Sprintf("%s-pod-%s-%s.log", pod.Namespace, pod.Name, c.Name)
					toDebug[filename] = cLog
				}
			}
			// let's make sure and print the pr that starts the analysis first

			logs, err := f.AsKubeAdmin.TektonController.GetPipelineRunLogs(componentPipelineRun.Name, testNamespace)
			if err != nil {
				GinkgoWriter.Printf("got error fetching PR logs: %s\n", err.Error())
			}
			filename := fmt.Sprintf("%s-pr-%s.log", testNamespace, componentPipelineRun.Name)
			toDebug[filename] = logs

			prList, err := f.AsKubeAdmin.TektonController.ListAllPipelineRuns(testNamespace)
			if err != nil {
				GinkgoWriter.Printf("got error fetching PR list: %s\n", err.Error())
			}
			GinkgoWriter.Printf("total number of pipeline runs not pruned: %d\n", len(prList.Items))
			for _, pr := range prList.Items {
				if pr.Name == componentPipelineRun.Name {
					continue
				}
				prLog, err := f.AsKubeAdmin.TektonController.GetPipelineRunLogs(pr.Name, pr.Namespace)
				if err != nil {
					GinkgoWriter.Printf("got error fetching PR logs for %s: %s\n", pr.Name, err.Error())
				}
				filename := fmt.Sprintf("%s-pr-%s.log", pr.Namespace, pr.Name)
				toDebug[filename] = prLog
			}

			for _, ab := range abList.Items {
				v, err := json.MarshalIndent(ab, "", "  ")
				if err != nil {
					GinkgoWriter.Printf("error when marshalling content of %s from %s namespace: %+v\n", ab.Name, ab.Namespace, err)
				} else {
					filename := fmt.Sprintf("%s-ab-%s.json", ab.Namespace, ab.Name)
					toDebug[filename] = string(v)
				}
			}
			for _, db := range dbList.Items {
				v, err := json.MarshalIndent(db, "", "  ")
				if err != nil {
					GinkgoWriter.Printf("error when marshalling content of %s from %s namespace: %+v\n", db.Name, db.Namespace, err)
				} else {
					filename := fmt.Sprintf("%s-db-%s.json", db.Namespace, db.Name)
					toDebug[filename] = string(v)
				}
			}

			for file, content := range toDebug {
				if storeLogsInFiles {
					filename := fmt.Sprintf("%s/%s", testLogsDir, file)
					if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
						GinkgoWriter.Printf("cannot write to %s: %+v\n", filename, err)
					} else {
						continue
					}
				} else {
					GinkgoWriter.Printf("%s\n%s\n", file, content)
				}
			}
		} else {
			Expect(f.AsKubeAdmin.HasController.DeleteHasComponent(componentName, testNamespace, false)).To(Succeed())
			Expect(f.AsKubeAdmin.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
			Expect(f.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
			Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
		}
		// Cleanup artifact builds and dependency builds which are already
		// archived in case of a failure
		for _, ab := range abList.Items {
			err := f.AsKubeAdmin.JvmbuildserviceController.DeleteArtifactBuild(ab.Name, ab.Namespace)
			if err != nil {
				GinkgoWriter.Printf("got error deleting AB %s: %s\n", ab.Name, err.Error())
			}
		}
		for _, db := range dbList.Items {
			err := f.AsKubeAdmin.JvmbuildserviceController.DeleteDependencyBuild(db.Name, db.Namespace)
			if err != nil {
				GinkgoWriter.Printf("got error deleting DB %s: %s\n", db.Name, err.Error())
			}
		}
	})

	BeforeAll(func() {
		f, err = framework.NewFramework(utils.GetGeneratedNamespace("jvm-build"))
		Expect(err).NotTo(HaveOccurred())
		testNamespace = f.UserNamespace
		Expect(testNamespace).NotTo(BeNil(), "failed to create sandbox user namespace")

		GinkgoWriter.Printf("Test namespace: %s\n", testNamespace)

		_, err = f.AsKubeAdmin.JvmbuildserviceController.CreateJBSConfig(constants.JBSConfigName, testNamespace, utils.GetQuayIOOrganization())
		Expect(err).ShouldNot(HaveOccurred())

		sharedSecret, err := f.AsKubeAdmin.CommonController.GetSecret(constants.SharedPullSecretNamespace, constants.SharedPullSecretName)
		Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("error when getting shared secret - make sure the secret %s in %s namespace is created", constants.SharedPullSecretName, constants.SharedPullSecretNamespace))

		jvmBuildSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: constants.JVMBuildImageSecretName, Namespace: testNamespace},
			Data: map[string][]byte{".dockerconfigjson": sharedSecret.Data[".dockerconfigjson"]}}
		_, err = f.AsKubeAdmin.CommonController.CreateSecret(testNamespace, jvmBuildSecret)
		Expect(err).ShouldNot(HaveOccurred())

		customJavaPipelineBundleRef := os.Getenv(constants.CUSTOM_JAVA_PIPELINE_BUILD_BUNDLE_ENV)
		if len(customJavaPipelineBundleRef) > 0 {
			ps := &buildservice.BuildPipelineSelector{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "build-pipeline-selector",
					Namespace: testNamespace,
				},
				Spec: buildservice.BuildPipelineSelectorSpec{Selectors: []buildservice.PipelineSelector{
					{
						Name: "custom java selector",
						PipelineRef: v1beta1.PipelineRef{
							Name:   "java-builder",
							Bundle: customJavaPipelineBundleRef,
						},
						WhenConditions: buildservice.WhenCondition{Language: "java"},
					},
				}},
			}
			Expect(f.AsKubeAdmin.CommonController.KubeRest().Create(context.TODO(), ps)).To(Succeed())
		}

		timeout = time.Minute * 20
		interval = time.Second * 10

		applicationName = fmt.Sprintf("jvm-build-suite-application-%s", util.GenerateRandomString(4))
		app, err := f.AsKubeAdmin.HasController.CreateHasApplication(applicationName, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
			Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
		)

		componentName = fmt.Sprintf("jvm-build-suite-component-%s", util.GenerateRandomString(4))
		outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

		// Create a component with Git Source URL being defined
		_, err = f.AsKubeAdmin.HasController.CreateComponent(applicationName, componentName, testNamespace, testProjectGitUrl, testProjectRevision, "", outputContainerImage, "", true)
		Expect(err).ShouldNot(HaveOccurred())
	})

	When("the Component with s2i-java component is created", func() {
		It("a PipelineRun is triggered", func() {
			Eventually(func() bool {
				componentPipelineRun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Println("PipelineRun has not been created yet")
					return false
				}
				return componentPipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
		})

		It("the build-container task from component pipelinerun references a correct analyzer image", func() {
			ciAnalyzerImage := os.Getenv("JVM_BUILD_SERVICE_REQPROCESSOR_IMAGE")

			if ciAnalyzerImage == "" {
				Skip("JVM_BUILD_SERVICE_REQPROCESSOR_IMAGE env var is not exported, skipping the test...")
			}

			err = wait.PollImmediate(interval, timeout, func() (done bool, err error) {
				pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Printf("get pr for the component %s produced err: %s\n", componentName, err.Error())
					return false, nil
				}

				for _, tr := range pr.Status.TaskRuns {
					if tr.PipelineTaskName == "build-container" && tr.Status != nil && tr.Status.TaskSpec != nil && tr.Status.TaskSpec.Steps != nil {
						for _, step := range tr.Status.TaskSpec.Steps {
							if step.Name == "analyse-dependencies-java-sbom" {
								if step.Image != ciAnalyzerImage {
									Fail(fmt.Sprintf("the build-container task from component pipelinerun doesn't reference the correct request processor image. expected: %v, actual: %v", ciAnalyzerImage, step.Image))
								} else {
									return true, nil
								}
							}
						}
					}
				}
				return false, nil
			})
			if err != nil {
				Fail(fmt.Sprintf("failure occurred when verifying the request processor image reference in pipelinerun: %v", err))
			}
		})

		It("that PipelineRun completes successfully", func() {
			Eventually(func() bool {
				pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Printf("get of pr %s returned error: %s\n", pr.Name, err.Error())
					return false
				}
				if !pr.IsDone() {
					GinkgoWriter.Printf("pipeline run %s not done\n", pr.Name)
					return false
				}
				if !pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
					Fail("component pipeline run did not succeed")
				}
				return true
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the pipeline run to complete")
		})

		It("artifactbuilds and dependencybuilds are generated", func() {
			Eventually(func() bool {
				abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing artifactbuilds: %s\n", err.Error())
					return false
				}
				gotABs := false
				if len(abList.Items) > 0 {
					gotABs = true
				}
				dbList, err := f.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing dependencybuilds: %s\n", err.Error())
					return false
				}
				gotDBs := false
				if len(dbList.Items) > 0 {
					gotDBs = true
				}
				if gotABs && gotDBs {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the generation of artifactbuilds and dependencybuilds")
		})

		It("some artifactbuilds and dependencybuilds complete", func() {
			err = wait.PollImmediate(interval, timeout, func() (done bool, err error) {
				abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing artifactbuilds: %s\n", err.Error())
					return false, nil
				}
				abComplete := false
				for _, ab := range abList.Items {
					if ab.Status.State == v1alpha1.ArtifactBuildStateComplete {
						abComplete = true
						break
					}
				}
				dbList, err := f.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing dependencybuilds: %s\n", err.Error())
					return false, nil
				}
				dbComplete := false
				for _, db := range dbList.Items {
					if db.Status.State == v1alpha1.DependencyBuildStateComplete {
						dbComplete = true
						break
					}
				}
				if abComplete && dbComplete {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				ciRepoName := os.Getenv("REPO_NAME")
				// Fail only in case the test was run from jvm-build-service repo or locally
				if ciRepoName == "jvm-build-service" || ciRepoName == "" {
					Fail("timed out waiting for some artifactbuilds/dependencybuilds to complete")
				} else {
					doCollectLogs = true
					Skip("SKIPPING: unstable feature: timed-out when waiting for some artifactbuilds and dependencybuilds complete")
				}
			}
		})
	})
})
