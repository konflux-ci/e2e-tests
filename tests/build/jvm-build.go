package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	v1 "k8s.io/api/apps/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/library/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	testProjectGitUrl   = utils.GetEnv("JVM_BUILD_SERVICE_TEST_REPO_URL", "https://github.com/redhat-appstudio-qe/hacbs-test-project")
	testProjectRevision = utils.GetEnv("JVM_BUILD_SERVICE_TEST_REPO_REVISION", "main")
)

var _ = framework.JVMBuildSuiteDescribe("JVM Build Service E2E tests", Label("jvm-build", "HACBS"), func() {
	var f *framework.Framework
	var err error

	defer GinkgoRecover()

	var testNamespace, applicationName, componentName string
	var componentPipelineRun *v1beta1.PipelineRun
	var component *appservice.Component
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
					cLog, cerr := utils.GetContainerLogs(f.AsKubeAdmin.CommonController.KubeInterface(), pod.Name, c.Name, pod.Namespace)
					if cerr != nil {
						GinkgoWriter.Printf("error getting logs for pod/container %s/%s: %s\n", pod.Name, c.Name, cerr.Error())
						continue
					}
					filename := fmt.Sprintf("%s-pod-%s-%s.log", pod.Namespace, pod.Name, c.Name)
					toDebug[filename] = cLog
				}
			}
			// In case the test fails before the Component PipelineRun is created,
			// we are unable to collect following resources
			if componentPipelineRun != nil {
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
			Expect(f.AsKubeAdmin.HasController.DeleteApplication(applicationName, testNamespace, false)).To(Succeed())
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

		_, err = f.AsKubeAdmin.JvmbuildserviceController.CreateJBSConfig(constants.JBSConfigName, testNamespace)
		Expect(err).ShouldNot(HaveOccurred())

		//TODO: not using SPI at the moment for auto created repos
		//var SPITokenBinding *spi.SPIAccessTokenBinding
		////this should result in the creation of an SPIAccessTokenBinding
		//Eventually(func() bool {
		//	SPITokenBinding, err = f.AsKubeDeveloper.SPIController.GetSPIAccessTokenBinding(constants.JVMBuildImageSecretName, testNamespace)
		//
		//	if err != nil {
		//		return false
		//	}
		//
		//	return SPITokenBinding.Status.Phase == spi.SPIAccessTokenBindingPhaseInjected
		//}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Access token binding should be created")

		//wait for the cache

		WaitForCache(f.AsKubeAdmin, testNamespace)

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
		app, err := f.AsKubeAdmin.HasController.CreateApplication(applicationName, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(utils.WaitUntil(f.AsKubeAdmin.HasController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
			Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
		)

		componentName = fmt.Sprintf("jvm-build-suite-component-%s", util.GenerateRandomString(4))

		// Create a component with Git Source URL being defined
		component, err = f.AsKubeAdmin.HasController.CreateComponent(applicationName, componentName, testNamespace, testProjectGitUrl, testProjectRevision, "", "", "", true)
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

			Eventually(func() bool {
				pr, err := f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				if err != nil {
					GinkgoWriter.Printf("get pr for the component %s produced err: %s\n", componentName, err.Error())
					return false
				}

				for _, chr := range pr.Status.ChildReferences {
					taskRun := &v1beta1.TaskRun{}
					taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
					err := f.AsKubeAdmin.CommonController.KubeRest().Get(context.TODO(), taskRunKey, taskRun)
					Expect(err).ShouldNot(HaveOccurred())

					prTrStatus := &v1beta1.PipelineRunTaskRunStatus{
						PipelineTaskName: chr.PipelineTaskName,
						Status:           &taskRun.Status,
					}

					if chr.PipelineTaskName == "build-container" && prTrStatus.Status != nil && prTrStatus.Status.TaskSpec != nil && prTrStatus.Status.TaskSpec.Steps != nil {
						for _, step := range prTrStatus.Status.TaskSpec.Steps {
							if step.Name == "analyse-dependencies-java-sbom" {
								if step.Image != ciAnalyzerImage {
									Fail(fmt.Sprintf("the build-container task from component pipelinerun doesn't reference the correct request processor image. expected: %v, actual: %v", ciAnalyzerImage, step.Image))
								} else {
									return true
								}
							}
						}
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when verifying the request processor image reference in pipelinerun")
		})

		It("that PipelineRun completes successfully", func() {
			Expect(f.AsKubeAdmin.HasController.WaitForComponentPipelineToBeFinished(component, "", 2)).To(Succeed())
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
			Eventually(func() bool {
				abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
				if err != nil {
					GinkgoWriter.Printf("error listing artifactbuilds: %s\n", err.Error())
					return false
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
					return false
				}
				dbComplete := false
				for _, db := range dbList.Items {
					if db.Status.State == v1alpha1.DependencyBuildStateComplete {
						dbComplete = true
						break
					}
				}
				if abComplete && dbComplete {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for some artifactbuilds and dependencybuilds to complete")
		})

		It("all artifactbuild and dependencybuilds complete", func() {
			Eventually(func() bool {
				abList, err := f.AsKubeAdmin.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
				Expect(err).ShouldNot(HaveOccurred(), "error in listing artifact builds")
				// we want to make sure there is more than one ab and that they are all complete
				abComplete := len(abList.Items) > 0
				GinkgoWriter.Printf("number of artifactbuilds: %d\n", len(abList.Items))
				for _, ab := range abList.Items {
					if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
						GinkgoWriter.Printf("artifactbuild %s not complete\n", ab.Spec.GAV)
						abComplete = false
						break
					}
				}
				dbList, err := f.AsKubeAdmin.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
				Expect(err).ShouldNot(HaveOccurred(), "error in listing dependency builds")
				dbComplete := len(dbList.Items) > 0
				GinkgoWriter.Printf("number of dependencybuilds: %d\n", len(dbList.Items))
				for _, db := range dbList.Items {
					if db.Status.State != v1alpha1.DependencyBuildStateComplete {
						GinkgoWriter.Printf("dependencybuild %s not complete\n", db.Spec.ScmInfo.SCMURL)
						dbComplete = false
						break
					} else if db.Status.State == v1alpha1.DependencyBuildStateFailed {
						Fail(fmt.Sprintf("dependencybuild %s FAILED", db.Spec.ScmInfo.SCMURL))
					}
				}
				if abComplete && dbComplete {
					return true
				}
				return false
			}, 2*timeout, interval).Should(BeTrue(), "timed out when waiting for all artifactbuilds and dependencybuilds to complete")
		})

		It("does rebuild use cached dependencies", func() {
			prun := &v1beta1.PipelineRun{}

			component, err := f.AsKubeAdmin.HasController.GetComponent(componentName, testNamespace)
			Expect(err).ShouldNot(HaveOccurred(), "could not get component")

			annotations := component.GetAnnotations()
			delete(annotations, constants.ComponentInitialBuildAnnotationKey)
			component.SetAnnotations(annotations)
			Expect(f.AsKubeAdmin.CommonController.KubeRest().Update(context.TODO(), component, &client.UpdateOptions{})).To(Succeed())

			Eventually(func() bool {
				prun, err = f.AsKubeAdmin.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, "")
				return err == nil
			}, timeout, interval).Should(BeTrue())

			ctx := context.TODO()

			watch, err := f.AsKubeAdmin.TektonController.WatchPipelineRun(ctx, testNamespace)
			Expect(err).ShouldNot(HaveOccurred(), "watch pipelinerun failed")

			exitForLoop := false

			for {
				select {
				case <-time.After(15 * time.Minute):
					Fail("timed out waiting for second build to complete")
				case event := <-watch.ResultChan():
					if event.Object == nil {
						continue
					}
					pr, ok := event.Object.(*v1beta1.PipelineRun)
					if !ok {
						continue
					}
					if prun.Name != pr.Name {
						if pr.IsDone() {
							GinkgoWriter.Printf("got event for pipelinerun %s in a terminal state\n", pr.Name)
							continue
						}
						Fail("another non-completed pipeline run was generated when it should not")
					}
					GinkgoWriter.Printf("done processing event for pr %s\n", pr.Name)
					if pr.IsDone() {
						GinkgoWriter.Println("pr is done")

						podClient := f.AsKubeAdmin.CommonController.KubeInterface().CoreV1().Pods(testNamespace)
						listOptions := metav1.ListOptions{
							LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", pr.Name),
						}
						podList, err := podClient.List(context.TODO(), listOptions)
						Expect(err).ShouldNot(HaveOccurred(), "error listing pr pods")

						pods := podList.Items

						if len(pods) == 0 {
							Fail("pod for pipeline run unexpectedly missing")
						}

						containers := []corev1.Container{}
						containers = append(containers, pods[0].Spec.InitContainers...)
						containers = append(containers, pods[0].Spec.Containers...)

						for _, container := range containers {
							if !strings.Contains(container.Name, "analyse-dependecies") {
								continue
							}
							cLog, err := utils.GetContainerLogs(f.AsKubeAdmin.CommonController.KubeInterface(), pods[0].Name, container.Name, testNamespace)
							Expect(err).ShouldNot(HaveOccurred(), "getting container logs failed")
							if strings.Contains(cLog, "\"publisher\" : \"central\"") {
								Fail(fmt.Sprintf("pipelinerun %s has container %s with dep analysis still pointing to central %s", pr.Name, container.Name, cLog))
							}
							if !strings.Contains(cLog, "\"publisher\" : \"rebuilt\"") {
								Fail(fmt.Sprintf("pipelinerun %s has container %s with dep analysis that does not access rebuilt %s", pr.Name, container.Name, cLog))
							}
							if !strings.Contains(cLog, "\"java:scm-uri\" : \"https://github.com/stuartwdouglas/hacbs-test-simple-jdk8.git\"") {
								Fail(fmt.Sprintf("pipelinerun %s has container %s with dep analysis did not include java:scm-uri %s", pr.Name, container.Name, cLog))
							}
							if !strings.Contains(cLog, "\"java:scm-commit\" : \"") {
								Fail(fmt.Sprintf("pipelinerun %s has container %s with dep analysis did not include java:scm-commit %s", pr.Name, container.Name, cLog))
							}
							break
						}
						GinkgoWriter.Println("pr is done and has correct analyse-dependecies output, exiting")
						exitForLoop = true
					}
				}
				if exitForLoop {
					break
				}
			}
		})
	})
})

func WaitForCache(client *framework.ControllerHub, testNamespace string) bool {
	return Eventually(func() bool {
		cache, err := client.CommonController.GetDeployment(v1alpha1.CacheDeploymentName, testNamespace)
		if err != nil {
			GinkgoWriter.Printf("get of cache: %s", err.Error())
			return false
		}
		if cache.Status.AvailableReplicas > 0 {
			GinkgoWriter.Printf("Cache is available")
			return true
		}
		for _, cond := range cache.Status.Conditions {
			if cond.Type == v1.DeploymentProgressing && cond.Status == "False" {
				panic("cache deployment failed")
			}

		}
		GinkgoWriter.Printf("Cache is progressing")
		return false
	}, 5*time.Minute, 5*time.Second).Should(BeTrue(), "Cache should be created and ready")
}
