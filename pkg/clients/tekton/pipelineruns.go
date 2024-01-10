package tekton

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/logs"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	g "github.com/onsi/ginkgo/v2"
)

// CreatePipelineRun creates a tekton pipelineRun and returns the pipelineRun or error
func (t *TektonController) CreatePipelineRun(pipelineRun *tektonv1.PipelineRun, ns string) (*tektonv1.PipelineRun, error) {
	return t.PipelineClient().TektonV1().PipelineRuns(ns).Create(context.Background(), pipelineRun, metav1.CreateOptions{})
}

// createAndWait creates a pipelineRun and waits until it starts.
func (t *TektonController) createAndWait(pr *tektonv1.PipelineRun, namespace string, taskTimeout int) (*tektonv1.PipelineRun, error) {
	pipelineRun, err := t.CreatePipelineRun(pr, namespace)
	if err != nil {
		return nil, err
	}
	g.GinkgoWriter.Printf("Creating Pipeline %q\n", pipelineRun.Name)
	return pipelineRun, utils.WaitUntil(t.CheckPipelineRunStarted(pipelineRun.Name, namespace), time.Duration(taskTimeout)*time.Second)
}

// RunPipeline creates a pipelineRun and waits for it to start.
func (t *TektonController) RunPipeline(g tekton.PipelineRunGenerator, namespace string, taskTimeout int) (*tektonv1.PipelineRun, error) {
	pr, err := g.Generate()
	if err != nil {
		return nil, err
	}
	pvcs := t.KubeInterface().CoreV1().PersistentVolumeClaims(pr.Namespace)
	for _, w := range pr.Spec.Workspaces {
		if w.PersistentVolumeClaim != nil {
			pvcName := w.PersistentVolumeClaim.ClaimName
			if _, err := pvcs.Get(context.Background(), pvcName, metav1.GetOptions{}); err != nil {
				if errors.IsNotFound(err) {
					err := tekton.CreatePVC(pvcs, pvcName)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
		}
	}

	return t.createAndWait(pr, namespace, taskTimeout)
}

// GetPipelineRun returns a pipelineRun with a given name.
func (t *TektonController) GetPipelineRun(pipelineRunName, namespace string) (*tektonv1.PipelineRun, error) {
	return t.PipelineClient().TektonV1().PipelineRuns(namespace).Get(context.Background(), pipelineRunName, metav1.GetOptions{})
}

// GetPipelineRunLogs returns logs of a given pipelineRun.
func (t *TektonController) GetPipelineRunLogs(pipelineRunName, namespace string) (string, error) {
	podClient := t.KubeInterface().CoreV1().Pods(namespace)
	podList, err := podClient.List(context.Background(), metav1.ListOptions{})
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
			cLog, err = t.fetchContainerLog(pod.Name, c.Name, namespace)
			podLog = podLog + fmt.Sprintf("\ninit container %s: \n", c.Name) + cLog
			if err != nil {
				return podLog, err
			}
		}
		for _, c := range pod.Spec.Containers {
			var err error
			var cLog string
			cLog, err = t.fetchContainerLog(pod.Name, c.Name, namespace)
			podLog = podLog + fmt.Sprintf("\ncontainer %s: \n", c.Name) + cLog
			if err != nil {
				return podLog, err
			}
		}
	}
	return podLog, nil
}

// GetPipelineRunWatch returns pipelineRun watch interface.
func (t *TektonController) GetPipelineRunWatch(ctx context.Context, namespace string) (watch.Interface, error) {
	return t.PipelineClient().TektonV1beta1().PipelineRuns(namespace).Watch(ctx, metav1.ListOptions{})
}

// WatchPipelineRun waits until pipelineRun finishes.
func (t *TektonController) WatchPipelineRun(pipelineRunName, namespace string, taskTimeout int) error {
	g.GinkgoWriter.Printf("Waiting for pipeline %q to finish\n", pipelineRunName)
	return utils.WaitUntil(t.CheckPipelineRunFinished(pipelineRunName, namespace), time.Duration(taskTimeout)*time.Second)
}

// WatchPipelineRunSucceeded waits until the pipelineRun succeeds.
func (t *TektonController) WatchPipelineRunSucceeded(pipelineRunName, namespace string, taskTimeout int) error {
	g.GinkgoWriter.Printf("Waiting for pipeline %q to finish\n", pipelineRunName)
	return utils.WaitUntil(t.CheckPipelineRunSucceeded(pipelineRunName, namespace), time.Duration(taskTimeout)*time.Second)
}

