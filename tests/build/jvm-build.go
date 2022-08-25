package build

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/util/wait"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"knative.dev/pkg/apis"
)

var (
	testProjectGitUrl   = utils.GetEnv("JVM_BUILD_SERVICE_TEST_REPO_URL", "https://github.com/stuartwdouglas/hacbs-test-project")
	testProjectRevision = utils.GetEnv("JVM_BUILD_SERVICE_TEST_REPO_REVISION", "main")
)

var _ = framework.JVMBuildSuiteDescribe("JVM Build Service E2E tests", Label("jvm-build"), func() {
	defer GinkgoRecover()

	var testNamespace, applicationName, componentName, outputContainerImage string
	var componentPipelineRun v1beta1.PipelineRun
	var timeout, interval time.Duration
	var doCollectLogs bool

	f, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	AfterAll(func() {
		abList, err := f.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
		if err != nil {
			klog.Infof("got error fetching artifactbuilds: %s", err.Error())
		}

		dbList, err := f.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
		if err != nil {
			klog.Infof("got error fetching dependencybuilds: %s", err.Error())
		}

		if CurrentSpecReport().Failed() || doCollectLogs {
			var testLogsDir string
			artifactDir := os.Getenv("ARTIFACT_DIR")
			var storeLogsInFiles bool

			if artifactDir != "" {
				testLogsDir = fmt.Sprintf("%s/jvm-build-service-test", artifactDir)
				err := os.MkdirAll(testLogsDir, 0755)
				if err != nil && !os.IsExist(err) {
					klog.Infof("cannot create a folder %s for storing test logs/resources: %+v", testLogsDir, err)
				} else {
					storeLogsInFiles = true
				}
			}
			// get jvm-build-service logs
			toDebug := map[string]string{}

			jvmPodList, jerr := f.CommonController.K8sClient.KubeInterface().CoreV1().Pods("jvm-build-service").List(context.TODO(), metav1.ListOptions{})
			if jerr != nil {
				klog.Infof("error listing jvm-build-service pods: %s", jerr.Error())
			}
			klog.Infof("found %d pods in jvm-build-service namespace", len(jvmPodList.Items))
			for _, pod := range jvmPodList.Items {
				var containers []corev1.Container
				containers = append(containers, pod.Spec.InitContainers...)
				containers = append(containers, pod.Spec.Containers...)
				for _, c := range containers {
					cLog, cerr := f.CommonController.GetContainerLogs(pod.Name, c.Name, pod.Namespace)
					if cerr != nil {
						klog.Infof("error getting logs for pod/container %s/%s: %s", pod.Name, c.Name, cerr.Error())
						continue
					}
					filename := fmt.Sprintf("%s-pod-%s-%s.log", pod.Namespace, pod.Name, c.Name)
					toDebug[filename] = cLog
				}
			}
			// let's make sure and print the pr that starts the analysis first

			logs, err := f.TektonController.GetPipelineRunLogs(componentPipelineRun.Name, testNamespace)
			if err != nil {
				klog.Infof("got error fetching PR logs: %s", err.Error())
			}
			filename := fmt.Sprintf("%s-pr-%s.log", testNamespace, componentPipelineRun.Name)
			toDebug[filename] = logs

			prList, err := f.TektonController.ListAllPipelineRuns(testNamespace)
			if err != nil {
				klog.Infof("got error fetching PR list: %s", err.Error())
			}
			klog.Infof("total number of pipeline runs not pruned: %d", len(prList.Items))
			for _, pr := range prList.Items {
				if pr.Name == componentPipelineRun.Name {
					continue
				}
				prLog, err := f.TektonController.GetPipelineRunLogs(pr.Name, pr.Namespace)
				if err != nil {
					klog.Infof("got error fetching PR logs for %s: %s", pr.Name, err.Error())
				}
				filename := fmt.Sprintf("%s-pr-%s.log", pr.Namespace, pr.Name)
				toDebug[filename] = prLog
			}

			for _, ab := range abList.Items {
				v, err := json.MarshalIndent(ab, "", "  ")
				if err != nil {
					klog.Infof("error when marshalling content of %s from %s namespace: %+v", ab.Name, ab.Namespace, err)
				} else {
					filename := fmt.Sprintf("%s-ab-%s.json", ab.Namespace, ab.Name)
					toDebug[filename] = string(v)
				}
			}
			for _, db := range dbList.Items {
				v, err := json.MarshalIndent(db, "", "  ")
				if err != nil {
					klog.Infof("error when marshalling content of %s from %s namespace: %+v", db.Name, db.Namespace, err)
				} else {
					filename := fmt.Sprintf("%s-db-%s.json", db.Namespace, db.Name)
					toDebug[filename] = string(v)
				}
			}

			for file, content := range toDebug {
				if storeLogsInFiles {
					filename := fmt.Sprintf("%s/%s", testLogsDir, file)
					if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
						klog.Infof("cannot write to %s: %+v", filename, err)
					} else {
						continue
					}
				} else {
					klog.Infof("%s\n%s", file, content)
				}
			}
		}
		// Cleanup
		for _, ab := range abList.Items {
			err := f.JvmbuildserviceController.DeleteArtifactBuild(ab.Name, ab.Namespace)
			if err != nil {
				klog.Infof("got error deleting AB %s: %s", ab.Name, err.Error())
			}
		}
		for _, db := range dbList.Items {
			err := f.JvmbuildserviceController.DeleteDependencyBuild(db.Name, db.Namespace)
			if err != nil {
				klog.Infof("got error deleting DB %s: %s", db.Name, err.Error())
			}
		}
		Expect(f.HasController.DeleteHasComponent(componentName, testNamespace)).To(Succeed())
		Expect(f.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
		Expect(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
	})

	BeforeAll(func() {
		testNamespace = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, "appstudio-e2e-test")

		klog.Infof("Test namespace: %s", testNamespace)

		_, err := f.CommonController.CreateTestNamespace(testNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)

		customBundleConfigMap, err := f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, testNamespace)
		if err != nil {
			if errors.IsNotFound(err) {
				defaultBundleConfigMap, err := f.CommonController.GetConfigMap(constants.BuildPipelinesConfigMapName, constants.BuildPipelinesConfigMapDefaultNamespace)
				Expect(err).ToNot(HaveOccurred())

				bundlePullSpec := defaultBundleConfigMap.Data["default_build_bundle"]
				hacbsBundleConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: constants.BuildPipelinesConfigMapName},
					Data:       map[string]string{"default_build_bundle": strings.Replace(bundlePullSpec, "build-", "hacbs-", 1)},
				}
				_, err = f.CommonController.CreateConfigMap(hacbsBundleConfigMap, testNamespace)
				Expect(err).ToNot(HaveOccurred())
				DeferCleanup(f.CommonController.DeleteConfigMap, constants.BuildPipelinesConfigMapName, testNamespace, false)
			} else {
				Fail(fmt.Sprintf("error occured when trying to get configmap %s in %s namespace: %v", constants.BuildPipelinesConfigMapName, testNamespace, err))
			}
		} else {
			bundlePullSpec := customBundleConfigMap.Data["default_build_bundle"]
			hacbsBundleConfigMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: constants.BuildPipelinesConfigMapName},
				Data:       map[string]string{"default_build_bundle": strings.Replace(bundlePullSpec, "build-", "hacbs-", 1)},
			}

			_, err = f.CommonController.UpdateConfigMap(hacbsBundleConfigMap, testNamespace)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() error {
				hacbsBundleConfigMap.Data = customBundleConfigMap.Data
				_, err := f.CommonController.UpdateConfigMap(hacbsBundleConfigMap, testNamespace)
				if err != nil {
					return err
				}
				return nil
			})
		}

		timeout = time.Minute * 10
		interval = time.Second * 10

		applicationName = fmt.Sprintf("jvm-build-suite-application-%s", util.GenerateRandomString(4))
		_, err = f.HasController.CreateHasApplication(applicationName, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		componentName = fmt.Sprintf("jvm-build-suite-component-%s", util.GenerateRandomString(4))
		outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

		// Create a component with Git Source URL being defined
		_, err = f.HasController.CreateComponent(applicationName, componentName, testNamespace, testProjectGitUrl, testProjectRevision, "", outputContainerImage, "")
		Expect(err).ShouldNot(HaveOccurred())
	})

	When("the Component with s2i-java component is created", func() {
		It("a PipelineRun is triggered", func() {
			Eventually(func() bool {
				componentPipelineRun, err = f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false)
				if err != nil {
					klog.Infoln("PipelineRun has not been created yet")
					return false
				}
				return componentPipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
		})

		It("the build-container task from component PipelineRun references a correct sidecar image", func() {
			ciSidecarImage := os.Getenv("JVM_BUILD_SERVICE_SIDECAR_IMAGE")
			if ciSidecarImage == "" {
				Skip("JVM_BUILD_SERVICE_SIDECAR_IMAGE env var is not exported, skipping the test...")
			}

			err = wait.PollImmediate(interval, timeout, func() (done bool, err error) {
				pr, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false)
				if err != nil {
					klog.Infof("get pr for component %s produced err: %s", componentName, err.Error())
					return false, nil
				}

				for _, tr := range pr.Status.TaskRuns {
					if tr.PipelineTaskName == "build-container" && tr.Status != nil && tr.Status.TaskSpec != nil && tr.Status.TaskSpec.Sidecars != nil {
						for _, sc := range tr.Status.TaskSpec.Sidecars {
							if sc.Name == "proxy" {
								if sc.Image != ciSidecarImage {
									Fail(fmt.Sprintf("the build-container task from component pipelinerun doesn't contain correct sidecar image. expected: %v, actual: %v", ciSidecarImage, sc.Image))
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
				Fail(fmt.Sprintf("failure occured when verifying the sidecar image reference in pipelinerun: %v", err))
			}
		})

		It("the build-container task from component pipelinerun references a correct analyzer image", func() {
			ciAnalyzerImage := os.Getenv("JVM_BUILD_SERVICE_ANALYZER_IMAGE")

			if ciAnalyzerImage == "" {
				Skip("JVM_BUILD_SERVICE_ANALYZER_IMAGE env var is not exported, skipping the test...")
			}

			err = wait.PollImmediate(interval, timeout, func() (done bool, err error) {
				pr, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false)
				if err != nil {
					klog.Infof("get pr for the component %s produced err: %s", componentName, err.Error())
					return false, nil
				}

				for _, tr := range pr.Status.TaskRuns {
					if tr.PipelineTaskName == "build-container" && tr.Status != nil && tr.Status.TaskSpec != nil && tr.Status.TaskSpec.Steps != nil {
						for _, step := range tr.Status.TaskSpec.Steps {
							if step.Name == "analyse-dependencies-java-sbom" {
								if step.Image != ciAnalyzerImage {
									Fail(fmt.Sprintf("the build-container task from component pipelinerun doesn't reference the correct analyzer image. expected: %v, actual: %v", ciAnalyzerImage, step.Image))
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
				Fail(fmt.Sprintf("failure occured when verifying the analyzer image reference in pipelinerun: %v", err))
			}
		})

		It("that PipelineRun completes successfully", func() {
			Eventually(func() bool {
				pr, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false)
				if err != nil {
					klog.Infof("get of pr %s returned error: %s", pr.Name, err.Error())
					return false
				}
				if !pr.IsDone() {
					klog.Infof("pipeline run %s not done", pr.Name)
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
				abList, err := f.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
				if err != nil {
					klog.Infof("error listing artifactbuilds: %s", err.Error())
					return false
				}
				gotABs := false
				if len(abList.Items) > 0 {
					gotABs = true
				}
				dbList, err := f.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
				if err != nil {
					klog.Infof("error listing dependencybuilds: %s", err.Error())
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
			err = wait.PollImmediate(interval, 2*timeout, func() (done bool, err error) {
				abList, err := f.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
				if err != nil {
					klog.Infof("error listing artifactbuilds: %s", err.Error())
					return false, nil
				}
				abComplete := false
				for _, ab := range abList.Items {
					if ab.Status.State == v1alpha1.ArtifactBuildStateComplete {
						abComplete = true
						break
					}
				}
				dbList, err := f.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
				if err != nil {
					klog.Infof("error listing dependencybuilds: %s", err.Error())
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
