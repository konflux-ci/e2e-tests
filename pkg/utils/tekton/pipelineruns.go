package tekton

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	g "github.com/onsi/ginkgo/v2"
)

// Create a tekton pipelineRun and return the pipelineRun or error
func (s *SuiteController) CreatePipelineRun(pipelineRun *v1beta1.PipelineRun, ns string) (*v1beta1.PipelineRun, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(ns).Create(context.TODO(), pipelineRun, metav1.CreateOptions{})
}

func (k KubeController) createAndWait(pr *v1beta1.PipelineRun, taskTimeout int) (*v1beta1.PipelineRun, error) {
	pipelineRun, err := k.Tektonctrl.CreatePipelineRun(pr, k.Namespace)
	if err != nil {
		return nil, err
	}
	g.GinkgoWriter.Printf("Creating Pipeline %q\n", pipelineRun.Name)
	return pipelineRun, utils.WaitUntil(k.Tektonctrl.CheckPipelineRunStarted(pipelineRun.Name, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

func (s *SuiteController) GetPipelineRun(pipelineRunName, namespace string) (*v1beta1.PipelineRun, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(namespace).Get(context.TODO(), pipelineRunName, metav1.GetOptions{})
}

// DUPLICATION!!!!!!!!!!!!!!!!!!!!!
// GetListOfPipelineRunsInNamespace returns a List of all PipelineRuns in namespace.
func (s *SuiteController) GetListOfPipelineRunsInNamespace(namespace string) (*v1beta1.PipelineRunList, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(namespace).List(context.TODO(), metav1.ListOptions{})
}

func (s *SuiteController) GetPipelineRunLogs(pipelineRunName, namespace string) (string, error) {
	podClient := s.KubeInterface().CoreV1().Pods(namespace)
	podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	podLog := ""
	for _, pod := range podList.Items {
		if !strings.HasPrefix(pod.Name, pipelineRunName) {
			continue
		}
		for _, c := range pod.Spec.InitContainers {
			var err error
			var cLog string
			cLog, err = s.fetchContainerLog(pod.Name, c.Name, namespace)
			podLog = podLog + fmt.Sprintf("\ninit container %s: \n", c.Name) + cLog
			if err != nil {
				return podLog, err
			}
		}
		for _, c := range pod.Spec.Containers {
			var err error
			var cLog string
			cLog, err = s.fetchContainerLog(pod.Name, c.Name, namespace)
			podLog = podLog + fmt.Sprintf("\ncontainer %s: \n", c.Name) + cLog
			if err != nil {
				return podLog, err
			}
		}
	}
	return podLog, nil
}

// GetFailedPipelineRunLogs gets the logs of the pipelinerun failed task
func GetFailedPipelineRunLogs(c crclient.Client, ki kubernetes.Interface, pipelineRun *v1beta1.PipelineRun) (string, error) {
	var d *utils.FailedPipelineRunDetails
	var err error
	failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
	if d, err = utils.GetFailedPipelineRunDetails(c, pipelineRun); err != nil {
		return "", err
	}
	if d.FailedContainerName != "" {
		logs, _ := utils.GetContainerLogs(ki, d.PodName, d.FailedContainerName, pipelineRun.Namespace)
		failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
	}
	return failMessage, nil
}

// StorePipelineRunLogs stores logs and parsed yamls of pipelineRuns into directory of pipelineruns' namespace under ARTIFACT_DIR env.
// In case the files can't be stored in ARTIFACT_DIR, they will be recorder in GinkgoWriter.
func StorePipelineRun(pipelineRun *v1beta1.PipelineRun, c crclient.Client, ki kubernetes.Interface) error {
	wd, _ := os.Getwd()
	artifactDir := utils.GetEnv("ARTIFACT_DIR", fmt.Sprintf("%s/tmp", wd))
	testLogsDir := fmt.Sprintf("%s/%s", artifactDir, pipelineRun.GetNamespace())

	pipelineRunLog, err := GetFailedPipelineRunLogs(c, ki, pipelineRun)
	if err != nil {
		return fmt.Errorf("failed to store PipelineRun: %+v", err)
	}

	pipelineRunYaml, prYamlErr := yaml.Marshal(pipelineRun)
	if prYamlErr != nil {
		g.GinkgoWriter.Printf("\nfailed to get pipelineRunYaml: %s\n", prYamlErr.Error())
	}

	err = os.MkdirAll(testLogsDir, os.ModePerm)

	if err != nil {
		g.GinkgoWriter.Printf("\n%s\nFailed pipelineRunLog:\n%s\n---END OF THE LOG---\n", pipelineRun.GetName(), pipelineRunLog)
		if prYamlErr == nil {
			g.GinkgoWriter.Printf("Failed pipelineRunYaml:\n%s\n", pipelineRunYaml)
		}
		return err
	}

	filename := fmt.Sprintf("%s-pr-%s.log", pipelineRun.Namespace, pipelineRun.Name)
	filePath := fmt.Sprintf("%s/%s", testLogsDir, filename)
	if err := os.WriteFile(filePath, []byte(pipelineRunLog), 0644); err != nil {
		g.GinkgoWriter.Printf("cannot write to %s: %+v\n", filename, err)
		g.GinkgoWriter.Printf("\n%s\nFailed pipelineRunLog:\n%s\n", filename, pipelineRunLog)
	}

	if prYamlErr == nil {
		filename = fmt.Sprintf("%s-pr-%s.yaml", pipelineRun.Namespace, pipelineRun.Name)
		filePath = fmt.Sprintf("%s/%s", testLogsDir, filename)
		if err := os.WriteFile(filePath, pipelineRunYaml, 0644); err != nil {
			g.GinkgoWriter.Printf("cannot write to %s: %+v\n", filename, err)
			g.GinkgoWriter.Printf("\n%s\nFailed pipelineRunYaml:\n%s\n", filename, pipelineRunYaml)
		}
	}

	return nil
}

func (s *SuiteController) StorePipelineRuns(componentPipelineRun *v1beta1.PipelineRun, testLogsDir, testNamespace string) error {
	if err := os.MkdirAll(testLogsDir, os.ModePerm); err != nil {
		return err
	}

	filepath := fmt.Sprintf("%s/%s-pr-%s.log", testLogsDir, testNamespace, componentPipelineRun.Name)
	pipelineLogs, err := s.GetPipelineRunLogs(componentPipelineRun.Name, testNamespace)
	if err != nil {
		g.GinkgoWriter.Printf("got error fetching PR logs: %v\n", err.Error())
	} else {
		if err := os.WriteFile(filepath, []byte(pipelineLogs), 0644); err != nil {
			g.GinkgoWriter.Printf("cannot write to %s: %+v\n", filepath, err)
		}
	}

	pipelineRuns, err := s.ListAllPipelineRuns(testNamespace)

	if err != nil {
		return fmt.Errorf("got error fetching PR list: %v\n", err.Error())
	}

	for _, pipelineRun := range pipelineRuns.Items {
		filepath := fmt.Sprintf("%s/%s-pr-%s.log", testLogsDir, testNamespace, pipelineRun.Name)
		pipelineLogs, err := s.GetPipelineRunLogs(pipelineRun.Name, testNamespace)
		if err != nil {
			g.GinkgoWriter.Printf("got error fetching PR logs: %v\n", err.Error())
			continue
		}

		if err := os.WriteFile(filepath, []byte(pipelineLogs), 0644); err != nil {
			g.GinkgoWriter.Printf("cannot write to %s: %+v\n", filepath, err)
		}
	}

	return nil
}

func (s *SuiteController) WatchPipelineRun(ctx context.Context, namespace string) (watch.Interface, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(namespace).Watch(ctx, metav1.ListOptions{})
}

func (k KubeController) WatchPipelineRun(pipelineRunName string, taskTimeout int) error {
	g.GinkgoWriter.Printf("Waiting for pipeline %q to finish\n", pipelineRunName)
	return utils.WaitUntil(k.Tektonctrl.CheckPipelineRunFinished(pipelineRunName, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

func (k KubeController) WatchPipelineRunSucceeded(pipelineRunName string, taskTimeout int) error {
	g.GinkgoWriter.Printf("Waiting for pipeline %q to finish\n", pipelineRunName)
	return utils.WaitUntil(k.Tektonctrl.CheckPipelineRunSucceeded(pipelineRunName, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

func (s *SuiteController) CheckPipelineRunStarted(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := s.GetPipelineRun(pipelineRunName, namespace)
		if err != nil {
			return false, nil
		}
		if pr.Status.StartTime != nil {
			return true, nil
		}
		return false, nil
	}
}

func (s *SuiteController) CheckPipelineRunFinished(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := s.GetPipelineRun(pipelineRunName, namespace)
		if err != nil {
			return false, nil
		}
		if pr.Status.CompletionTime != nil {
			return true, nil
		}
		return false, nil
	}
}

func (s *SuiteController) CheckPipelineRunSucceeded(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := s.GetPipelineRun(pipelineRunName, namespace)
		if err != nil {
			return false, err
		}
		if len(pr.Status.Conditions) > 0 {
			for _, c := range pr.Status.Conditions {
				if c.Type == "Succeeded" && c.Status == "True" {
					return true, nil
				}
			}
		}
		return false, nil
	}
}

func (s *SuiteController) ListAllPipelineRuns(ns string) (*v1beta1.PipelineRunList, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(ns).List(context.TODO(), metav1.ListOptions{})
}

func (s *SuiteController) DeletePipelineRun(name, ns string) error {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// DeleteAllPipelineRunsInASpecificNamespace deletes all PipelineRuns in a given namespace (removing the finalizers field, first)
func (s *SuiteController) DeleteAllPipelineRunsInASpecificNamespace(ns string) error {

	pipelineRunList, err := s.ListAllPipelineRuns(ns)
	if err != nil || pipelineRunList == nil {
		return fmt.Errorf("unable to delete all PipelineRuns in '%s': %v", ns, err)
	}

	for _, pipelineRun := range pipelineRunList.Items {
		err := wait.PollImmediate(time.Second, 30*time.Second, func() (done bool, err error) {
			pipelineRunCR := v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: ns,
				},
			}
			if err := s.KubeRest().Get(context.TODO(), crclient.ObjectKeyFromObject(&pipelineRunCR), &pipelineRunCR); err != nil {
				if errors.IsNotFound(err) {
					// PipelinerRun CR is already removed
					return true, nil
				}
				g.GinkgoWriter.Printf("unable to retrieve PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil

			}

			// Remove the finalizer, so that it can be deleted.
			pipelineRunCR.Finalizers = []string{}
			if err := s.KubeRest().Update(context.TODO(), &pipelineRunCR); err != nil {
				g.GinkgoWriter.Printf("unable to remove finalizers from PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil
			}

			if err := s.KubeRest().Delete(context.TODO(), &pipelineRunCR); err != nil {
				g.GinkgoWriter.Printf("unable to delete PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return fmt.Errorf("deletion of PipelineRun '%s' in '%s' timed out", pipelineRun.Name, ns)
		}

	}

	return nil
}