// CheckPipelineRunStarted checks if pipelineRUn started.
func (t *TektonController) CheckPipelineRunStarted(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := t.GetPipelineRun(pipelineRunName, namespace)
		if err != nil {
			return false, nil
		}
		if pr.Status.StartTime != nil {
			return true, nil
		}
		return false, nil
	}
}

// CheckPipelineRunFinished checks if pipelineRun finished.
func (t *TektonController) CheckPipelineRunFinished(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := t.GetPipelineRun(pipelineRunName, namespace)
		if err != nil {
			return false, nil
		}
		if pr.Status.CompletionTime != nil {
			return true, nil
		}
		return false, nil
	}
}

// CheckPipelineRunSucceeded checks if pipelineRun succeeded. Returns error if getting pipelineRun fails.
func (t *TektonController) CheckPipelineRunSucceeded(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := t.GetPipelineRun(pipelineRunName, namespace)
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

// ListAllPipelineRuns returns a list of all pipelineRuns in a namespace.
func (t *TektonController) ListAllPipelineRuns(ns string) (*tektonv1.PipelineRunList, error) {
	return t.PipelineClient().TektonV1().PipelineRuns(ns).List(context.Background(), metav1.ListOptions{})
}

// DeletePipelineRun deletes a pipelineRun form a given namespace.
func (t *TektonController) DeletePipelineRun(name, ns string) error {
	return t.PipelineClient().TektonV1().PipelineRuns(ns).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// DeleteAllPipelineRunsInASpecificNamespace deletes all PipelineRuns in a given namespace (removing the finalizers field, first)
func (t *TektonController) DeleteAllPipelineRunsInASpecificNamespace(ns string) error {

	pipelineRunList, err := t.ListAllPipelineRuns(ns)
	if err != nil || pipelineRunList == nil {
		return fmt.Errorf("unable to delete all PipelineRuns in '%s': %v", ns, err)
	}

	for _, pipelineRun := range pipelineRunList.Items {
		err := wait.PollUntilContextTimeout(context.Background(), time.Second, 30*time.Second, true, func(ctx context.Context) (done bool, err error) {
			pipelineRunCR := tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: ns,
				},
			}
			if err := t.KubeRest().Get(context.Background(), crclient.ObjectKeyFromObject(&pipelineRunCR), &pipelineRunCR); err != nil {
				if errors.IsNotFound(err) {
					// PipelinerRun CR is already removed
					return true, nil
				}
				g.GinkgoWriter.Printf("unable to retrieve PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil

			}

			// Remove the finalizer, so that it can be deleted.
			pipelineRunCR.Finalizers = []string{}
			if err := t.KubeRest().Update(context.Background(), &pipelineRunCR); err != nil {
				g.GinkgoWriter.Printf("unable to remove finalizers from PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil
			}

			if err := t.KubeRest().Delete(context.Background(), &pipelineRunCR); err != nil {
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

// StorePipelineRun stores a given PipelineRun as an artifact.
func (t *TektonController) StorePipelineRun(pipelineRun *tektonv1.PipelineRun) error {
	artifacts := make(map[string][]byte)
	pipelineRunLog, err := t.GetPipelineRunLogs(pipelineRun.Name, pipelineRun.Namespace)
	if err != nil {
		return err
	}
	artifacts["pipelineRun-"+pipelineRun.Name+".log"] = []byte(pipelineRunLog)

	pipelineRunYaml, err := yaml.Marshal(pipelineRun)
	if err != nil {
		return err
	}
	artifacts["pipelineRun-"+pipelineRun.Name+".yaml"] = pipelineRunYaml

	if err := logs.StoreArtifacts(artifacts); err != nil {
		return err
	}

	return nil
}

// StoreAllPipelineRuns stores all PipelineRuns in a given namespace.
func (t *TektonController) StoreAllPipelineRuns(namespace string) error {
	pipelineRuns, err := t.ListAllPipelineRuns(namespace)
	if err != nil {
		return fmt.Errorf("got error fetching PR list: %v\n", err.Error())
	}

	for _, pipelineRun := range pipelineRuns.Items {
		pipelineRun := pipelineRun
		if err := t.StorePipelineRun(&pipelineRun); err != nil {
			return fmt.Errorf("got error storing PR: %v\n", err.Error())
		}
	}

	return nil
}
