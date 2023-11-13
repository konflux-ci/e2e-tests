package tekton

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/redhat-appstudio/e2e-tests/pkg/logs"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	g "github.com/onsi/ginkgo/v2"
)

const sslCertDir = "/var/run/secrets/kubernetes.io/serviceaccount"

type PipelineRunGenerator interface {
	Generate() (*v1beta1.PipelineRun, error)
}

type BuildahDemo struct {
	Image     string
	Bundle    string
	Name      string
	Namespace string
}

type ECIntegrationTestScenario struct {
	Image                 string
	Name                  string
	Namespace             string
	PipelineGitURL        string
	PipelineGitRevision   string
	PipelineGitPathInRepo string
}

// This is a demo pipeline to create test image and task signing
func (b BuildahDemo) Generate() (*v1beta1.PipelineRun, error) {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.Name,
			Namespace: b.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			Params: []v1beta1.Param{
				{
					Name:  "dockerfile",
					Value: *v1beta1.NewArrayOrString("Dockerfile"),
				},
				{
					Name:  "output-image",
					Value: *v1beta1.NewArrayOrString(b.Image),
				},
				{
					Name:  "git-url",
					Value: *v1beta1.NewArrayOrString("https://github.com/ziwoshixianzhe/simple_docker_app.git"),
				},
				{
					Name:  "skip-checks",
					Value: *v1beta1.NewArrayOrString("true"),
				},
			},
			PipelineRef: utils.NewBundleResolverPipelineRef("docker-build", b.Bundle),
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name: "workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "app-studio-default-workspace",
					},
				},
			},
		},
	}, nil
}

// Generates pipelineRun from VerifyEnterpriseContract.
func (p VerifyEnterpriseContract) Generate() (*v1beta1.PipelineRun, error) {
	var applicationSnapshotJSON, err = json.Marshal(p.Snapshot)
	if err != nil {
		return nil, err
	}
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-run-", p.Name),
			Namespace:    p.Namespace,
			Labels: map[string]string{
				"appstudio.openshift.io/application": p.Snapshot.Application,
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name: "verify-enterprise-contract",
						Params: []v1beta1.Param{
							{
								Name: "IMAGES",
								Value: v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeString,
									StringVal: string(applicationSnapshotJSON),
								},
							},
							{
								Name: "POLICY_CONFIGURATION",
								Value: v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.PolicyConfiguration,
								},
							},
							{
								Name: "PUBLIC_KEY",
								Value: v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.PublicKey,
								},
							},
							{
								Name: "SSL_CERT_DIR",
								Value: v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeString,
									StringVal: sslCertDir,
								},
							},
							{
								Name: "STRICT",
								Value: v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeString,
									StringVal: strconv.FormatBool(p.Strict),
								},
							},
							{
								Name: "EFFECTIVE_TIME",
								Value: v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.EffectiveTime,
								},
							},
							{
								Name: "IGNORE_REKOR",
								Value: v1beta1.ParamValue{
									Type:      v1beta1.ParamTypeString,
									StringVal: strconv.FormatBool(p.IgnoreRekor),
								},
							},
						},
						TaskRef: &v1beta1.TaskRef{
							Name:   "verify-enterprise-contract",
							Bundle: p.TaskBundle,
						},
					},
				},
			},
		},
	}, nil
}

// Generates pipelineRun from ECIntegrationTestScenario.
func (p ECIntegrationTestScenario) Generate() (*v1beta1.PipelineRun, error) {

	snapshot := `{"components": [
		{"containerImage": "` + p.Image + `"}
	]}`

	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ec-integration-test-scenario-run-",
			Namespace:    p.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				ResolverRef: v1beta1.ResolverRef{
					Resolver: "git",
					Params: []v1beta1.Param{
						{Name: "url", Value: *v1beta1.NewStructuredValues(p.PipelineGitURL)},
						{Name: "revision", Value: *v1beta1.NewStructuredValues(p.PipelineGitRevision)},
						{Name: "pathInRepo", Value: *v1beta1.NewStructuredValues(p.PipelineGitPathInRepo)},
					},
				},
			},
			Params: []v1beta1.Param{
				{Name: "SNAPSHOT", Value: *v1beta1.NewStructuredValues(snapshot)},
			},
		},
	}, nil
}

