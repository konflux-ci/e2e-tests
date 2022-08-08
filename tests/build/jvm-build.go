package build

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"strings"
// 	"time"

// 	"github.com/devfile/library/pkg/util"
// 	"github.com/google/uuid"
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// 	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"

// 	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
// 	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
// 	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	klog "k8s.io/klog/v2"
// 	"knative.dev/pkg/apis"
// )

// const (
// 	testProjectGitUrl = "https://github.com/stuartwdouglas/hacbs-test-project"
// 	// TODO update this
// 	testProjectDevfileUrl = "https://raw.githubusercontent.com/psturc/shaded-java-app/main/devfile.yaml"
// )

// var _ = framework.JVMBuildSuiteDescribe("JVM Build Service E2E tests", Label("jvm-build"), func() {
// 	defer GinkgoRecover()

// 	var testNamespace, prGeneratedName, applicationName, componentName, outputContainerImage string
// 	var timeout, interval time.Duration

// 	f, err := framework.NewFramework()
// 	Expect(err).NotTo(HaveOccurred())

// 	// got panics in DeferCleanup when I tried to do multi param invocations, so following the pattern we used in openshift/origin
// 	AfterAll(func() {
// 		if CurrentSpecReport().Failed() {
// 			// get jvm-build-service logs
// 			jvmPodList, jerr := f.CommonController.K8sClient.KubeInterface().CoreV1().Pods("jvm-build-service").List(context.TODO(), metav1.ListOptions{})
// 			if jerr != nil {
// 				klog.Infof("error listing jvm-build-service pods: %s", jerr.Error())
// 			}
// 			klog.Infof("found %d pods in jvm-build-service namespace", len(jvmPodList.Items))
// 			for _, pod := range jvmPodList.Items {
// 				podLog := fmt.Sprintf("jvm-build-service namespace pod %s:\n", pod.Name)
// 				containers := []corev1.Container{}
// 				containers = append(containers, pod.Spec.InitContainers...)
// 				containers = append(containers, pod.Spec.Containers...)
// 				for _, c := range containers {
// 					cLog, cerr := f.CommonController.GetContainerLogs(pod.Name, c.Name, pod.Namespace)
// 					if cerr != nil {
// 						klog.Infof("error getting logs for pod/container %s/%s: %s", pod.Name, c.Name, cerr.Error())
// 						continue
// 					}
// 					podLog = fmt.Sprintf("%s\n%s\n", podLog, cLog)
// 				}
// 				klog.Info(podLog)
// 			}
// 			// let's make sure and print the pr that starts the analysis first
// 			logs, err := f.TektonController.GetPipelineRunLogs(prGeneratedName, testNamespace)
// 			if err != nil {
// 				klog.Infof("got error fetching PR logs: %s", err.Error())
// 			}
// 			klog.Infof("failed PR logs: %s", logs)
// 			prList, err := f.TektonController.ListAllPipelineRuns(testNamespace)
// 			if err != nil {
// 				klog.Infof("got error fetching PR list: %s", err.Error())
// 			}
// 			klog.Infof("total number of pipeline runs not pruned: %d", len(prList.Items))
// 			for _, pr := range prList.Items {
// 				if pr.Name == prGeneratedName {
// 					continue
// 				}
// 				prLog, err := f.TektonController.GetPipelineRunLogs(pr.Name, pr.Namespace)
// 				if err != nil {
// 					klog.Infof("got error fetching PR logs for %s: %s", pr.Name, err.Error())
// 				}
// 				klog.Infof("pipeline run log for %s: %s", pr.Name, prLog)
// 			}
// 		}
// 		abList, err := f.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
// 		if err != nil {
// 			klog.Infof("got error fetching artifactbuilds: %s", err.Error())
// 		}
// 		for _, ab := range abList.Items {
// 			err := f.JvmbuildserviceController.DeleteArtifactBuild(ab.Name, ab.Namespace)
// 			if err != nil {
// 				klog.Infof("got error deleting AB %s: %s", ab.Name, err.Error())
// 			}
// 		}
// 		dbList, err := f.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
// 		if err != nil {
// 			klog.Infof("got error fetching dependencybuilds: %s", err.Error())
// 		}
// 		for _, db := range dbList.Items {
// 			err := f.JvmbuildserviceController.DeleteDependencyBuild(db.Name, db.Namespace)
// 			if err != nil {
// 				klog.Infof("got error deleting DB %s: %s", db.Name, err.Error())
// 			}
// 		}
// 		err = f.TektonController.DeletePipelineRun(prGeneratedName, testNamespace)
// 		if err != nil {
// 			klog.Infof("error deleting pr %s: %s", prGeneratedName, err.Error())
// 		}
// 		err = f.TektonController.DeletePipeline("sample-component-build", testNamespace)
// 		if err != nil {
// 			klog.Infof("error deleting pipeline sample-component-build: %s", err.Error())
// 		}
// 		err = f.TektonController.DeleteTask("maven", testNamespace)
// 		if err != nil {
// 			klog.Infof("error deleting task maven: %s", err.Error())
// 		}
// 		err = f.TektonController.DeleteTask("git-clone", testNamespace)
// 		if err != nil {
// 			klog.Infof("error deleting task git-clone", err.Error())
// 		}
// 	})

