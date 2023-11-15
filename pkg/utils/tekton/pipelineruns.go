package tekton

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
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

type FailedPipelineRunDetails struct {
	FailedTaskRunName   string
	PodName             string
	FailedContainerName string
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
			PipelineRef: &v1beta1.PipelineRef{
				Name:   "docker-build",
				Bundle: b.Bundle, //nolint:all
			},
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

// GetFailedPipelineRunLogs gets the logs of the pipelinerun failed task
func GetFailedPipelineRunLogs(c crclient.Client, ki kubernetes.Interface, pipelineRun *v1beta1.PipelineRun) (string, error) {
	var d *FailedPipelineRunDetails
	var err error
	failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
	if d, err = GetFailedPipelineRunDetails(c, pipelineRun); err != nil {
		return "", err
	}
	if d.FailedContainerName != "" {
		logs, _ := utils.GetContainerLogs(ki, d.PodName, d.FailedContainerName, pipelineRun.Namespace)
		failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
	}
	return failMessage, nil
}

func HasPipelineRunSucceeded(pr *v1beta1.PipelineRun) bool {
	return pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue()
}

func HasPipelineRunFailed(pr *v1beta1.PipelineRun) bool {
	return pr.IsDone() && pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsFalse()
}

func GetFailedPipelineRunDetails(c crclient.Client, pipelineRun *v1beta1.PipelineRun) (*FailedPipelineRunDetails, error) {
	d := &FailedPipelineRunDetails{}
	for _, chr := range pipelineRun.Status.PipelineRunStatusFields.ChildReferences {
		taskRun := &v1beta1.TaskRun{}
		taskRunKey := types.NamespacedName{Namespace: pipelineRun.Namespace, Name: chr.Name}
		if err := c.Get(context.Background(), taskRunKey, taskRun); err != nil {
			return nil, fmt.Errorf("failed to get details for PR %s: %+v", pipelineRun.GetName(), err)
		}
		for _, c := range taskRun.Status.Conditions {
			if c.Reason == "Failed" {
				d.FailedTaskRunName = taskRun.Name
				d.PodName = taskRun.Status.PodName
				for _, s := range taskRun.Status.TaskRunStatusFields.Steps {
					if s.Terminated.Reason == "Error" {
						d.FailedContainerName = s.ContainerName
						return d, nil
					}
				}
			}
		}
	}
	return d, nil
}