// CreatePipelineRun creates a tekton pipelineRun and returns the pipelineRun or error
func (t *TektonController) CreatePipelineRun(pipelineRun *v1beta1.PipelineRun, ns string) (*v1beta1.PipelineRun, error) {
	return t.PipelineClient().TektonV1beta1().PipelineRuns(ns).Create(context.TODO(), pipelineRun, metav1.CreateOptions{})
}

// createAndWait creates a pipelineRun and waits until it starts.
func (t *TektonController) createAndWait(pr *v1beta1.PipelineRun, namespace string, taskTimeout int) (*v1beta1.PipelineRun, error) {
	pipelineRun, err := t.CreatePipelineRun(pr, namespace)
	if err != nil {
		return nil, err
	}
	g.GinkgoWriter.Printf("Creating Pipeline %q\n", pipelineRun.Name)
	return pipelineRun, utils.WaitUntil(t.CheckPipelineRunStarted(pipelineRun.Name, namespace), time.Duration(taskTimeout)*time.Second)
}

// RunPipeline creates a pipelineRun and waits for it to start.
func (t *TektonController) RunPipeline(g PipelineRunGenerator, namespace string, taskTimeout int) (*v1beta1.PipelineRun, error) {
	pr, err := g.Generate()
	if err != nil {
		return nil, err
	}
	pvcs := t.KubeInterface().CoreV1().PersistentVolumeClaims(pr.Namespace)
	for _, w := range pr.Spec.Workspaces {
		if w.PersistentVolumeClaim != nil {
			pvcName := w.PersistentVolumeClaim.ClaimName
			if _, err := pvcs.Get(context.TODO(), pvcName, metav1.GetOptions{}); err != nil {
				if errors.IsNotFound(err) {
					err := createPVC(pvcs, pvcName)
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
func (t *TektonController) GetPipelineRun(pipelineRunName, namespace string) (*v1beta1.PipelineRun, error) {
	return t.PipelineClient().TektonV1beta1().PipelineRuns(namespace).Get(context.TODO(), pipelineRunName, metav1.GetOptions{})
}

// GetPipelineRunLogs returns logs of a given pipelineRun.
func (t *TektonController) GetPipelineRunLogs(pipelineRunName, namespace string) (string, error) {
	podClient := t.KubeInterface().CoreV1().Pods(namespace)
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
func (t *TektonController) ListAllPipelineRuns(ns string) (*v1beta1.PipelineRunList, error) {
	return t.PipelineClient().TektonV1beta1().PipelineRuns(ns).List(context.TODO(), metav1.ListOptions{})
}

// DeletePipelineRun deletes a pipelineRun form a given namespace.
func (t *TektonController) DeletePipelineRun(name, ns string) error {
	return t.PipelineClient().TektonV1beta1().PipelineRuns(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// DeleteAllPipelineRunsInASpecificNamespace deletes all PipelineRuns in a given namespace (removing the finalizers field, first)
func (t *TektonController) DeleteAllPipelineRunsInASpecificNamespace(ns string) error {

	pipelineRunList, err := t.ListAllPipelineRuns(ns)
	if err != nil || pipelineRunList == nil {
		return fmt.Errorf("unable to delete all PipelineRuns in '%s': %v", ns, err)
	}

	for _, pipelineRun := range pipelineRunList.Items {
		err := wait.PollUntilContextTimeout(context.Background(), time.Second, 30*time.Second, true, func(ctx context.Context) (done bool, err error) {
			pipelineRunCR := v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: ns,
				},
			}
			if err := t.KubeRest().Get(context.TODO(), crclient.ObjectKeyFromObject(&pipelineRunCR), &pipelineRunCR); err != nil {
				if errors.IsNotFound(err) {
					// PipelinerRun CR is already removed
					return true, nil
				}
				g.GinkgoWriter.Printf("unable to retrieve PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil

			}

			// Remove the finalizer, so that it can be deleted.
			pipelineRunCR.Finalizers = []string{}
			if err := t.KubeRest().Update(context.TODO(), &pipelineRunCR); err != nil {
				g.GinkgoWriter.Printf("unable to remove finalizers from PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil
			}

			if err := t.KubeRest().Delete(context.TODO(), &pipelineRunCR); err != nil {
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
func (t *TektonController) StorePipelineRun(pipelineRun *v1beta1.PipelineRun) error {
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
		if err := t.StorePipelineRun(&pipelineRun); err != nil {
			return fmt.Errorf("got error storing PR: %v\n", err.Error())
		}
	}

	return nil
}