// 	BeforeAll(func() {
// 		testNamespace = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, "appstudio-e2e-test")

// 		klog.Infof("Test namespace: %s", testNamespace)

// 		_, err := f.CommonController.CreateTestNamespace(testNamespace)
// 		Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)

// 		timeout = time.Minute * 10
// 		interval = time.Second * 10

// 		applicationName = fmt.Sprintf("jvm-build-suite-application-%s", util.GenerateRandomString(4))
// 		_, err = f.HasController.CreateHasApplication(applicationName, testNamespace)
// 		Expect(err).NotTo(HaveOccurred())

// 		componentName = fmt.Sprintf("jvm-build-suite-component-%s", util.GenerateRandomString(4))
// 		outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))

// 		// Create a component with Git Source URL being defined
// 		_, err = f.HasController.CreateComponentFromDevfile(applicationName, componentName, testNamespace, testProjectGitUrl, testProjectDevfileUrl, "", outputContainerImage, "")
// 		Expect(err).ShouldNot(HaveOccurred())

// 		DeferCleanup(f.TektonController.DeleteAllPipelineRunsInASpecificNamespace, testNamespace)
// 	})

// 	When("the Component with s2i-java component is created", func() {
// 		It("a PipelineRun is triggered", func() {
// 			Eventually(func() bool {
// 				pipelineRun, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false)
// 				if err != nil {
// 					klog.Infoln("PipelineRun has not been created yet")
// 					return false
// 				}
// 				return pipelineRun.HasStarted()
// 			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to start")
// 		})

// 		It("that PipelineRun completes successfully", func() {
// 			Eventually(func() bool {
// 				pr, err := f.HasController.GetComponentPipelineRun(componentName, applicationName, testNamespace, false)
// 				if err != nil {
// 					klog.Infof("get of pr %s returned error: %s", prGeneratedName, err.Error())
// 					return false
// 				}
// 				if !pr.IsDone() {
// 					klog.Infof("pipeline run %s not done", pr.Name)
// 					return false
// 				}
// 				if !pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
// 					// just because the condition succeeded is not set does not mean it won't be soon
// 					prBytes, err := json.MarshalIndent(pr, "", "  ")
// 					if err != nil {
// 						klog.Infof("problem marshalling failed pipelinerun to bytes: %s", err.Error())
// 						return false
// 					}
// 					klog.Infof("not yet successful pipeline run: %s", string(prBytes))
// 					return false
// 				}
// 				return true
// 			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the pipeline run to complete")
// 		})
// 		It("artifactbuilds and dependencybuilds are generated", func() {
// 			Eventually(func() bool {
// 				abList, err := f.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
// 				if err != nil {
// 					klog.Infof("error listing artifactbuilds: %s", err.Error())
// 					return false
// 				}
// 				gotABs := false
// 				if len(abList.Items) > 0 {
// 					gotABs = true
// 				}
// 				dbList, err := f.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
// 				if err != nil {
// 					klog.Infof("error listing dependencybuilds: %s", err.Error())
// 					return false
// 				}
// 				gotDBs := false
// 				if len(dbList.Items) > 0 {
// 					gotDBs = true
// 				}
// 				if gotABs && gotDBs {
// 					return true
// 				}
// 				return false
// 			}, timeout, interval).Should(BeTrue(), "timed out when waiting for the generation of artifactbuilds and dependencybuilds")
// 		})

// 		It("some artifactbuilds and dependencybuilds complete", func() {
// 			Eventually(func() bool {
// 				abList, err := f.JvmbuildserviceController.ListArtifactBuilds(testNamespace)
// 				if err != nil {
// 					klog.Infof("error listing artifactbuilds: %s", err.Error())
// 					return false
// 				}
// 				abComplete := false
// 				for _, ab := range abList.Items {
// 					if ab.Status.State == v1alpha1.ArtifactBuildStateComplete {
// 						abComplete = true
// 						break
// 					}
// 				}
// 				dbList, err := f.JvmbuildserviceController.ListDependencyBuilds(testNamespace)
// 				if err != nil {
// 					klog.Infof("error listing dependencybuilds: %s", err.Error())
// 					return false
// 				}
// 				dbComplete := false
// 				for _, db := range dbList.Items {
// 					if db.Status.State == v1alpha1.DependencyBuildStateComplete {
// 						dbComplete = true
// 						break
// 					}
// 				}
// 				if abComplete && dbComplete {
// 					return true
// 				}
// 				return false
// 			}, 2*timeout, interval).Should(BeTrue(), "timed out waiting for some artifactbuilds/dependencybuilds to complete")
// 		})
// 	})
// })
